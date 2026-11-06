package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"ai-knowledge-assistant/Repository"
	"ai-knowledge-assistant/Utils"
	"ai-knowledge-assistant/config"
	"ai-knowledge-assistant/models"
)

type AskService struct {
	Repo   repository.Repository
	LLM    LLM
	Config config.Config
}
type understanding struct {
	QuestionType   string   `json:"question_type"`
	PolicyCategory string   `json:"policy_category"`
	RequiredFacts  []string `json:"required_facts"`
	MissingFacts   []string `json:"missing_facts"`
	NeedsDecision  bool     `json:"needs_decision"`
}

func (s AskService) Ask(ctx context.Context, rawToken, question string) (map[string]any, int, error) {
	assistant, org, err := s.Repo.ResolveAssistant(ctx, utils.HashToken(rawToken))
	if err != nil {
		return nil, 401, fmt.Errorf("invalid assistant token")
	}
	question = utils.CleanQuestion(question)
	if question == "" {
		return nil, 400, fmt.Errorf("question is required")
	}
	if isGreeting(question) {
		_ = s.Repo.Log(ctx, org, assistant, "answered")
		return map[string]any{
			"answer":     "Hello! Ask a question about this organization's uploaded documents.",
			"outcome":    "answered",
			"references": []models.Reference{},
		}, 200, nil
	}
	chunks, err := s.Repo.Search(ctx, org, question)
	if err != nil {
		return nil, 500, err
	}
	if len(chunks) == 0 {
		_ = s.Repo.Log(ctx, org, assistant, "unable_to_answer")
		return map[string]any{"answer": "The knowledge base does not contain the required information.", "outcome": "unable_to_answer", "references": []models.Reference{}}, 422, nil
	}
	refs, evidence := references(chunks), evidence(chunks)
	u := s.understand(ctx, question, evidence)
	decision := evaluate(question, u, evidence)
	outcome := "answered"
	if len(decision.MissingFacts) > 0 {
		outcome = "human_review"
	}
	answer, err := s.generate(ctx, question, evidence, decision, refs, outcome)
	if err != nil {
		return nil, 502, err
	}
	_ = s.Repo.Log(ctx, org, assistant, outcome)
	return map[string]any{"answer": answer, "outcome": outcome, "references": refs, "decision": decision}, 200, nil
}

func isGreeting(question string) bool {
	q := strings.ToLower(strings.TrimSpace(question))
	return q == "hi" || q == "hii" || q == "hello" || q == "hey" || q == "good morning" || q == "good afternoon" || q == "good evening"
}

func references(chunks []models.Chunk) []models.Reference {
	seen := map[string]bool{}
	out := []models.Reference{}
	for _, c := range chunks {
		k := fmt.Sprintf("%s:%d", c.Filename, c.Page)
		if !seen[k] {
			seen[k] = true
			out = append(out, models.Reference{Document: c.Filename, Page: c.Page})
		}
	}
	return out
}
func evidence(chunks []models.Chunk) string {
	var b strings.Builder
	for _, c := range chunks {
		fmt.Fprintf(&b, "[%s — Page %d]\n%s\n", c.Filename, c.Page, c.Content)
	}
	return b.String()
}
func (s AskService) understand(ctx context.Context, q, e string) understanding {
	fallback := heuristic(q)
	if s.Config.LLMAPIKey == "" {
		return fallback
	}
	prompt := `Return JSON only: {"question_type":"direct_information|policy|procedural|multi_condition_decision|comparison|no_answer","policy_category":"string","required_facts":["string"],"missing_facts":["string"],"needs_decision":true}. Use only evidence. Do not invent facts.`
	raw, err := s.LLM.Complete(ctx, prompt, "Question: "+q+"\nEvidence:\n"+e)
	if err != nil {
		return fallback
	}
	var u understanding
	if json.Unmarshal([]byte(jsonBlock(raw)), &u) != nil || u.QuestionType == "" {
		return fallback
	}
	return u
}
func heuristic(q string) understanding {
	q = strings.ToLower(q)
	u := understanding{QuestionType: "direct_information"}
	if strings.Contains(q, "how") || strings.Contains(q, "procedure") {
		u.QuestionType = "procedural"
	}
	if strings.Contains(q, "compare") || strings.Contains(q, "difference") {
		u.QuestionType = "comparison"
	}
	if strings.Contains(q, "eligible") || strings.Contains(q, "qualify") || strings.Contains(q, "can i") {
		u.QuestionType = "multi_condition_decision"
		u.NeedsDecision = true
		u.PolicyCategory = "policy"
	}
	return u
}
func evaluate(q string, u understanding, e string) models.Decision {
	d := models.Decision{QuestionType: u.QuestionType, Category: u.PolicyCategory, MissingFacts: u.MissingFacts}
	if !u.NeedsDecision {
		return d
	}
	supported := strings.TrimSpace(e) != ""
	eligible := supported
	if !strings.Contains(strings.ToLower(q), "gpa") && strings.Contains(strings.ToLower(e), "gpa") {
		d.MissingFacts = append(d.MissingFacts, "gpa")
	}
	if len(d.MissingFacts) > 0 {
		eligible = false
	}
	d.Eligible = &eligible
	d.Score = 0.86
	return d
}
func (s AskService) generate(ctx context.Context, q, e string, d models.Decision, refs []models.Reference, outcome string) (string, error) {
	if s.Config.LLMAPIKey == "" {
		if outcome == "human_review" {
			return "More information is required before this policy decision can be made.", nil
		}
		return relevantExcerpt(q, e), nil
	}
	payload, _ := json.Marshal(map[string]any{"question": q, "evidence": e, "decision_result": d, "references": refs, "outcome": outcome})
	return s.LLM.Complete(ctx, "Answer using only supplied evidence and decision result. Do not invent facts. If insufficient, say the knowledge base does not contain the required information. Return plain answer text.", string(payload))
}

func relevantExcerpt(question, evidence string) string {
	compact := strings.Join(strings.Fields(evidence), " ")
	lower := strings.ToLower(compact)
	ignored := map[string]bool{"about": true, "and": true, "are": true, "can": true, "does": true, "for": true, "from": true, "has": true, "have": true, "how": true, "the": true, "this": true, "what": true, "which": true, "with": true, "your": true}
	for _, raw := range strings.Fields(strings.ToLower(question)) {
		term := strings.Trim(raw, ".,?!:;\"'()[]{}")
		if len(term) <= 2 || ignored[term] {
			continue
		}
		if at := strings.Index(lower, term); at >= 0 {
			start := at - 180
			if start < 0 {
				start = 0
			}
			end := at + 520
			if end > len(compact) {
				end = len(compact)
			}
			prefix, suffix := "", ""
			if start > 0 {
				prefix = "…"
			}
			if end < len(compact) {
				suffix = "…"
			}
			return prefix + strings.TrimSpace(compact[start:end]) + suffix
		}
	}
	return "The retrieved document contains relevant information."
}
func jsonBlock(s string) string { re := regexp.MustCompile(`(?s)\{.*\}`); return re.FindString(s) }
