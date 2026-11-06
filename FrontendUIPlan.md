# Frontend UI Plan — Multi-Tenant AI Knowledge & Decision Assistant

## Scope

Build two UI experiences using the existing backend API without changing backend code:

1. **Organization dashboard** — account access, knowledge-base management, assistant URL/embed code, and usage.
2. **Public embedded assistant** — a no-login chat interface for a single organization assistant token.

The UI must never allow an assistant token to access dashboard screens or administrative API calls. Dashboard requests use the JWT only; public chat requests use the assistant token only.

## UI routes

| Route | Access | Screen purpose |
|---|---|---|
| `/register` | Public | Create organization account and receive dashboard session. |
| `/login` | Public | Sign in to the organization dashboard. |
| `/dashboard/knowledge-base` | JWT | List, upload, delete, and replace organization PDFs. |
| `/dashboard/assistant` | JWT | Show assistant URL, copy embed code, and regenerate token. |
| `/dashboard/usage` | JWT | Show questions asked, answered, unable-to-answer, and human-review counts. |
| `/assistant/:assistantToken` | Public | Visitor chat interface; no dashboard navigation or login. |

After registration or login, redirect to `/dashboard/knowledge-base`. The dashboard header shows organization name and a logout action. Logout removes the stored JWT, calls `POST /api/auth/logout`, and redirects to `/login`.

## Dashboard layout

```text
┌───────────────────────────────────────────────────────────────┐
│ Organization name                              Logout          │
├─────────────────┬─────────────────────────────────────────────┤
│ Knowledge Base  │ Page content                                 │
│ Assistant       │                                             │
│ Usage           │                                             │
└─────────────────┴─────────────────────────────────────────────┘
```

Use a responsive layout: persistent left navigation on desktop and a compact top navigation on narrow screens. Use clear empty, loading, success, and error states on every screen.

## Screen requirements

### Registration and login

**Registration fields:** organization name, email, password.

- Validate required values before submit; password must be at least eight characters.
- Call `POST /api/auth/register` with JSON.
- Store `access_token` only for the active dashboard session.
- Show the returned assistant URL/embed code on success, then route to Knowledge Base.
- Show the backend error message for duplicate email or invalid fields.

**Login fields:** email, password.

- Call `POST /api/auth/login` with JSON.
- Store `access_token`; fetch `GET /api/organization` to show organization name.
- On `401`, show “Invalid email or password.”

### Knowledge Base

Show:

- Document count as `n / 10`.
- Upload control with visible limits: PDF only, maximum 10 MB, maximum 20 pages, maximum 10 documents.
- A table or card list: filename, size, page count, processing status, uploaded date, delete action.
- An empty state that explains the organization needs a PDF before visitors can receive answers.

Upload interaction:

1. Reject non-PDF files and files larger than 10 MB in the browser before submission.
2. Send one file using `POST /api/documents` as `multipart/form-data`.
3. Refresh the document list after success.
4. Display mapped error messages for invalid PDF, document limit, size limit, page limit, and processing failure.

Delete interaction:

1. Ask for confirmation, naming the document.
2. Send `DELETE /api/documents/:documentId`.
3. Remove the document from the list only after success.
4. A replacement is a delete followed by a new upload, so the organization never exceeds the 10-document limit.

### Assistant

Show:

- Read-only assistant URL with Copy URL action.
- Read-only iframe embed code with Copy embed code action.
- An embedded preview or “Open assistant” link.
- Regenerate token action with confirmation: existing embedded assistants will stop working after regeneration.

After confirmation, call `POST /api/assistant/regenerate-token`, then replace the displayed URL/embed code with the returned values. Never display dashboard credentials, document content, or JWT values in this screen.

### Usage

Display the four backend values as simple count cards:

- Questions asked
- Answered
- Unable to answer
- Human review

Use `GET /api/usage`. This screen has no document content and no public assistant token.

### Public assistant chat

The visitor experience must be simple and independent of dashboard authentication.

