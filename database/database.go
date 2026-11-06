package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	return pgxpool.New(ctx, url)
}

func Migrate(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TABLE IF NOT EXISTS organizations (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS users (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, email text NOT NULL UNIQUE, password_hash text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS assistants (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE, public_token text NOT NULL UNIQUE, token_hash text NOT NULL UNIQUE, token_hint text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), rotated_at timestamptz);
CREATE TABLE IF NOT EXISTS documents (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, filename text NOT NULL, storage_key text NOT NULL, size_bytes bigint NOT NULL, page_count integer NOT NULL, status text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS document_chunks (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, document_id uuid NOT NULL REFERENCES documents(id) ON DELETE CASCADE, page_number integer NOT NULL, content text NOT NULL);
CREATE INDEX IF NOT EXISTS document_chunks_tenant_idx ON document_chunks(organization_id, document_id);
CREATE TABLE IF NOT EXISTS question_logs (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, assistant_id uuid NOT NULL REFERENCES assistants(id) ON DELETE CASCADE, outcome text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());`)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `ALTER TABLE assistants ADD COLUMN IF NOT EXISTS public_token text`)
	return err
}
