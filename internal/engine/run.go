package engine

import (
	"context"
	"strings"
	"time"

	"github.com/jonathanhecl/vWriter/internal/ollama"
	"github.com/jonathanhecl/vWriter/internal/prompt"
)

// runAuditAndRepair audits the generated text and applies only a minimal,
// deterministic cleanup: lines that cite media the user did not provide are
// dropped when they are short (retention/definition lines), and a continuation
// is made to name itself and cite its source video when the model omitted
// those. No model-based repair is run — it destroyed valid content — so the
// deliverable is always a real response, never a stripped skeleton.
func (e *Engine) runAuditAndRepair(ctx context.Context, client *ollama.Client, model string,
	assembled *prompt.Assembled, plan *prompt.Plan, text string,
	onPhase func(string), onProgress func(int),
) *Result {
	expected := prompt.ReferenceTags(userMessageContent(assembled))
	// The source video of a continuation is a first-class reference: even
	// though it is not an uploaded asset, its label must always survive the
	// cleanup, so parts keep referencing the previous part's video.
	source := strings.TrimSpace(assembled.Input.SourceVideoLabel)
	if source != "" {
		expected[source] = true
	}
	// Generic placeholders copied from the instructions (<Video N>, <Subject N>)
	// are corrected before the cleanup: <Video N> becomes the real source label.
	text = prompt.FixPlaceholderLabels(text, source)
	sanitized := prompt.SanitizeMediaPrompt(text, expected)
	if strings.TrimSpace(sanitized) == "" {
		sanitized = text
	}
	// A continuation must name itself and cite the previous part's video even
	// when the model omitted them; this deterministic step only adds the
	// missing required reference, never rewriting existing content.
	sanitized = prompt.EnforceContinuationContract(sanitized, source)
	// Shot timestamps must never exceed the configured duration of the part.
	sanitized = prompt.ClampTimestamps(sanitized, assembled.Input.DurationSeconds)
	result := &Result{Prompt: sanitized, Audit: enrichAudit(assembled, sanitized), Plan: plan}
	if sanitized != text {
		result.RepairApplied = true
	}
	return result
}

// enrichAudit fills the tag, subject, audio-task, and explicit-constraint checks.
func enrichAudit(assembled *prompt.Assembled, text string) *prompt.Audit {
	audit := prompt.AuditPrompt(text, assembled.Input.DurationSeconds,
		prompt.CameraStructureRequested(intentText(assembled)))
	request := userMessageContent(assembled)
	expected := prompt.ReferenceTags(request)
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
	// Malformed placeholders such as "<Video None>" never correspond to a
	// real asset and are always treated as invented.
	audit.UnexpectedReferenceTags = append(audit.UnexpectedReferenceTags, prompt.MalformedTags(text)...)
	// Subjects are bounded by the story so far: in a continuation the request
	// carries the previous part's prompt with its subject definitions, so any
	// <Subject N> beyond that set is invented. A fresh generation defines them
	// freely, so the check only applies when a prior subject set exists.
	expectedSubjects := prompt.SubjectTags(request)
	if len(expectedSubjects) > 0 {
		for tag := range prompt.SubjectTags(text) {
			if !expectedSubjects[tag] {
				audit.UnexpectedSubjects = append(audit.UnexpectedSubjects, tag)
			}
		}
	}
	audit.UnexpectedAudioTask = prompt.UnexpectedAudioTask(audit.TaskLabel, expected)
	audit.ExplicitConstraintViolations = prompt.ExplicitConstraintViolations(intentText(assembled), text)
	audit.UndeclaredMediaMentions = prompt.UndeclaredMediaMentions(text, expected)
	if len(audit.MissingReferenceTags) > 0 || len(audit.UnexpectedReferenceTags) > 0 ||
		len(audit.UnexpectedSubjects) > 0 ||
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
