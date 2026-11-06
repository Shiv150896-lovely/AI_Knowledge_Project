# Implementation Plan — LLM Reference

## Responsibility split

| Stage | Component | Input | Output |
|---|---|---|---|
| Tenant resolution | Application + PostgreSQL/pgx | `X-Assistant-Token` | Organization ID or invalid-token error. |
| Retrieval | Application + retrieval store | Organization ID, question | Relevant chunks from that organization only, with document/page metadata. |
| Question understanding | LLM | Question and retrieved policy text | Structured question type, category, required facts, and whether decisions are required. |
| Structured decision | System 1/application logic | Extracted facts and retrieved policy rules | Independent choice/classification, boolean eligibility, and optional score results. |
| Combination | Application logic | Decision outputs | Overall outcome and reasons. |
| Answer generation | LLM | Question, retrieved evidence, combined result, references | Concise answer based only on evidence; no invented facts. |

## LLM contract: understand/decompose

Use this only after tenant-scoped retrieval. The LLM must return JSON, not a final answer.

System instruction:

```text
You analyze a visitor question using only the supplied tenant-scoped document excerpts. Return valid JSON. Classify the question as direct_information, policy, procedural, multi_condition_decision, comparison, or no_answer. Identify the policy category and required facts. Set needs_decision true only when structured rules must be evaluated. Do not infer missing facts or use knowledge outside the excerpts.
```

Input:

```json
{"question":"Am I eligible for a merit scholarship with a 3.8 GPA and sports scholarship?","evidence":[{"document":"Scholarship_Rules.pdf","page":4,"text":"..."}]}
```

Expected JSON:

```json
{"question_type":"multi_condition_decision","policy_category":"merit_scholarship","required_facts":["academic_year","gpa","existing_sports_scholarship"],"provided_facts":{"gpa":3.8,"existing_sports_scholarship":true},"missing_facts":["academic_year"],"needs_decision":true}
```

## System 1 decision contracts

Run independent decisions in parallel where inputs are available.

| Decision type | Name | Input | Output |
|---|---|---|---|
| Classification/choice | Policy category | Question-understanding JSON and policy excerpts | `{ "category": "merit_scholarship" }` |
| Boolean | Independent policy checks | Parsed policy rules and question facts | GPA, academic-year, and existing-scholarship pass/fail reasons; combined into `eligible`. |
| Score | Policy support | Number of passed checks / number of required checks | `{ "score": 0.67 }` for two of three satisfied conditions. |

Application logic marks the result `human_review` when a retrieved policy requires facts absent from the question. The final LLM cannot override a boolean rule result. If no relevant policy is retrieved, return `unable_to_answer` instead of human review.

## LLM contract: grounded answer generation

System instruction:

```text
Generate a clear answer using only the supplied evidence and structured decision result. Do not introduce facts, policies, or citations not present in the input. If evidence is insufficient, state that the knowledge base does not contain the required information. Preserve the supplied source references.
```

Input:

```json
{"question":"Am I eligible for a merit scholarship with a 3.8 GPA?","evidence":[{"document":"Scholarship_Rules.pdf","page":4,"text":"..."}],"decision_result":{"category":"merit_scholarship","eligible":true,"reasons":["GPA meets requirement"],"score":0.86},"references":[{"document":"Scholarship_Rules.pdf","page":4}]}
```

Expected JSON:

```json
{"answer":"Based on the scholarship rules, your 3.8 GPA meets the stated GPA requirement. Other listed conditions still apply.","outcome":"answered","references":[{"document":"Scholarship_Rules.pdf","page":4}]}
```

## API orchestration for POST /api/ask

1. Validate `X-Assistant-Token` and resolve `organization_id`.
2. Validate non-empty `question`.
3. Retrieve only chunks matching `organization_id`.
4. If retrieval has no relevant evidence, write an `unable_to_answer` usage record and return `422`.
5. Invoke the understanding contract.
6. If `needs_decision` is true, run the required System 1 decisions, combine their results, and handle missing facts as `human_review` when necessary.
7. Invoke the grounded-answer contract with evidence, combined decision result, and references.
8. Persist only the outcome/count data required for usage; return the answer and references.

## Configuration required in `.env`

```dotenv
APP_PORT=8080
DATABASE_URL=postgres://user:password@localhost:5432/knowledge_assistant
JWT_SECRET=replace_me
LLM_API_KEY=replace_me
LLM_MODEL=replace_me
DOCUMENT_STORAGE_PATH=./storage
APP_BASE_URL=http://localhost:5173
```

Keep `.env` out of version control; provide the same variable names without secrets in `.env.example`.
