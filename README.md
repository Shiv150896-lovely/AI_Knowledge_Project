# Multi-Tenant AI Knowledge & Decision Assistant

Gin + PostgreSQL/pgx MVP implementing organization-isolated PDF knowledge bases, public assistant URLs, embedded chat, grounded references, LLM understanding/generation, and structured decisions.

## Run locally

1. Create a PostgreSQL database named `knowledge_assistant`.
2. Copy `.env.example` to `.env`, then set `DATABASE_URL`, `JWT_SECRET`, and an OpenAI-compatible `LLM_API_KEY`.
3. Install dependencies and start:

```powershell
go mod tidy
go run .
```

Tables are created automatically at startup. The API is served at `http://localhost:8080`.

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

The public assistant route is `GET /assistant/:assistantToken`; it renders the visitor chat UI. `POST /api/ask` requires `X-Assistant-Token` and a JSON `{ "question": "..." }` body.
