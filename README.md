# Multi-Tenant AI Knowledge & Decision Assistant

A working MVP for organizations that need a private, source-grounded AI assistant built from PDF documents. Each organization has an isolated knowledge base, a public assistant URL, iframe embed code, and a management dashboard.

## Features

- Organization registration, login, JWT dashboard access, and logout.
- Private PDF knowledge bases: 10 documents per organization, 10 MB per PDF, and 20 pages per PDF.
- Page-aware text extraction, chunking, tenant-scoped keyword retrieval, and source references.
- Public assistant URLs and iframe embedding without dashboard login.
- LLM-based question classification and grounded answer phrasing when configured.
- Deterministic System 1 policy decisions for GPA, academic year, and existing sports-scholarship conditions.
- Usage counts for answered, unable-to-answer, and human-review outcomes.

## Prerequisites

- Go 1.22 or later.
- Node.js 18 or later and npm.
- PostgreSQL 14 or later.
- Optional: an OpenAI-compatible API key for LLM-based understanding and answer generation.
- Optional but recommended on Windows: `pdftotext` on `PATH` for PDFs that the Go parser cannot read.

## Installation and local run

1. Create a PostgreSQL database named `knowledge_assistant`.
2. Create and configure the backend environment file:

```powershell
Copy-Item .env.example .env
```

3. Start the backend from the repository root:

```powershell
go mod tidy
go run .
```

The backend creates tables at startup and listens on `http://localhost:8080`.

4. In a second terminal, start the frontend:

```powershell
cd frontend
Copy-Item .env.example .env
npm install
npm run dev
```

5. Set `APP_BASE_URL=http://127.0.0.1:5173` in the backend `.env` when generated assistant URLs and iframe code should open the React public chat. Restart the backend after changing it.

Open `http://127.0.0.1:5173/register` and create an organization.

## Environment variables

| Variable | Required | Purpose |
|---|---:|---|
| `APP_PORT` | No | Backend port; defaults to `8080`. |
| `DATABASE_URL` | Yes | PostgreSQL/pgx connection string. |
| `JWT_SECRET` | Yes | Secret used to sign dashboard JWTs. Use a long random value outside local development. |
| `LLM_API_KEY` | No | OpenAI-compatible API key; enables LLM classification and answer phrasing. |
| `LLM_BASE_URL` | No | API base URL; defaults to `https://api.openai.com/v1`. |
| `LLM_MODEL` | No | Chat model; defaults to `gpt-4.1-mini`. |
| `DOCUMENT_STORAGE_PATH` | No | Private PDF directory; defaults to `./storage`. |
| `APP_BASE_URL` | Yes for React embed URLs | Public frontend origin, for example `http://127.0.0.1:5173`. |
| `VITE_API_BASE_URL` | Frontend | Backend API origin in `frontend/.env`, for example `http://localhost:8080`. |

Never commit `.env`, API keys, database passwords, or production JWT secrets.

## Architecture

```text
                    +-----------------------+
                    | React / Vite frontend  |
                    | Dashboard + public chat|
                    +-----------+-----------+
                                |
             JWT admin API      |      X-Assistant-Token public API
                                v
                    +-----------------------+
                    | Gin API               |
                    | handlers + middleware |
                    +-----------+-----------+
                                |
       +------------------------+-------------------------+
       |                        |                         |
       v                        v                         v
+---------------+      +-------------------+     +---------------------+
| Auth/JWT       |      | Document service  |     | Ask service         |
| organization   |      | validate/extract/ |     | retrieve/understand |
| context        |      | chunk/store       |     | decide/generate     |
+-------+--------+      +---------+---------+     +----------+----------+
        |                           |                        |
        +---------------------------+------------------------+
                                    v
                    +-------------------------------+
                    | PostgreSQL via pgx             |
                    | tenants, users, assistants,    |
                    | documents, chunks, usage logs  |
                    +-------------------------------+
```

| Component | Responsibility |
|---|---|
| `config/` | Environment-based configuration. |
| `database/` | pgx connection pool and schema migration. |
| `middleware/` | JWT validation and authenticated organization context. |
| `handler/` | HTTP validation and JSON responses. |
| `service/` | PDF processing, retrieval orchestration, LLM calls, and decisions. |
| `Repository/` | Tenant-scoped PostgreSQL access. |
| `Utils/` | JWT, token, PDF, validation, and chunking helpers. |
| `frontend/` | React/Vite dashboard and public assistant UI. |

## Data and API flow

### Knowledge-base flow

```text
JWT dashboard user
  -> POST /api/documents (multipart PDF)
  -> validate extension, header, size, document count, and page limit
  -> store PDF outside public web routes
  -> extract text per page
  -> split page text into approximately 1,200-character chunks
  -> save document + chunks with organization_id and page number
  -> return ready document metadata
```

Keyword retrieval searches only `document_chunks` whose `organization_id` belongs to the resolved assistant. Each chunk retains its document filename and page number, so answers cite sources such as `Scholarship_Rules.pdf - Page 4`.

### Visitor question flow

```text
Visitor question
  -> POST /api/ask with X-Assistant-Token
  -> resolve assistant token to organization_id
  -> retrieve relevant chunks for that organization only
  -> LLM/heuristic question understanding
  -> deterministic System 1 policy checks when required
  -> combine decision result + evidence
  -> LLM grounded phrasing or local grounded fallback
  -> answer, outcome, and document/page references
```

