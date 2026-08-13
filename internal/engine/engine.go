// Package engine orchestrates prompt generation and refinement against an
// Ollama server: assemble, plan, chat, audit, and one narrow repair pass.
package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jonathanhecl/vWriter/internal/config"
	"github.com/jonathanhecl/vWriter/internal/media"
	"github.com/jonathanhecl/vWriter/internal/ollama"
	"github.com/jonathanhecl/vWriter/internal/prompt"
)

// Generation phases surfaced to the UI.
const (
	PhaseIdle            = "idle"
	PhaseLoadingModel    = "loading_model"
	PhaseProcessingMedia = "processing_media"
	PhaseGenerating      = "generating"
	PhaseRepairing       = "repairing"
	PhaseCancelling      = "cancelling"
)

// Error is a stable, user-facing engine failure.
type Error struct {
	Code    string
	Message string
	Details any
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// Is reports whether err is an engine Error, optionally with a specific code.
func Is(err error, code string) bool {
	var eerr *Error
	return errors.As(err, &eerr) && (code == "" || eerr.Code == code)
}

// Engine is the single-generation orchestrator of the app.
type Engine struct {
	Store *media.Store

	mu              sync.Mutex
	client          *ollama.Client
	phase           string
	active          bool
	activeCancel    context.CancelFunc
	generationCache map[string]string
}

// NewEngine creates an engine around a media store.
func NewEngine(store *media.Store) *Engine {
	return &Engine{
		Store:           store,
		phase:           PhaseIdle,
		generationCache: map[string]string{},
	}
}

// SetOllamaURL (re)connects the engine to an Ollama server.
func (e *Engine) SetOllamaURL(rawURL string) error {
	client, err := ollama.NewClient(rawURL)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.client = client
	e.mu.Unlock()
	return nil
}

// Client returns the current Ollama client, or OLLAMA_UNAVAILABLE when the
// engine has no configured server yet.
func (e *Engine) Client() (*ollama.Client, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.client == nil {
		return nil, &Error{Code: "OLLAMA_UNAVAILABLE", Message: "No Ollama server is configured yet."}
	}
	return e.client, nil
}

// Phase reports the current generation phase.
func (e *Engine) Phase() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.phase
}

func (e *Engine) setPhase(phase string, onPhase func(string)) {
	e.mu.Lock()
	e.phase = phase
	e.mu.Unlock()
	if onPhase != nil {
		onPhase(phase)
	}
}

// Cancel interrupts the active generation, if any.
func (e *Engine) Cancel() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.active {
		return false
	}
	e.phase = PhaseCancelling
	if e.activeCancel != nil {
		e.activeCancel()
	}
	return true
}

