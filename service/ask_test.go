package service

import "testing"

const scholarshipPolicy = `Merit Scholarship Policy
Applicants must have a GPA of 3.50 or higher. Students in year 2 or year 3 may apply.
Applicants with a sports scholarship are not eligible for this merit scholarship.`

func TestEvaluateCombinesIndependentPolicyRules(t *testing.T) {
	decision := evaluate(
		"Am I eligible for the merit scholarship with a 3.80 GPA in 2nd year and no sports scholarship?",
		understanding{QuestionType: "multi_condition_decision", PolicyCategory: "merit_scholarship", NeedsDecision: true},
		scholarshipPolicy,
	)
	if decision.Eligible == nil || !*decision.Eligible {
		t.Fatalf("expected eligible decision, got %#v", decision)
	}
	if decision.Score != 1 {
		t.Fatalf("expected complete support score, got %v", decision.Score)
	}
	if len(decision.MissingFacts) != 0 {
		t.Fatalf("expected no missing facts, got %v", decision.MissingFacts)
	}
	if len(decision.Reasons) != 3 {
		t.Fatalf("expected three independently evaluated reasons, got %v", decision.Reasons)
	}
}

func TestEvaluateRequestsHumanReviewForMissingPolicyFacts(t *testing.T) {
	decision := evaluate(
		"Can I qualify for the merit scholarship?",
		understanding{QuestionType: "multi_condition_decision", PolicyCategory: "merit_scholarship", NeedsDecision: true},
		scholarshipPolicy,
	)
	want := map[string]bool{"gpa": true, "academic_year": true, "existing_sports_scholarship": true}
	if decision.Eligible != nil {
		t.Fatalf("eligibility must remain undecided while required facts are missing: %#v", decision)
	}
	if len(decision.MissingFacts) != len(want) {
		t.Fatalf("expected all policy facts to be missing, got %v", decision.MissingFacts)
	}
	for _, fact := range decision.MissingFacts {
		if !want[fact] {
			t.Fatalf("unexpected missing fact %q", fact)
		}
	}
}

func TestEvaluateRejectsFailedRuleWithoutHumanReview(t *testing.T) {
	decision := evaluate(
		"Am I eligible with a 3.20 GPA in year 2 and no sports scholarship?",
		understanding{QuestionType: "multi_condition_decision", NeedsDecision: true},
		scholarshipPolicy,
	)
	if decision.Eligible == nil || *decision.Eligible {
		t.Fatalf("expected an ineligible decision, got %#v", decision)
	}
	if len(decision.MissingFacts) != 0 {
		t.Fatalf("failed rules are not missing facts, got %v", decision.MissingFacts)
	}
}
