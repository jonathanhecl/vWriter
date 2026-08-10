package prompt

import (
	"fmt"
	"math"
	"strings"
)

// Context profiles map to the num_ctx option sent to Ollama.
var contextProfiles = map[string]int{
	"low":      8192,
	"standard": 16384,
	"extended": 24576,
}

var contextProfileAliases = map[string]string{
	"8k": "low", "16k": "standard", "24k": "extended",
}

const (
	contextSafetyTokens        = 512
	estimatedVisualTokens      = 280
	chatTemplateOverheadTokens = 384
	standardOutputTokens       = 1536
	thinkingOutputTokens       = 6144
)

// Plan is the resolved context and output budget for one request.
type Plan struct {
	RequestedProfile      string `json:"requested_context_profile"`
	Profile               string `json:"context_profile"`
	ContextTokens         int    `json:"context_tokens"`
	Thinking              bool   `json:"thinking"`
	EstimatedTextTokens   int    `json:"estimated_text_tokens"`
	EstimatedInputTokens  int    `json:"estimated_input_tokens"`
	VisualInputCount      int    `json:"visual_input_count"`
	MaxOutputTokens       int    `json:"max_output_tokens"`
	ReservedOutputTokens  int    `json:"reserved_output_tokens"`
	ThinkingBudgetReduced bool   `json:"thinking_budget_reduced"`
}

// EstimateTextTokens is a conservative model-family estimate needing no
// tokenizer or model load.
func EstimateTextTokens(text string) int {
	return int(math.Ceil(float64(max(len(text), len([]byte(text)))) / 3.0))
}

func assembledText(assembled *Assembled) string {
	var parts []string
	for _, message := range assembled.Messages {
		parts = append(parts, message.Content)
	}
	return strings.Join(parts, "\n\n")
}

// PlanContext resolves the requested profile ("auto", low/8k, standard/16k,
// extended/24k) into a concrete budget. Auto starts at standard and escalates
// to extended when the estimate requires it; a manual choice is never
// silently overridden.
func PlanContext(assembled *Assembled, requested string, thinking bool) (*Plan, error) {
	requestedValue := strings.ToLower(strings.TrimSpace(requested))
	if requestedValue == "" {
		requestedValue = "auto"
	}
	if alias, ok := contextProfileAliases[requestedValue]; ok {
		requestedValue = alias
	}
	automatic := requestedValue == "auto"
	profile := requestedValue
	if automatic {
		profile = "standard"
	}
	if _, ok := contextProfiles[profile]; !ok {
		return nil, &Error{
			Code:    "INVALID_CONTEXT_PROFILE",
			Message: "Context must be Auto, Low 8K, Standard 16K, or Extended 24K.",
			Details: map[string]any{"context_profile": requested},
		}
	}
	if profile == "low" && thinking {
		return nil, &Error{
			Code:    "THINKING_DISABLED_LOW_CONTEXT",
			Message: "Thinking is unavailable in Low 8K context. Switch to Standard 16K or turn Thinking off.",
			Details: map[string]any{"context_profile": profile, "context_tokens": contextProfiles[profile], "suggested_context_profile": "standard"},
		}
	}

	visualCount := len(assembled.MediaInputs)
	estimatedText := EstimateTextTokens(assembledText(assembled))
	estimatedInput := estimatedText + visualCount*estimatedVisualTokens + chatTemplateOverheadTokens
	minimumRequired := estimatedInput + standardOutputTokens + contextSafetyTokens

	if automatic && minimumRequired > contextProfiles[profile] {
		profile = "extended"
	}
	contextTokens := contextProfiles[profile]
	if minimumRequired > contextTokens {
		suggested := ""
		for _, name := range []string{"low", "standard", "extended"} {
			if contextProfiles[name] >= minimumRequired && contextProfiles[name] > contextTokens {
				suggested = name
				break
			}
		}
		suggestion := "Remove references or shorten the creative brief."
		if suggested != "" {
			suggestion = fmt.Sprintf("Switch to %s context or remove references.", strings.ToUpper(suggested[:1])+suggested[1:])
		}
		return nil, &Error{
			Code:    "CONTEXT_BUDGET_EXCEEDED",
			Message: "This request does not leave enough context for a complete prompt.",
			Details: map[string]any{
				"estimated_input_tokens":    estimatedInput,
				"minimum_output_tokens":     standardOutputTokens,
				"safety_tokens":             contextSafetyTokens,
				"context_profile":           profile,
				"context_tokens":            contextTokens,
				"suggested_context_profile": suggested,
				"suggestion":                suggestion,
			},
		}
	}

	availableOutput := contextTokens - estimatedInput - contextSafetyTokens
	maxOutput := standardOutputTokens
	if thinking {
		maxOutput = min(thinkingOutputTokens, availableOutput)
	}
	return &Plan{
		RequestedProfile:      requested,
		Profile:               profile,
		ContextTokens:         contextTokens,
		Thinking:              thinking,
		EstimatedTextTokens:   estimatedText,
		EstimatedInputTokens:  estimatedInput,
		VisualInputCount:      visualCount,
		MaxOutputTokens:       maxOutput,
		ReservedOutputTokens:  maxOutput + contextSafetyTokens,
		ThinkingBudgetReduced: thinking && maxOutput < thinkingOutputTokens,
	}, nil
}
