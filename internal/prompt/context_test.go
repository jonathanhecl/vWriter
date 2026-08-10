package prompt

import (
	"strings"
	"testing"
)

func contextRequest(text string, visualCount int) *Assembled {
	if text == "" {
		text = "short brief"
	}
	inputs := make([]MediaInput, visualCount)
	for index := range inputs {
		inputs[index] = MediaInput{Type: "image"}
	}
	return &Assembled{
		Messages: []Message{
			{Role: "system", Content: "guide"},
			{Role: "user", Content: text},
		},
		MediaInputs: inputs,
	}
}

func TestAutoUsesStandardProfile(t *testing.T) {
	plan, err := PlanContext(contextRequest("", 0), "auto", false)
	if err != nil {
		t.Fatalf("PlanContext: %v", err)
	}
	if plan.ContextTokens != 16384 || plan.Profile != "standard" {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.MaxOutputTokens != 1536 || plan.ReservedOutputTokens != 2048 {
		t.Fatalf("output budget = %+v", plan)
	}
}

func TestAutoEscalatesToExtendedWhenRequestNeedsIt(t *testing.T) {
	plan, err := PlanContext(contextRequest(strings.Repeat("x", 40000), 8), "auto", false)
	if err != nil {
		t.Fatalf("PlanContext: %v", err)
	}
	if plan.Profile != "extended" || plan.ContextTokens != 24576 {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestManualLowContextRejectsThinkingBeforeGeneration(t *testing.T) {
	_, err := PlanContext(contextRequest("", 0), "8k", true)
	if !Is(err, "THINKING_DISABLED_LOW_CONTEXT") {
		t.Fatalf("err = %v", err)
	}
}

func TestThinkingBudgetIsDynamic(t *testing.T) {
	plan, err := PlanContext(contextRequest(strings.Repeat("x", 30000), 0), "16k", true)
	if err != nil {
		t.Fatalf("PlanContext: %v", err)
	}
	if plan.MaxOutputTokens >= 6144 || plan.MaxOutputTokens < 1536 {
		t.Fatalf("max output = %d", plan.MaxOutputTokens)
	}
	if !plan.ThinkingBudgetReduced {
		t.Fatal("thinking budget must be marked reduced")
	}
}

func TestPreflightSuggestsLargerProfileWithoutDroppingMedia(t *testing.T) {
	_, err := PlanContext(contextRequest(strings.Repeat("x", 20000), 8), "8k", false)
	if !Is(err, "CONTEXT_BUDGET_EXCEEDED") {
		t.Fatalf("err = %v", err)
	}
	details, ok := err.(*Error).Details.(map[string]any)
	if !ok || details["suggested_context_profile"] != "standard" {
		t.Fatalf("details = %+v", err)
	}
}

func TestInvalidProfileRejected(t *testing.T) {
	if _, err := PlanContext(contextRequest("", 0), "64k", false); !Is(err, "INVALID_CONTEXT_PROFILE") {
		t.Fatalf("err = %v", err)
	}
}

func TestEstimateTextTokens(t *testing.T) {
	if got := EstimateTextTokens(strings.Repeat("x", 300)); got != 100 {
		t.Fatalf("estimate = %d, want 100", got)
	}
}
