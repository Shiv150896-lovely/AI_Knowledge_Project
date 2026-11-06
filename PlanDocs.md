# MVP Plan — Multi-Tenant AI Knowledge & Decision Assistant

## Scope

Deliver this flow end to end:

`Registration → Login → PDF upload → processing/indexing → assistant URL/embed code → visitor question → tenant-scoped retrieval → LLM + System 1 decisions → grounded answer + references`

Each organization has isolated users, documents, chunks, assistant token, and usage. A tenant is always resolved server-side; every organization-owned database query filters by `organization_id`.

## Required project layout

```text
.
├── config/          # environment loading and application configuration
├── database/        # pgx pool, migrations, schema setup
├── handler/         # HTTP request/response handlers
├── middleware/      # JWT authentication, tenant/auth context, error handling
├── service/         # auth, document processing, retrieval, ask pipeline, decisions
├── models/          # request, response, database/domain models
├── Utils/           # JWT, password, PDF, validation, response helpers
├── Repository/      # pgx data-access repositories
├── routes/          # Gin route registration
├── .env             # local secrets/configuration; do not commit
├── .env.example     # non-secret environment template
├── main.go          # application bootstrap and Gin server
└── README.md        # setup, migration, run, and test instructions
```

## Core data model

| Entity | Required fields | Purpose |
|---|---|---|
| organizations | id, name, created_at | Tenant boundary. |
| users | id, organization_id, email, password_hash, created_at | Dashboard account. |
| assistants | id, organization_id, public_token, token_hash, token_hint, created_at, rotated_at | Public assistant identity; retain the random public URL token for dashboard embed-code retrieval and its hash for server-side validation. |
| documents | id, organization_id, filename, storage_key, size_bytes, page_count, status, created_at | Private uploaded PDF metadata. |
| document_chunks | id, organization_id, document_id, page_number, content | Tenant-scoped keyword-retrieval source. |
| question_logs | id, organization_id, assistant_id, outcome, created_at | Usage counts: answered, unable_to_answer, human_review; question text is intentionally not stored. |

## Processing and question pipeline

1. Validate authenticated organization and upload constraints: PDF only, at most 10 files per organization, at most 10 MB and 20 pages per file.
2. Store the PDF privately, extract page-aware text, chunk it, create retrieval data, and persist chunks with `organization_id`, `document_id`, and page number.
3. For public questions, validate the assistant token and resolve its organization.
4. Retrieve only chunks where `organization_id` equals the resolved organization. Keep document filename and page number with retrieved context.
5. Use the LLM to classify/decompose the question and to generate the final answer from retrieved context and decision results.
6. For policy/multi-condition questions, the LLM classifies/decomposes the question but deterministic application/System 1 logic extracts policy rules and user facts, then independently evaluates GPA, academic-year, and existing-scholarship conditions. Combine the boolean results into eligibility and calculate a score from passed conditions.
7. Combine decision outputs in application logic, then ask the LLM to phrase a grounded answer. Return cited sources. If evidence is absent, return an unable-to-answer response and no invented content.

## Endpoint map

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | /api/auth/register | No | Create organization, first user, and assistant token. |
| POST | /api/auth/login | No | Authenticate dashboard user and issue JWT. |
| GET | /api/organization | JWT | Get organization/account details. |
| GET | /api/documents | JWT | List the organization’s documents. |
| POST | /api/documents | JWT | Upload and queue/process one PDF. |
| DELETE | /api/documents/:documentId | JWT | Delete one organization-owned document and its chunks. |
| GET | /api/assistant | JWT | Get the organization’s assistant URL and embed code. |
| POST | /api/assistant/regenerate-token | JWT | Invalidate current token and issue a new assistant token. |
| GET | /api/usage | JWT | Get basic question outcome counts. |
| POST | /api/ask | No; assistant token | Answer a visitor question against the token’s organization only. |

## Mandatory API contract

### Common conventions

- JSON endpoints: request and response header `Content-Type: application/json`.
- Authenticated endpoints: header `Authorization: Bearer <jwt>`.
- File upload: header `Content-Type: multipart/form-data`.
- Public assistant access sends the raw assistant token only in `X-Assistant-Token`; never return document content or credentials.
- Success responses use `{ "data": ... }`; errors use `{ "error": { "code": "...", "message": "..." } }`.

### POST /api/auth/register

Headers: `Content-Type: application/json`  
Body: `{ "organization_name": "Acme University", "email": "admin@acme.edu", "password": "secret123" }`  
Response `201`:

```json
{"data":{"organization":{"id":"org_uuid","name":"Acme University"},"access_token":"jwt","assistant":{"url":"/assistant/asst_token","embed_code":"<iframe src=\"https://app.example/assistant/asst_token\"></iframe>"}}}
```

