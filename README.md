# Multi-Tenant AI Knowledge & Decision Assistant

Gin + PostgreSQL/pgx MVP implementing organization-isolated PDF knowledge bases, public assistant URLs, embedded chat, grounded references, LLM understanding/generation, and structured decisions.

## Run locally

1. Create a PostgreSQL database named `knowledge_assistant`.
2. Copy `.env.example` to `.env`, then set `DATABASE_URL` and `JWT_SECRET`. Set an OpenAI-compatible `LLM_API_KEY` for LLM-based question classification and grounded answer phrasing.
3. Set `APP_BASE_URL=http://127.0.0.1:5173` when using the React frontend so generated assistant URLs and iframe code open the public React chat.
4. Start the API:

```powershell
go mod tidy
go run .
```

Tables are created automatically at startup. The API is served at `http://localhost:8080`.

5. In another terminal, start the frontend:

```powershell
cd frontend
Copy-Item .env.example .env
npm install
npm run dev
```

Open `http://127.0.0.1:5173/register`.

## Main flow

1. `POST /api/auth/register` creates the organization, user, and assistant token.
2. Send `Authorization: Bearer <access_token>` to upload PDFs at `POST /api/documents`.
3. Use the registration/regeneration response embed code, or open its assistant URL.
4. The public page calls `POST /api/ask` using the assistant token. Retrieval filters every chunk by the token’s organization ID.

## Security and limits

- Administrative API groups require a JWT; assistant tokens cannot call them.
- Assistant tokens are stored as hashes and regenerated server-side.
- PDFs are private files; documents are never served publicly.
- Maximum 10 PDFs per organization, 10 MB per PDF, and 20 pages per PDF.
- The LLM receives only retrieved chunks from the resolved organization. The boolean/choice decision result is created in application logic and is passed to the LLM only for phrasing.

## Demonstration embed page

Save this as a page on any site after generating an assistant URL:

```html
<iframe src="http://localhost:8080/assistant/ASSISTANT_TOKEN" width="400" height="600" title="AI Assistant"></iframe>
```

The React public assistant route is `/assistant/:assistantToken`; it renders the visitor chat UI. `POST /api/ask` requires `X-Assistant-Token` and a JSON `{ "question": "..." }` body.

## Reviewer test flow

1. Register Organization A and upload a text-based PDF.
2. Register Organization B and upload different content. Confirm each dashboard lists only its own files.
3. Ask each public assistant a question unique to the other organization. The assistant must return no relevant knowledge, never the other organization's text or references.
4. Upload a PDF made from `sample-data/Merit_Scholarship_Policy.txt`. Test the three questions in that file to verify eligible, ineligible, and human-review outcomes.
5. Open Usage and confirm result counts. Regenerate the assistant token, then confirm the old URL receives an invalid-token response.

Run deterministic policy tests with `go test ./service`. To run the PostgreSQL isolation test against a disposable local database:

```powershell
$env:TEST_DATABASE_URL = "postgres://postgres:postgres@localhost:5432/knowledge_assistant?sslmode=disable"
go test ./Repository -run TenantIsolation
```
