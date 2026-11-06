package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
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
	QuestionType   string         `json:"question_type"`
	PolicyCategory string         `json:"policy_category"`
	RequiredFacts  []string       `json:"required_facts"`
	ProvidedFacts  map[string]any `json:"provided_facts"`
	MissingFacts   []string       `json:"missing_facts"`
	NeedsDecision  bool           `json:"needs_decision"`
}

type policyRules struct {
	Category                  string
	MinimumGPA                *float64
	AllowedYears              []int
	DisallowSportsScholarship bool
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
	prompt := `Return JSON only: {"question_type":"direct_information|policy|procedural|multi_condition_decision|comparison|no_answer","policy_category":"string","required_facts":["string"],"provided_facts":{},"missing_facts":["string"],"needs_decision":true}. Use only evidence. Do not decide eligibility and do not invent facts.`
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
	if strings.Contains(q, "eligible") || strings.Contains(q, "qualify") || strings.Contains(q, "can i") || strings.Contains(q, "am i") {
		u.QuestionType = "multi_condition_decision"
		u.NeedsDecision = true
		u.PolicyCategory = "policy"
	}
	return u
}
func evaluate(q string, u understanding, e string) models.Decision {
	d := models.Decision{QuestionType: u.QuestionType, Category: u.PolicyCategory, RequiredFacts: uniqueStrings(u.RequiredFacts), ProvidedFacts: map[string]any{}, MissingFacts: uniqueStrings(u.MissingFacts)}
	if !u.NeedsDecision {
		return d
	}
	rules := extractPolicyRules(e, u.PolicyCategory)
	if rules.Category != "" {
		d.Category = rules.Category
	}
	if d.Category == "" {
		d.Category = "policy"
	}
	facts := extractQuestionFacts(q)
	for k, v := range facts {
		d.ProvidedFacts[k] = v
	}
	checked, passed := 0, 0
	if rules.MinimumGPA != nil {
		d.RequiredFacts = appendUnique(d.RequiredFacts, "gpa")
		checked++
		gpa, ok := facts["gpa"].(float64)
		if !ok {
			d.MissingFacts = appendUnique(d.MissingFacts, "gpa")
		} else if gpa >= *rules.MinimumGPA {
			passed++
			d.Reasons = append(d.Reasons, fmt.Sprintf("GPA %.2f meets the minimum %.2f.", gpa, *rules.MinimumGPA))
		} else {
			d.Reasons = append(d.Reasons, fmt.Sprintf("GPA %.2f is below the minimum %.2f.", gpa, *rules.MinimumGPA))
		}
	}
	if len(rules.AllowedYears) > 0 {
		d.RequiredFacts = appendUnique(d.RequiredFacts, "academic_year")
		checked++
		year, ok := facts["academic_year"].(int)
		if !ok {
			d.MissingFacts = appendUnique(d.MissingFacts, "academic_year")
		} else if containsInt(rules.AllowedYears, year) {
			passed++
			d.Reasons = append(d.Reasons, fmt.Sprintf("Academic year %d is permitted by the policy.", year))
		} else {
			d.Reasons = append(d.Reasons, fmt.Sprintf("Academic year %d is not permitted by the policy.", year))
		}
	}
	if rules.DisallowSportsScholarship {
		d.RequiredFacts = appendUnique(d.RequiredFacts, "existing_sports_scholarship")
		checked++
		sports, ok := facts["existing_sports_scholarship"].(bool)
		if !ok {
			d.MissingFacts = appendUnique(d.MissingFacts, "existing_sports_scholarship")
		} else if !sports {
			passed++
			d.Reasons = append(d.Reasons, "No existing sports scholarship conflicts with this policy.")
		} else {
			d.Reasons = append(d.Reasons, "An existing sports scholarship conflicts with this policy.")
		}
	}
	d.RequiredFacts = uniqueStrings(d.RequiredFacts)
	d.MissingFacts = uniqueStrings(d.MissingFacts)
	if checked == 0 {
		d.Reasons = append(d.Reasons, "The retrieved policy does not expose machine-readable eligibility conditions.")
		d.Score = 0
		return d
	}
	d.Score = float64(passed) / float64(checked)
	if len(d.MissingFacts) == 0 {
		eligible := passed == checked
		d.Eligible = &eligible
	}
	return d
}

func extractPolicyRules(evidence, fallbackCategory string) policyRules {
	lower := strings.ToLower(evidence)
	rules := policyRules{Category: fallbackCategory}
	if strings.Contains(lower, "scholarship") {
		rules.Category = "scholarship"
	}
	if strings.Contains(lower, "merit scholarship") {
		rules.Category = "merit_scholarship"
	}
	if m := regexp.MustCompile(`(?i)gpa(?:\s+of|\s+is|\s+must\s+be|\s+at\s+least|\s+minimum)?\s*(?:of\s*)?(\d(?:\.\d+)?)`).FindStringSubmatch(evidence); len(m) == 2 {
		if value, err := strconv.ParseFloat(m[1], 64); err == nil {
			rules.MinimumGPA = &value
		}
	}
	for _, m := range regexp.MustCompile(`(?i)(?:year|years)\s*(\d)`).FindAllStringSubmatch(evidence, -1) {
		if year, err := strconv.Atoi(m[1]); err == nil {
			rules.AllowedYears = appendUniqueInt(rules.AllowedYears, year)
		}
	}
	rules.DisallowSportsScholarship = strings.Contains(lower, "sports scholarship") && (strings.Contains(lower, "not eligible") || strings.Contains(lower, "cannot") || strings.Contains(lower, "must not") || strings.Contains(lower, "ineligible"))
	return rules
}

func extractQuestionFacts(question string) map[string]any {
	lower := strings.ToLower(question)
	facts := map[string]any{}
	if m := regexp.MustCompile(`(?i)(\d(?:\.\d+)?)\s*(?:gpa|cgpa)`).FindStringSubmatch(question); len(m) == 2 {
		if value, err := strconv.ParseFloat(m[1], 64); err == nil {
			facts["gpa"] = value
		}
	}
	if m := regexp.MustCompile(`(?i)(?:year\s*)([1-9])|([1-9])(?:st|nd|rd|th)\s+year`).FindStringSubmatch(question); len(m) == 3 {
		raw := m[1]
		if raw == "" {
			raw = m[2]
		}
		if year, err := strconv.Atoi(raw); err == nil {
			facts["academic_year"] = year
		}
	}
	if strings.Contains(lower, "sports scholarship") {
		facts["existing_sports_scholarship"] = !(strings.Contains(lower, "no sports scholarship") || strings.Contains(lower, "without a sports scholarship") || strings.Contains(lower, "do not have a sports scholarship"))
	}
	return facts
}

func uniqueStrings(items []string) []string {
	out := []string{}
	for _, item := range items {
		if strings.TrimSpace(item) != "" && !containsString(out, item) {
			out = append(out, item)
		}
	}
	return out
}
func appendUnique(items []string, value string) []string {
	if !containsString(items, value) {
		return append(items, value)
	}
	return items
}
func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
func appendUniqueInt(items []int, value int) []int {
	if !containsInt(items, value) {
		return append(items, value)
	}
	return items
}
func containsInt(items []int, value int) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
func (s AskService) generate(ctx context.Context, q, e string, d models.Decision, refs []models.Reference, outcome string) (string, error) {
	if outcome == "human_review" {
		return "More information is required before this policy decision can be made. Please provide: " + strings.Join(d.MissingFacts, ", ") + ".", nil
	}
	if s.Config.LLMAPIKey == "" {
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