Errors: `400` invalid fields, `409` duplicate email.

### POST /api/auth/login

Headers: `Content-Type: application/json`  
Body: `{ "email": "admin@acme.edu", "password": "secret123" }`  
Response `200`: `{"data":{"access_token":"jwt","organization":{"id":"org_uuid","name":"Acme University"}}}`  
Errors: `400` invalid fields, `401` invalid credentials.

### GET /api/organization

Headers: `Authorization: Bearer <jwt>`  
Params: none  
Response `200`: `{"data":{"id":"org_uuid","name":"Acme University","email":"admin@acme.edu"}}`  
Errors: `401` missing/invalid JWT.

### GET /api/documents

Headers: `Authorization: Bearer <jwt>`  
Query params: none  
Response `200`:

```json
{"data":{"limit":10,"count":1,"documents":[{"id":"doc_uuid","filename":"Scholarship_Rules.pdf","size_bytes":524288,"page_count":8,"status":"ready","created_at":"2026-11-06T10:00:00Z"}]}}
```

Errors: `401` missing/invalid JWT.

### POST /api/documents

Headers: `Authorization: Bearer <jwt>`, `Content-Type: multipart/form-data`  
Form params: `file` (required PDF)  
Response `201`: `{"data":{"id":"doc_uuid","filename":"Scholarship_Rules.pdf","size_bytes":524288,"page_count":8,"status":"ready"}}`  
The current MVP processes uploads synchronously. Errors: `400` missing/invalid PDF, `409` document limit reached, `413` file exceeds 10 MB, `422` PDF exceeds 20 pages or document-processing failure, `500` storage/database failure.

### DELETE /api/documents/:documentId

Headers: `Authorization: Bearer <jwt>`  
Path params: `documentId` (required UUID)  
Response `200`: `{"data":{"id":"doc_uuid","deleted":true}}`  
Errors: `401` missing/invalid JWT, `404` document not found for this organization.

### GET /api/assistant

Headers: `Authorization: Bearer <jwt>`  
Params: none  
Response `200`: `{"data":{"url":"https://app.example/assistant/asst_token","embed_code":"<iframe src=\"https://app.example/assistant/asst_token\"></iframe>"}}`  
Errors: `401` missing/invalid JWT.

### POST /api/assistant/regenerate-token

Headers: `Authorization: Bearer <jwt>`, `Content-Type: application/json`  
Body: none  
Response `200`: `{"data":{"url":"https://app.example/assistant/new_asst_token","embed_code":"<iframe src=\"https://app.example/assistant/new_asst_token\"></iframe>"}}`  
Errors: `401` missing/invalid JWT.

### GET /api/usage

Headers: `Authorization: Bearer <jwt>`  
Query params: none  
Response `200`: `{"data":{"questions_asked":12,"answered":9,"unable_to_answer":2,"human_review":1}}`  
Errors: `401` missing/invalid JWT.

### POST /api/ask

Headers: `Content-Type: application/json`, `X-Assistant-Token: <assistant_token>`  
Body: `{ "question": "Am I eligible for a merit scholarship with a 3.8 GPA?" }`  
Response `200`:

```json
{"data":{"answer":"Based on the scholarship rules, a 3.8 GPA satisfies the GPA requirement. Other listed conditions still apply.","outcome":"answered","references":[{"document":"Scholarship_Rules.pdf","page":4}],"decision":{"question_type":"eligibility","category":"merit_scholarship","eligible":true,"score":0.86}}}
```

Errors: `400` empty question, `401` invalid assistant token, `422` no relevant knowledge, `502` LLM/API or decision-model failure.

## Isolation and error acceptance checks

- `TEST_DATABASE_URL="postgres://..." go test ./Repository -run TenantIsolation` proves Organization A search results contain only A chunks and references, and that regenerating A's assistant invalidates its old token.
- A JWT for Organization A cannot list, upload against, or delete Organization B documents.
- An assistant token for Organization A cannot retrieve Organization B chunks or references.
- Replacing/regenerating a token makes the old token invalid.
- Invalid PDF; 11th document; PDF over 10 MB; PDF over 20 pages; empty question; unavailable LLM/decision service; and extraction failure return the mapped user-facing errors.
- Verify direct information, policy, procedural, multi-condition decision, comparison, and no-answer questions.

## Reviewer fixture

`sample-data/Merit_Scholarship_Policy.txt` contains policy text and expected questions for deterministic decision testing. Save or print this text as a PDF, upload it, then use the listed questions to verify eligible, ineligible, and human-review outcomes.
