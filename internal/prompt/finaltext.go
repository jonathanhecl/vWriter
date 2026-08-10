package prompt

import "strings"

// FinalText cleans raw model output into the deliverable prompt text.
// Thinking content arrives in a separate Ollama field, so only stray special
// tokens need stripping here.
func FinalText(response string) string {
	response = strings.ReplaceAll(response, "<|end_of_turn|>", "")
	response = strings.ReplaceAll(response, "<eos>", "")
	return strings.TrimSpace(response)
}