- Header: organization assistant label only.
- Scrollable conversation area with user messages and assistant messages.
- Single-line question input and Send action; disable Send while the request is in progress.
- Each successful response shows answer text followed by source references in this exact form: `Document.pdf — Page 4`.
- Preserve the backend outcome in the UI:
  - `answered`: show answer and references.
  - `unable_to_answer`: show that the knowledge base does not contain the required information.
  - `human_review`: show that more information is required before the policy decision can be made.
- Do not render decision internals as the answer; the visitor sees the grounded result and sources.

For the existing backend route, use the token in the page URL only to send `X-Assistant-Token` to `POST /api/ask`. Do not attach the dashboard JWT and do not call any `/api` dashboard endpoint from this page.

## Existing API integration map

| UI action | Method and path | Headers | Body/form | Success UI | Error UI |
|---|---|---|---|---|---|
| Register | `POST /api/auth/register` | `Content-Type: application/json` | organization_name, email, password | Save JWT; show assistant details; enter dashboard | Validation or duplicate-account message |
| Login | `POST /api/auth/login` | `Content-Type: application/json` | email, password | Save JWT; enter dashboard | Invalid credentials |
| Load account | `GET /api/organization` | `Authorization: Bearer <jwt>` | None | Organization name in header | Clear session and return to login on `401` |
| List PDFs | `GET /api/documents` | `Authorization: Bearer <jwt>` | None | Document list and count | List error state |
| Upload PDF | `POST /api/documents` | `Authorization: Bearer <jwt>` | `multipart/form-data`, `file` | Refresh documents | Limit, file, page, or processing error |
| Delete PDF | `DELETE /api/documents/:documentId` | `Authorization: Bearer <jwt>` | None | Remove document | Not-found or delete error |
| Load assistant | `GET /api/assistant` | `Authorization: Bearer <jwt>` | None | URL and embed code | Assistant load error |
| Regenerate assistant | `POST /api/assistant/regenerate-token` | `Authorization: Bearer <jwt>` | None | Replace URL and embed code | Regeneration error |
| Load usage | `GET /api/usage` | `Authorization: Bearer <jwt>` | None | Count cards | Usage load error |
| Logout | `POST /api/auth/logout` | `Authorization: Bearer <jwt>` | None | Remove JWT; route to login | Remove local JWT even if request fails |
| Ask question | `POST /api/ask` | `Content-Type: application/json`, `X-Assistant-Token: <assistant_token>` | question | Grounded answer + references | Invalid token, empty question, no knowledge, LLM/decision failure |

## End-to-end UI acceptance flow

1. Register Organization A and enter its dashboard.
2. Upload a valid PDF and confirm its filename, page count, and `ready` status appear in Knowledge Base.
3. Copy Organization A’s embed code into a simple external HTML page and open it.
4. Ask direct-information, policy, procedural, multi-condition decision, comparison, and no-answer questions.
5. Confirm answers show document/page references when knowledge exists, and the no-answer message when it does not.
6. Register Organization B, upload different documents, and verify its dashboard and embedded assistant show only Organization B data.
7. Confirm Organization A’s assistant token cannot open dashboard pages or call dashboard APIs; confirm Organization B’s assistant never returns Organization A sources.
8. Regenerate Organization A’s assistant token and confirm the old embedded assistant receives the invalid-token error while the new URL works.

## Frontend error messages

| Backend condition | UI message |
|---|---|
| Invalid PDF | “Upload a valid PDF document.” |
| File exceeds 10 MB | “This PDF exceeds the 10 MB limit.” |
| PDF exceeds 20 pages | “This PDF exceeds the 20-page limit.” |
| Ten-document limit | “Delete a document before uploading another PDF.” |
| Processing failure | “This document could not be processed. Try another PDF.” |
| Invalid assistant token | “This assistant link is invalid or has been regenerated.” |
| Empty question | “Enter a question to continue.” |
| No relevant knowledge | “The knowledge base does not contain the required information.” |
| LLM or decision failure | “The assistant could not process this question. Please try again.” |
