package repository

import (
	"context"
	"os"
	"testing"

	"ai-knowledge-assistant/Utils"
	"ai-knowledge-assistant/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestTenantIsolationIntegration is intentionally opt-in because it creates
// temporary tenant records in a real PostgreSQL database. Run it with:
// TEST_DATABASE_URL="postgres://..." go test ./Repository -run TenantIsolation
func TestTenantIsolationIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL tenant-isolation integration tests")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	var orgA, orgB, docA, docB string
	if err = db.QueryRow(ctx, "INSERT INTO organizations(name) VALUES('isolation-a') RETURNING id").Scan(&orgA); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(ctx, "DELETE FROM organizations WHERE id=$1", orgA)
	if err = db.QueryRow(ctx, "INSERT INTO organizations(name) VALUES('isolation-b') RETURNING id").Scan(&orgB); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(ctx, "DELETE FROM organizations WHERE id=$1", orgB)
	if _, err = db.Exec(ctx, "INSERT INTO assistants(organization_id,public_token,token_hash,token_hint) VALUES($1,'public-a','hash-a','a')", orgA); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO assistants(organization_id,public_token,token_hash,token_hint) VALUES($1,'public-b','hash-b','b')", orgB); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, "INSERT INTO documents(organization_id,filename,storage_key,size_bytes,page_count,status) VALUES($1,'A.pdf','a.pdf',1,1,'ready') RETURNING id", orgA).Scan(&docA); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, "INSERT INTO documents(organization_id,filename,storage_key,size_bytes,page_count,status) VALUES($1,'B.pdf','b.pdf',1,1,'ready') RETURNING id", orgB).Scan(&docB); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO document_chunks(organization_id,document_id,page_number,content) VALUES($1,$2,1,'isolated alpha knowledge'),($3,$4,1,'isolated beta knowledge')", orgA, docA, orgB, docB); err != nil {
		t.Fatal(err)
	}

	repo := Repository{DB: db}
	chunks, err := repo.Search(ctx, orgA, "alpha knowledge")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks[0].DocumentID != docA || chunks[0].Filename != "A.pdf" {
		t.Fatalf("organization A received non-A content: %#v", chunks)
	}
	_, resolvedOrg, err := repo.ResolveAssistant(ctx, "hash-a")
	if err != nil || resolvedOrg != orgA {
		t.Fatalf("assistant A must resolve only to organization A: org=%s err=%v", resolvedOrg, err)
	}
	if err = repo.RotateAssistant(ctx, orgA, "new-public-a", utils.HashToken("new-token-a"), "new"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.ResolveAssistant(ctx, "hash-a"); err == nil {
		t.Fatal("old assistant token hash must be invalid after rotation")
	}
	if _, resolvedOrg, err = repo.ResolveAssistant(ctx, utils.HashToken("new-token-a")); err != nil || resolvedOrg != orgA {
		t.Fatalf("new assistant token must resolve to organization A: org=%s err=%v", resolvedOrg, err)
	}
}
