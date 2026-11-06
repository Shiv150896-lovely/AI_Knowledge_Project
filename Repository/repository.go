package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ai-knowledge-assistant/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }

func (r Repository) Register(ctx context.Context, name, email, hash, publicToken, tokenHash, hint string) (string, string, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	var org, user string
	if err = tx.QueryRow(ctx, "INSERT INTO organizations(name) VALUES($1) RETURNING id", name).Scan(&org); err != nil {
		return "", "", err
	}
	if err = tx.QueryRow(ctx, "INSERT INTO users(organization_id,email,password_hash) VALUES($1,$2,$3) RETURNING id", org, email, hash).Scan(&user); err != nil {
		return "", "", err
	}
	_, err = tx.Exec(ctx, "INSERT INTO assistants(organization_id,public_token,token_hash,token_hint) VALUES($1,$2,$3,$4)", org, publicToken, tokenHash, hint)
	if err != nil {
		return "", "", err
	}
	return org, user, tx.Commit(ctx)
}
func (r Repository) Login(ctx context.Context, email string) (userID, orgID, hash, name string, err error) {
	err = r.DB.QueryRow(ctx, "SELECT u.id,u.organization_id,u.password_hash,o.name FROM users u JOIN organizations o ON o.id=u.organization_id WHERE u.email=$1", strings.ToLower(email)).Scan(&userID, &orgID, &hash, &name)
	return
}
func (r Repository) Organization(ctx context.Context, org string) (models.Organization, error) {
	var x models.Organization
	err := r.DB.QueryRow(ctx, "SELECT o.id,o.name,u.email,o.created_at FROM organizations o JOIN users u ON u.organization_id=o.id WHERE o.id=$1 ORDER BY u.created_at LIMIT 1", org).Scan(&x.ID, &x.Name, &x.Email, &x.CreatedAt)
	return x, err
}
func (r Repository) DocumentCount(ctx context.Context, org string) (int, error) {
	var n int
	err := r.DB.QueryRow(ctx, "SELECT count(*) FROM documents WHERE organization_id=$1", org).Scan(&n)
	return n, err
}
func (r Repository) CreateDocument(ctx context.Context, org, filename, key string, size int64, pages int, status string) (string, error) {
	var id string
	err := r.DB.QueryRow(ctx, "INSERT INTO documents(organization_id,filename,storage_key,size_bytes,page_count,status) VALUES($1,$2,$3,$4,$5,$6) RETURNING id", org, filename, key, size, pages, status).Scan(&id)
	return id, err
}
func (r Repository) SetDocumentStatus(ctx context.Context, id, org, status string) error {
	_, err := r.DB.Exec(ctx, "UPDATE documents SET status=$1 WHERE id=$2 AND organization_id=$3", status, id, org)
	return err
}
func (r Repository) AddChunks(ctx context.Context, org, doc string, chunks []models.Chunk) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, x := range chunks {
		if _, err = tx.Exec(ctx, "INSERT INTO document_chunks(organization_id,document_id,page_number,content) VALUES($1,$2,$3,$4)", org, doc, x.Page, x.Content); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (r Repository) Documents(ctx context.Context, org string) ([]models.Document, error) {
	rows, err := r.DB.Query(ctx, "SELECT id,filename,status,size_bytes,page_count,created_at FROM documents WHERE organization_id=$1 ORDER BY created_at DESC", org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Document{}
	for rows.Next() {
		var x models.Document
		if err = rows.Scan(&x.ID, &x.Filename, &x.Status, &x.SizeBytes, &x.PageCount, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r Repository) DeleteDocument(ctx context.Context, org, id string) (string, error) {
	var key string
	err := r.DB.QueryRow(ctx, "DELETE FROM documents WHERE id=$1 AND organization_id=$2 RETURNING storage_key", id, org).Scan(&key)
	return key, err
}
func (r Repository) Assistant(ctx context.Context, org string) (id, publicToken string, err error) {
	err = r.DB.QueryRow(ctx, "SELECT id,public_token FROM assistants WHERE organization_id=$1", org).Scan(&id, &publicToken)
	return
}
func (r Repository) ResolveAssistant(ctx context.Context, hash string) (assistantID, org string, err error) {
	err = r.DB.QueryRow(ctx, "SELECT id,organization_id FROM assistants WHERE token_hash=$1", hash).Scan(&assistantID, &org)
	return
}
func (r Repository) RotateAssistant(ctx context.Context, org, publicToken, hash, hint string) error {
	_, err := r.DB.Exec(ctx, "UPDATE assistants SET public_token=$1,token_hash=$2,token_hint=$3,rotated_at=now() WHERE organization_id=$4", publicToken, hash, hint, org)
	return err
}
func (r Repository) Search(ctx context.Context, org, question string) ([]models.Chunk, error) {
	terms := strings.Fields(strings.ToLower(question))
	if len(terms) == 0 {
		return nil, nil
	}
	clauses := make([]string, 0, len(terms))
	args := []any{org}
	for _, t := range terms {
		if len(t) > 2 {
			args = append(args, "%"+t+"%")
			clauses = append(clauses, fmt.Sprintf("lower(c.content) LIKE $%d", len(args)))
		}
	}
	if len(clauses) == 0 {
		return nil, nil
	}
	q := "SELECT c.document_id,d.filename,c.page_number,c.content FROM document_chunks c JOIN documents d ON d.id=c.document_id WHERE c.organization_id=$1 AND d.status='ready' AND (" + strings.Join(clauses, " OR ") + ") ORDER BY c.page_number LIMIT 8"
	rows, err := r.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Chunk{}
	for rows.Next() {
		var x models.Chunk
		if err = rows.Scan(&x.DocumentID, &x.Filename, &x.Page, &x.Content); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r Repository) Log(ctx context.Context, org, assistant, outcome string) error {
	_, err := r.DB.Exec(ctx, "INSERT INTO question_logs(organization_id,assistant_id,outcome) VALUES($1,$2,$3)", org, assistant, outcome)
	return err
}
func (r Repository) Usage(ctx context.Context, org string) (map[string]int, error) {
	rows, err := r.DB.Query(ctx, "SELECT outcome,count(*) FROM question_logs WHERE organization_id=$1 GROUP BY outcome", org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]int{"questions_asked": 0, "answered": 0, "unable_to_answer": 0, "human_review": 0}
	for rows.Next() {
		var k string
		var n int
		if err = rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		m[k] = n
		m["questions_asked"] += n
	}
	return m, rows.Err()
}
func NotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