If no relevant tenant-scoped evidence is found, the API returns an unable-to-answer result and does not invent an answer.

## AI and System 1 decision architecture

The application deliberately separates language tasks from deterministic decisions.

| Stage | Component | Responsibility |
|---|---|---|
| Question understanding | LLM when configured; heuristic fallback | Classifies direct, policy, procedural, comparison, or multi-condition questions and identifies when a decision is required. |
| Rule extraction | Application logic | Reads supported policy patterns from retrieved evidence: minimum GPA, allowed academic years, and sports-scholarship restrictions. |
| Independent System 1 checks | Application logic | Evaluates GPA, academic year, and existing sports scholarship separately. |
| Combination | Application logic | Combines checks into eligible/ineligible, support score, reasons, and missing facts. |
| Answer generation | LLM when configured; local fallback | Produces a concise answer using only retrieved evidence and the combined decision. |

The final answer model never decides eligibility. If a retrieved policy requires facts absent from the visitor question, the result is `human_review` and the visitor receives the missing-facts message. If evidence is absent, the result is `unable_to_answer`.

## Security and tenant isolation

- Dashboard APIs require `Authorization: Bearer <jwt>`; the JWT contains the authenticated `organization_id`.
- Administrative queries filter by the authenticated organization ID.
- Public access requires an opaque random assistant token in `X-Assistant-Token`; the server resolves it to one organization before retrieval.
- Assistant tokens cannot call JWT-protected dashboard endpoints.
- Chunk retrieval filters by `organization_id`, preventing Organization A data from appearing in Organization B answers.
- PDFs are stored privately and no document download/static-file route is exposed.
- Regenerating an assistant token replaces the server-side verifier, invalidating the old token.
- An opt-in PostgreSQL integration test verifies tenant retrieval isolation and token rotation.

## Major API endpoints

| Method | Endpoint | Authentication | Purpose |
|---|---|---|---|
| `POST` | `/api/auth/register` | None | Create an organization, first user, and assistant. |
| `POST` | `/api/auth/login` | None | Create a dashboard JWT session. |
| `POST` | `/api/auth/logout` | JWT | End the client dashboard session. |
| `GET` | `/api/organization` | JWT | Get organization/account information. |
| `GET` | `/api/documents` | JWT | List the organization’s documents. |
| `POST` | `/api/documents` | JWT | Upload and synchronously process one PDF. |
| `DELETE` | `/api/documents/:documentId` | JWT | Delete one organization-owned document and chunks. |
| `GET` | `/api/assistant` | JWT | Get public assistant URL and iframe code. |
| `POST` | `/api/assistant/regenerate-token` | JWT | Create a new assistant token and invalidate the old token. |
| `GET` | `/api/usage` | JWT | Get question outcome counts. |
| `POST` | `/api/ask` | Assistant token | Answer a visitor question against one organization’s knowledge base. |

Successful JSON API responses use `{ "data": ... }`; errors use `{ "error": { "code": "...", "message": "..." } }`.

## Embedding the assistant

After setting `APP_BASE_URL` to the frontend origin, the dashboard returns embed code similar to:

```html
<iframe src="http://127.0.0.1:5173/assistant/ASSISTANT_TOKEN" width="400" height="600" title="AI Assistant"></iframe>
```

The iframe sends questions to `POST /api/ask`; it does not receive or use a dashboard JWT.

## Testing and reviewer flow

1. Register Organization A and upload a text-based PDF.
2. Register Organization B and upload different content.
3. Ask each assistant about the other organization’s document. It must not return cross-tenant content or references.
4. Save the four organization fixtures listed in [sample-data/TEST_DATA.md](sample-data/TEST_DATA.md) as text-based PDFs and upload each organization’s two PDFs only to that organization.
5. Run all 12 questions in the test matrix, including the multi-condition, human-review, no-answer, and cross-tenant cases.
6. Open Usage to verify counters, regenerate the assistant token, and confirm the old token no longer works.

```powershell
# Deterministic System 1 policy tests
go test ./service

# Complete backend suite
go test ./...

# PostgreSQL tenant-isolation integration test
$env:TEST_DATABASE_URL = "postgres://postgres:postgres@localhost:5432/knowledge_assistant?sslmode=disable"
go test ./Repository -run TenantIsolation

# Frontend production build
cd frontend
npm run build
```

## Known limitations and production improvements

- Retrieval is keyword-based; production should use embeddings or hybrid search, ranking, and relevance thresholds.
- Policy parsing is intentionally specialized to the demonstration rules; production should use a versioned rule engine and formal policy schemas.
- Upload processing is synchronous; production should use a background queue, retries, and observability.
- PDFs use local filesystem storage; production should use private object storage, malware scanning, encryption, retention rules, and signed internal access.
- The MVP records a human-review outcome but does not provide a reviewer queue, assignment workflow, or reviewer UI.
- The opt-in isolation test covers retrieval and token rotation; production should add HTTP integration tests for every authorization boundary.
- Add rate limiting, strict production CORS/origin policy, audit logs, monitoring, backup/restore, and secrets management before deployment.
- Configure a real LLM key for a complete LLM demonstration. The local fallback remains grounded but is less conversational.
