package engine

import (
	"context"
	"time"

	"github.com/jonathanhecl/vWriter/internal/ollama"
	"github.com/jonathanhecl/vWriter/internal/prompt"
)

// runAuditAndRepair audits the generated text and performs at most one narrow
// repair pass. A failed or still-invalid repair keeps the original draft.
func (e *Engine) runAuditAndRepair(ctx context.Context, client *ollama.Client, model string,
	assembled *prompt.Assembled, plan *prompt.Plan, text string,
	onPhase func(string), onProgress func(int),
) *Result {
	audit := enrichAudit(assembled, text)
	result := &Result{Prompt: text, Audit: audit, Plan: plan}
	expected := prompt.ReferenceTags(userMessageContent(assembled))
	if !audit.RepairRequired {
		return result
	}
	e.setPhase(PhaseRepairing, onPhase)
	result.RepairAttempted = true
	failures := prompt.AuditFailures(audit)
	repairMessages := prompt.NarrowRepairMessages(assembled, text, failures, expected, assembled.Input.DurationSeconds)
	repaired, err := client.Chat(ctx, ollama.ChatRequest{
		Model:    model,
		Messages: toOllamaMessages(repairMessages, nil),
		Options:  chatOptions(plan, nil),
	}, progressCallback(onProgress))
	if err != nil {
		return result
	}
	repairedText := prompt.FinalText(repaired.Content)
	if repairedText == "" {
		return result
	}
	repairedAudit := enrichAudit(assembled, repairedText)
	if !repairedAudit.RepairRequired {
		result.Prompt = repairedText
		result.Audit = repairedAudit
		result.RepairApplied = true
		return result
	}
	// The repair pass did not fully resolve the violations. As a deterministic
	// last-resort guard, strip every line referencing media that was not
	// uploaded (e.g. an invented <Audio N>) so the deliverable never cites
	// assets the user did not send.
	sanitized := prompt.SanitizePrompt(repairedText, expected)
	if sanitized != repairedText {
		result.Prompt = sanitized
		result.Audit = enrichAudit(assembled, sanitized)
		result.RepairApplied = true
	}
	return result
}

// enrichAudit fills the tag, audio-task, and explicit-constraint checks.
func enrichAudit(assembled *prompt.Assembled, text string) *prompt.Audit {
	audit := prompt.AuditPrompt(text, assembled.Input.DurationSeconds,
		prompt.CameraStructureRequested(intentText(assembled)))
	expected := prompt.ReferenceTags(userMessageContent(assembled))
	actual := prompt.ReferenceTags(text)
	for tag := range expected {
		if !actual[tag] {
			audit.MissingReferenceTags = append(audit.MissingReferenceTags, tag)
		}
	}
	for tag := range actual {
		if !expected[tag] {
			audit.UnexpectedReferenceTags = append(audit.UnexpectedReferenceTags, tag)
		}
	}
	audit.UnexpectedAudioTask = prompt.UnexpectedAudioTask(audit.TaskLabel, expected)
	audit.ExplicitConstraintViolations = prompt.ExplicitConstraintViolations(intentText(assembled), text)
	audit.UndeclaredMediaMentions = prompt.UndeclaredMediaMentions(text, expected)
	if len(audit.MissingReferenceTags) > 0 || len(audit.UnexpectedReferenceTags) > 0 ||
		audit.UnexpectedAudioTask || len(audit.ExplicitConstraintViolations) > 0 ||
		len(audit.UndeclaredMediaMentions) > 0 {
		audit.RepairRequired = true
	}
	return audit
}

// intentText is the user-intent source for constraint checks: the creative
// brief for generation, or the current prompt plus instruction for refine.
func intentText(assembled *prompt.Assembled) string {
	if assembled.Input.CreativeBrief != "" {
		return assembled.Input.CreativeBrief
	}
	return assembled.Input.CurrentPrompt + " " + assembled.Input.Instruction
}

// userMessageContent returns the assembled user message text.
func userMessageContent(assembled *prompt.Assembled) string {
	for _, message := range assembled.Messages {
		if message.Role == "user" {
			return message.Content
		}
	}
	return ""
}

// finishMetrics fills token and timing metrics from the final chat response.
func (e *Engine) finishMetrics(result *Result, chat *ollama.ChatResult, started time.Time) {
	result.PromptTokens = chat.PromptEvalCount
	result.OutputTokens = chat.EvalCount
	result.GenerationSeconds = float64(chat.EvalDuration) / 1e9
	result.TotalSeconds = time.Since(started).Seconds()
	if chat.EvalDuration > 0 {
		result.TokensPerSecond = float64(chat.EvalCount) / result.GenerationSeconds
	}
}

// maybeUnload releases the model unless the user keeps it loaded.
func (e *Engine) maybeUnload(client *ollama.Client, model string, keepLoaded bool) {
	if keepLoaded {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = client.Unload(ctx, model)
}

// progressCallback adapts streamed chunks into a token counter callback.
func progressCallback(onProgress func(tokens int)) func(ollama.ChatChunk) error {
	if onProgress == nil {
		return nil
	}
	count := 0
	return func(chunk ollama.ChatChunk) error {
		if chunk.Message.Content != "" || chunk.Message.Thinking != "" {
			count++
			onProgress(count)
		}
		return nil
	}
}

// buildFallbackRequest retries without thinking and a standard output budget.
func buildFallbackRequest(request ollama.ChatRequest) ollama.ChatRequest {
	off := false
	request.Think = &off
	if request.Options != nil {
		request.Options.NumPredict = 1536
	}
	return request
}