// begin enforces a single active generation and returns a context whose
// cancellation interrupts it, plus the end hook.
func (e *Engine) begin() (context.Context, func(), error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.active {
		return nil, nil, &Error{Code: "GENERATION_BUSY", Message: "Another generation request is already running."}
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.active = true
	e.activeCancel = cancel
	return ctx, func() {
		cancel()
		e.mu.Lock()
		e.active = false
		e.activeCancel = nil
		e.phase = PhaseIdle
		e.mu.Unlock()
	}, nil
}

// CachedGeneration returns the last prompt produced for a session.
func (e *Engine) CachedGeneration(sessionID string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.generationCache[sessionID]
}

// GenerateParams drives one Generate call.
type GenerateParams struct {
	SessionID            string
	Model                string
	CreativeBrief        string
	RefineInstruction    string
	DurationSeconds      float64
	AspectRatio          string
	ContextProfile       string
	Thinking             bool
	KeepModelLoaded      bool
	Seed                 *int
	SystemPromptOverride *string
	OnPhase              func(phase string)
	OnProgress           func(tokens int)
}

// RefineParams drives one Refine call.
type RefineParams struct {
	SessionID            string
	Model                string
	CurrentPrompt        string
	Instruction          string
	ContextProfile       string
	Thinking             bool
	KeepModelLoaded      bool
	Seed                 *int
	SystemPromptOverride *string
	OnPhase              func(phase string)
	OnProgress           func(tokens int)
}

// Result is the outcome of one generation or refinement.
type Result struct {
	Prompt            string        `json:"prompt"`
	Audit             *prompt.Audit `json:"audit"`
	Plan              *prompt.Plan  `json:"plan"`
	PromptTokens      int           `json:"prompt_tokens"`
	OutputTokens      int           `json:"output_tokens"`
	GenerationSeconds float64       `json:"generation_seconds"`
	TotalSeconds      float64       `json:"total_seconds"`
	TokensPerSecond   float64       `json:"tokens_per_second"`
	ThinkingFallback  bool          `json:"thinking_fallback"`
	RepairAttempted   bool          `json:"repair_attempted"`
	RepairApplied     bool          `json:"repair_applied"`
}

// Generate runs a fresh full-reference generation.
func (e *Engine) Generate(params GenerateParams) (*Result, error) {
	client, err := e.Client()
	if err != nil {
		return nil, err
	}
	ctx, end, err := e.begin()
	if err != nil {
		return nil, err
	}
	defer end()
	started := time.Now()

	e.setPhase(PhaseLoadingModel, params.OnPhase)
	if _, err := client.RequireVision(ctx, params.Model); err != nil {
		return nil, err
	}
	assembled, err := prompt.AssembleRequest(prompt.GenerateRequest{
		Manifest:             e.Store.Manifest(params.SessionID),
		CreativeBrief:        params.CreativeBrief,
		RefineInstruction:    params.RefineInstruction,
		DurationSeconds:      params.DurationSeconds,
		AspectRatio:          params.AspectRatio,
		SystemPromptOverride: params.SystemPromptOverride,
	})
	if err != nil {
		return nil, err
	}
	return e.run(ctx, client, assembled, runParams{
		Model:           params.Model,
		ContextProfile:  params.ContextProfile,
		Thinking:        params.Thinking,
		KeepModelLoaded: params.KeepModelLoaded,
		Seed:            params.Seed,
		OnPhase:         params.OnPhase,
		OnProgress:      params.OnProgress,
		SessionID:       params.SessionID,
		LogEvent:        "generate_succeeded",
	}, started)
}

// runParams carries the execution options shared by the generation pipeline.
type runParams struct {
	Model           string
	ContextProfile  string
	Thinking        bool
	KeepModelLoaded bool
	Seed            *int
	OnPhase         func(string)
	OnProgress      func(int)
	SessionID       string
	LogEvent        string
}

// run executes the shared generation pipeline: context planning, chat request
// building (with media attached), chat, audit, and one narrow repair pass.
func (e *Engine) run(ctx context.Context, client *ollama.Client, assembled *prompt.Assembled, rp runParams, started time.Time) (*Result, error) {
	plan, err := prompt.PlanContext(assembled, rp.ContextProfile, rp.Thinking)
	if err != nil {
		return nil, err
	}
	e.setPhase(PhaseProcessingMedia, rp.OnPhase)
	request, err := buildChatRequest(rp.Model, assembled, plan, rp.Thinking, rp.Seed)
	if err != nil {
		return nil, err
	}
	e.setPhase(PhaseGenerating, rp.OnPhase)
	chat, err := client.Chat(ctx, request, progressCallback(rp.OnProgress))
	if err != nil {
		return nil, err
	}
	fallbackUsed := false
	if rp.Thinking && (chat.DoneReason == "length" || chat.Content == "") {
		fallbackUsed = true
		chat, err = client.Chat(ctx, buildFallbackRequest(request), progressCallback(rp.OnProgress))
		if err != nil {
			return nil, err
		}
	}
	text := prompt.FinalText(chat.Content)
	if text == "" {
		return nil, &Error{Code: "EMPTY_GENERATION", Message: "The model did not produce a final prompt."}
	}

	result := e.runAuditAndRepair(ctx, client, rp.Model, assembled, plan, text, rp.OnPhase, rp.OnProgress)
	result.ThinkingFallback = fallbackUsed
	e.finishMetrics(result, chat, started)

	e.mu.Lock()
	e.generationCache[rp.SessionID] = result.Prompt
	e.mu.Unlock()
	e.maybeUnload(client, rp.Model, rp.KeepModelLoaded)
	config.LogEvent(rp.LogEvent, map[string]any{
		"model": rp.Model, "total_seconds": result.TotalSeconds,
		"output_tokens": result.OutputTokens, "repair_applied": result.RepairApplied,
	})
	return result, nil
}

// ContinuationParams drives one GenerateContinuation call: the next part of a
// multi-part story.
type ContinuationParams struct {
	SessionID            string
	Model                string
	PartBrief            string
	StoryBrief           string
	DurationSeconds      float64
	AspectRatio          string
	PreviousPrompt       string
	PreviousEnding       string
	SourceVideoLabel     string
	RefineInstruction    string
	ContextProfile       string
	Thinking             bool
	KeepModelLoaded      bool
	Seed                 *int
	SystemPromptOverride *string
	OnPhase              func(phase string)
	OnProgress           func(tokens int)
}

// GenerateContinuation generates the next part of a multi-part story as a
// video continuation of the previous part: the same reference media is
// re-attached and the previous ending is injected as a virtual first frame.
func (e *Engine) GenerateContinuation(params ContinuationParams) (*Result, error) {
	client, err := e.Client()
	if err != nil {
		return nil, err
	}
	ctx, end, err := e.begin()
	if err != nil {
		return nil, err
	}
	defer end()
	started := time.Now()

	e.setPhase(PhaseLoadingModel, params.OnPhase)
	if _, err := client.RequireVision(ctx, params.Model); err != nil {
		return nil, err
	}
	assembled, err := prompt.AssembleContinuation(prompt.ContinuationRequest{
		Manifest:             e.Store.Manifest(params.SessionID),
		PartBrief:            params.PartBrief,
		StoryBrief:           params.StoryBrief,
		DurationSeconds:      params.DurationSeconds,
		AspectRatio:          params.AspectRatio,
		PreviousPrompt:       params.PreviousPrompt,
		PreviousEnding:       params.PreviousEnding,
		SourceVideoLabel:     params.SourceVideoLabel,
		RefineInstruction:    params.RefineInstruction,
		SystemPromptOverride: params.SystemPromptOverride,
	})
	if err != nil {
		return nil, err
	}
	return e.run(ctx, client, assembled, runParams{
		Model:           params.Model,
		ContextProfile:  params.ContextProfile,
		Thinking:        params.Thinking,
		KeepModelLoaded: params.KeepModelLoaded,
		Seed:            params.Seed,
		OnPhase:         params.OnPhase,
		OnProgress:      params.OnProgress,
		SessionID:       params.SessionID,
		LogEvent:        "extend_succeeded",
	}, started)
}

// Refine rewrites the current prompt with a revision instruction, without
// re-sending media.
func (e *Engine) Refine(params RefineParams) (*Result, error) {
	client, err := e.Client()
	if err != nil {
		return nil, err
	}
	ctx, end, err := e.begin()
	if err != nil {
		return nil, err
	}
	defer end()
	started := time.Now()

	e.setPhase(PhaseLoadingModel, params.OnPhase)
	if _, err := client.RequireVision(ctx, params.Model); err != nil {
		return nil, err
	}
	assembled, err := prompt.AssembleRefinement(prompt.RefineRequest{
		Manifest:             e.Store.Manifest(params.SessionID),
		CurrentPrompt:        params.CurrentPrompt,
		Instruction:          params.Instruction,
		CachedObservation:    e.CachedGeneration(params.SessionID),
		SystemPromptOverride: params.SystemPromptOverride,
	})
	if err != nil {
		return nil, err
	}
	plan, err := prompt.PlanContext(assembled, params.ContextProfile, params.Thinking)
	if err != nil {
		return nil, err
	}

	e.setPhase(PhaseGenerating, params.OnPhase)
	request, err := buildChatRequest(params.Model, assembled, plan, params.Thinking, params.Seed)
	if err != nil {
		return nil, err
	}
	chat, err := client.Chat(ctx, request, progressCallback(params.OnProgress))
	if err != nil {
		return nil, err
	}
	fallbackUsed := false
	if params.Thinking && (chat.DoneReason == "length" || chat.Content == "") {
		fallbackUsed = true
		chat, err = client.Chat(ctx, buildFallbackRequest(request), progressCallback(params.OnProgress))
		if err != nil {
			return nil, err
		}
	}
	text := prompt.FinalText(chat.Content)
	if text == "" {
		return nil, &Error{Code: "EMPTY_GENERATION", Message: "The model did not produce a final prompt."}
	}

	result := e.runAuditAndRepair(ctx, client, params.Model, assembled, plan, text, params.OnPhase, params.OnProgress)
	result.ThinkingFallback = fallbackUsed
	e.finishMetrics(result, chat, started)

	e.mu.Lock()
	e.generationCache[params.SessionID] = result.Prompt
	e.mu.Unlock()
	e.maybeUnload(client, params.Model, params.KeepModelLoaded)
	config.LogEvent("refine_succeeded", map[string]any{
		"model": params.Model, "total_seconds": result.TotalSeconds,
	})
	return result, nil
}
