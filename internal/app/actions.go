package app

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jonathanhecl/vWriter/internal/config"
	"github.com/jonathanhecl/vWriter/internal/engine"
	"github.com/jonathanhecl/vWriter/internal/media"
	"github.com/jonathanhecl/vWriter/internal/prompt"
)

// durationSeconds maps the 0..1 slider to 1..20 seconds.
func (a *App) durationSeconds() int {
	return 1 + int(a.duration.Value*19+0.5)
}

// refreshModels fetches the installed models and their vision capability.
func (a *App) refreshModels() {
	a.mu.Lock()
	if a.modelsLoading {
		a.mu.Unlock()
		return
	}
	a.modelsLoading = true
	a.modelsError = ""
	a.mu.Unlock()
	go func() {
		client, err := a.engine.Client()
		if err != nil {
			a.setModels(nil, err)
			return
		}
		ctx := context.Background()
		tags, err := client.Tags(ctx)
		if err != nil {
			a.setModels(nil, err)
			return
		}
		entries := []modelEntry{}
		for _, model := range tags {
			info, err := client.Show(ctx, model.Name)
			if err != nil || !info.HasVision() {
				continue
			}
			entries = append(entries, modelEntry{Name: model.Name, Size: model.Details.ParameterSize})
		}
		a.setModels(entries, nil)
	}()
}

func (a *App) setModels(entries []modelEntry, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.modelsLoading = false
	a.models = entries
	if err != nil {
		a.modelsError = errorText(err)
	} else if len(entries) == 0 {
		a.modelsError = "No vision-capable models installed on this server."
	} else {
		a.modelsError = ""
	}
	a.window.Invalidate()
}

// connect applies the URL editor value and reconnects.
func (a *App) connect() {
	url := strings.TrimSpace(a.urlEditor.Text())
	if err := a.engine.SetOllamaURL(url); err != nil {
		a.pushToast(errorText(err), "", true)
		return
	}
	a.cfg.OllamaURL = url
	a.saveConfig()
	a.refreshModels()
}

// selectedModel returns the chosen model name, or "" when none.
func (a *App) selectedModel() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg.Model == "" && len(a.models) > 0 {
		return a.models[0].Name
	}
	for _, model := range a.models {
		if model.Name == a.cfg.Model {
			return model.Name
		}
	}
	if len(a.models) > 0 {
		return a.models[0].Name
	}
	return ""
}

// mediaExtensions lists supported reference file extensions.
var mediaExtensions = []string{
	".jpg", ".jpeg", ".png", ".webp", ".bmp", ".tif", ".tiff",
	".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4v",
	".wav", ".mp3", ".flac", ".m4a", ".ogg", ".aac", ".opus",
}

// addFile opens a single-file picker dialog and registers the selected file.
func (a *App) addFile() {
	go func() {
		single, err := a.explorer.ChooseFile(mediaExtensions...)
		if err != nil || single == nil {
			return // user cancelled or closed dialog
		}
		path := filePath(single)
		single.Close()
		if path == "" {
			a.mu.Lock()
			a.pushToast("Could not resolve file path.", "", true)
			a.mu.Unlock()
			return
		}
		if _, err := a.engine.Store.Add(a.session, path); err != nil {
			a.mu.Lock()
			a.pushToast(errorText(err), errorDetails(err), true)
			a.mu.Unlock()
		} else {
			a.autoSaveCurrentPreset()
		}
		a.window.Invalidate()
	}()
}

// addMedia picks any supported image, video or audio file.
func (a *App) addMedia() {
	go func() {
		files, err := a.explorer.ChooseFiles(mediaExtensions...)
		if err != nil || len(files) == 0 {
			return
		}
		for _, file := range files {
			if file == nil {
				continue
			}
			path := filePath(file)
			file.Close()
			if path == "" {
				continue
			}
			if _, err := a.engine.Store.Add(a.session, path); err != nil {
				a.mu.Lock()
				a.pushToast(errorText(err), errorDetails(err), true)
				a.mu.Unlock()
			} else {
				a.autoSaveCurrentPreset()
			}
		}
		a.window.Invalidate()
	}()
}

// filePath extracts a local filesystem path from a picked file.
func filePath(file io.ReadCloser) string {
	type urier interface{ URI() string }
	type namer interface{ Name() string }
	type pather interface{ Path() string }

	var raw string
	if f, ok := file.(*os.File); ok {
		raw = f.Name()
	} else if u, ok := file.(urier); ok {
		raw = u.URI()
	} else if p, ok := file.(pather); ok {
		raw = p.Path()
	} else if n, ok := file.(namer); ok {
		raw = n.Name()
	} else if s, ok := file.(fmt.Stringer); ok {
		raw = s.String()
	} else {
		return ""
	}

	if raw == "" {
		return ""
	}

	// Strip file:// or file: prefix safely without losing drive letters (e.g. file://C:/...)
	for strings.HasPrefix(raw, "file://") {
		raw = strings.TrimPrefix(raw, "file://")
	}
	if strings.HasPrefix(raw, "file:") {
		raw = strings.TrimPrefix(raw, "file:")
	}

	// Unescape URL encoded characters (e.g. %20 -> space, %C3%B1 -> ñ)
	if unescaped, err := url.PathUnescape(raw); err == nil {
		raw = unescaped
	}

	// On Windows, if path is /C:/dir/file.jpg or /F:/dir/file.jpg, strip leading /
	if len(raw) >= 3 && raw[0] == '/' && raw[2] == ':' {
		raw = raw[1:]
	}

	cleaned := filepath.Clean(raw)

	// Check if file exists on disk
	if _, err := os.Stat(cleaned); err == nil {
		return cleaned
	}

	return raw
}

// generate launches a generation in the background.
func (a *App) generate() {
	model := a.selectedModel()
	if model == "" {
		a.pushToast("Select a model first.", "", true)
		return
	}
	if strings.TrimSpace(a.briefEditor.Text()) == "" {
		a.pushToast("Creative brief is required.", "", true)
		return
	}
	a.mu.Lock()
	a.generating = true
	a.streamTokens = 0
	a.mu.Unlock()
	a.pendingAction = "generate"
	a.pendingIndex = 0
	a.pendingBrief = ""
	a.pendingRefine = ""
	a.saveConfig()

	params := engine.GenerateParams{
		SessionID:            a.session,
		Model:                model,
		CreativeBrief:        a.briefEditor.Text(),
		DurationSeconds:      float64(a.durationSeconds()),
		AspectRatio:          aspectOptions[a.aspectIndex],
		ContextProfile:       contextProfiles[a.contextIndex],
		Thinking:             a.thinking.Value,
		KeepModelLoaded:      a.keepLoaded.Value,
		SystemPromptOverride: a.systemPromptOverride(),
		OnPhase: func(phase string) {
			a.mu.Lock()
			a.phase = phase
			a.mu.Unlock()
			a.window.Invalidate()
		},
		OnProgress: func(tokens int) {
			a.mu.Lock()
			a.streamTokens = tokens
			a.mu.Unlock()
			a.window.Invalidate()
		},
	}
	go func() {
		result, err := a.engine.Generate(params)
		a.mu.Lock()
		if err != nil {
			a.pendingErr = err
		} else {
			a.pendingResult = result
		}
		a.mu.Unlock()
		a.window.Invalidate()
	}()
}

// refine launches a text-only refinement of the selected part in the background.
func (a *App) refine() {
	instruction := strings.TrimSpace(a.refineEditor.Text())
	if instruction == "" || !a.hasResult {
		return
	}
	if a.partIndex < 0 || a.partIndex >= len(a.storyParts) {
		return
	}
	current := a.storyParts[a.partIndex].Prompt
	a.mu.Lock()
	a.generating = true
	a.streamTokens = 0
	a.mu.Unlock()
	a.pendingAction = "refine"
	a.pendingIndex = a.partIndex
	a.pendingBrief = ""
	a.pendingRefine = instruction
	a.originalOut = current
	params := engine.RefineParams{
		SessionID:            a.session,
		Model:                a.selectedModel(),
		CurrentPrompt:        current,
		Instruction:          instruction,
		ContextProfile:       contextProfiles[a.contextIndex],
		Thinking:             a.thinking.Value,
		KeepModelLoaded:      a.keepLoaded.Value,
		SystemPromptOverride: a.systemPromptOverride(),
		OnPhase: func(phase string) {
			a.mu.Lock()
			a.phase = phase
			a.mu.Unlock()
			a.window.Invalidate()
		},
		OnProgress: func(tokens int) {
			a.mu.Lock()
			a.streamTokens = tokens
			a.mu.Unlock()
			a.window.Invalidate()
		},
	}
	go func() {
		result, err := a.engine.Refine(params)
		a.mu.Lock()
		if err != nil {
			a.pendingErr = err
		} else {
			a.pendingResult = result
		}
		a.mu.Unlock()
		a.window.Invalidate()
	}()
}

// systemPromptOverride returns the custom system prompt pointer when the
// editor differs from the default, or nil.
func (a *App) systemPromptOverride() *string {
	text := strings.TrimSpace(a.sysEditor.Text())
	if text == "" {
		return nil
	}
	return &text
}

// sourceVideoLabel returns the <Video N> label used by every extension of the
// current story for the previous part's generated video: the first number
// after the real video assets, so it never collides with them.
func (a *App) sourceVideoLabel() string {
	count := 0
	for _, asset := range a.engine.Store.List(a.session) {
		if asset.Type == media.Video {
			count++
		}
	}
	return fmt.Sprintf("<Video %d>", count+1)
}

// extendStory generates the next part of the story from the previous part's
// ending, using the idea the user wrote in the extend editor.
func (a *App) extendStory() {
	model := a.selectedModel()
	if model == "" {
		a.pushToast("Select a model first.", "", true)
		return
	}
	if len(a.storyParts) == 0 {
		return
	}
	brief := strings.TrimSpace(a.extendEditor.Text())
	if brief == "" {
		a.pushToast("Describe what happens in the next part first.", "", true)
		return
	}
	previous := a.storyParts[len(a.storyParts)-1].Prompt
	ending := prompt.ExtractEndingState(previous)
	if ending == "" {
		a.pushToast("The previous part has no extractable ending state.", "", true)
		return
	}
	a.mu.Lock()
	a.generating = true
	a.streamTokens = 0
	a.mu.Unlock()
	a.pendingAction = "extend"
	a.pendingIndex = len(a.storyParts)
	a.pendingBrief = brief
	a.pendingRefine = ""
	a.saveConfig()

	params := engine.ContinuationParams{
		SessionID:            a.session,
		Model:                model,
		PartBrief:            brief,
		DurationSeconds:      float64(a.durationSeconds()),
		AspectRatio:          aspectOptions[a.aspectIndex],
		PreviousPrompt:       previous,
		PreviousEnding:       ending,
		SourceVideoLabel:     a.sourceVideoLabel(),
		ContextProfile:       contextProfiles[a.contextIndex],
		Thinking:             a.thinking.Value,
		KeepModelLoaded:      a.keepLoaded.Value,
		SystemPromptOverride: a.systemPromptOverride(),
		OnPhase: func(phase string) {
			a.mu.Lock()
			a.phase = phase
			a.mu.Unlock()
			a.window.Invalidate()
		},
		OnProgress: func(tokens int) {
			a.mu.Lock()
			a.streamTokens = tokens
			a.mu.Unlock()
			a.window.Invalidate()
		},
	}
	go func() {
		result, err := a.engine.GenerateContinuation(params)
		a.mu.Lock()
		if err != nil {
			a.pendingErr = err
		} else {
			a.pendingResult = result
		}
		a.mu.Unlock()
		a.window.Invalidate()
	}()
}

// regeneratePart re-generates the selected part. Part 1 is a fresh generation
// from the main brief; later parts are continuations of the previous part's
// ending using the brief the user wrote for that part. Following parts are
// kept intact so the ideas already written in them are never lost.
func (a *App) regeneratePart() {
	model := a.selectedModel()
	if model == "" || !a.hasResult {
		return
	}
	index := a.partIndex
	if index < 0 || index >= len(a.storyParts) {
		return
	}
	a.mu.Lock()
	a.generating = true
	a.streamTokens = 0
	a.mu.Unlock()
	a.pendingAction = "regenerate"
	a.pendingIndex = index
	a.pendingBrief = ""
	a.pendingRefine = ""
	a.saveConfig()

	if index == 0 {
		params := engine.GenerateParams{
			SessionID:            a.session,
			Model:                model,
			CreativeBrief:        a.briefEditor.Text(),
			DurationSeconds:      float64(a.durationSeconds()),
			AspectRatio:          aspectOptions[a.aspectIndex],
			ContextProfile:       contextProfiles[a.contextIndex],
			Thinking:             a.thinking.Value,
			KeepModelLoaded:      a.keepLoaded.Value,
			SystemPromptOverride: a.systemPromptOverride(),
			OnPhase: func(phase string) {
				a.mu.Lock()
				a.phase = phase
				a.mu.Unlock()
				a.window.Invalidate()
			},
			OnProgress: func(tokens int) {
				a.mu.Lock()
				a.streamTokens = tokens
				a.mu.Unlock()
				a.window.Invalidate()
			},
		}
		go func() {
			result, err := a.engine.Generate(params)
			a.mu.Lock()
			if err != nil {
				a.pendingErr = err
			} else {
				a.pendingResult = result
			}
			a.mu.Unlock()
			a.window.Invalidate()
		}()
		return
	}

	previous := a.storyParts[index-1].Prompt
	ending := prompt.ExtractEndingState(previous)
	brief := a.storyParts[index].Brief
	if brief == "" {
		brief = a.extendEditor.Text()
	}
	if ending == "" {
		a.mu.Lock()
		a.generating = false
		a.mu.Unlock()
		a.pushToast("The previous part has no extractable ending state.", "", true)
		return
	}
	params := engine.ContinuationParams{
		SessionID:            a.session,
		Model:                model,
		PartBrief:            brief,
		DurationSeconds:      float64(a.durationSeconds()),
		AspectRatio:          aspectOptions[a.aspectIndex],
		PreviousPrompt:       previous,
		PreviousEnding:       ending,
		SourceVideoLabel:     a.sourceVideoLabel(),
		ContextProfile:       contextProfiles[a.contextIndex],
		Thinking:             a.thinking.Value,
		KeepModelLoaded:      a.keepLoaded.Value,
		SystemPromptOverride: a.systemPromptOverride(),
		OnPhase: func(phase string) {
			a.mu.Lock()
			a.phase = phase
			a.mu.Unlock()
			a.window.Invalidate()
		},
		OnProgress: func(tokens int) {
			a.mu.Lock()
			a.streamTokens = tokens
			a.mu.Unlock()
			a.window.Invalidate()
		},
	}
	go func() {
		result, err := a.engine.GenerateContinuation(params)
		a.mu.Lock()
		if err != nil {
			a.pendingErr = err
		} else {
			a.pendingResult = result
		}
		a.mu.Unlock()
		a.window.Invalidate()
	}()
}

// selectPart loads the selected story part into the output editor.
func (a *App) selectPart(index int) {
	if index < 0 || index >= len(a.storyParts) {
		return
	}
	a.partIndex = index
	a.originalOut = a.storyParts[index].Prompt
	a.outputEditor.SetText(a.storyParts[index].Prompt)
	a.lastAIMark = a.storyParts[index].Prompt
	a.hasResult = true
	if a.window != nil {
		a.window.Invalidate()
	}
}

// presetParts flattens the current story into the preset part list.
func (a *App) presetParts() []config.PresetPart {
	parts := make([]config.PresetPart, 0, len(a.storyParts))
	for _, part := range a.storyParts {
		parts = append(parts, config.PresetPart{Prompt: part.Prompt, Brief: part.Brief, Refines: append([]string(nil), part.Refines...)})
	}
	return parts
}

// unloadModel asks the server to evict the selected model.
func (a *App) unloadModel() {
	model := a.selectedModel()
	if model == "" {
		return
	}
	go func() {
		client, err := a.engine.Client()
		if err != nil {
			return
		}
		if err := client.Unload(context.Background(), model); err != nil {
			a.mu.Lock()
			a.pushToast(errorText(err), "", true)
			a.mu.Unlock()
		} else {
			a.mu.Lock()
			a.pushToast("Model unloaded from the server.", "", false)
			a.mu.Unlock()
		}
		a.window.Invalidate()
	}()
}

var (
	aspectOptions   = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "3:2", "2:3", "21:9"}
	contextOptions  = []string{"Auto (16K default)", "Low 8K", "Standard 16K", "Extended 24K"}
	contextProfiles = []string{"auto", "low", "standard", "extended"}
)

// saveCurrentPreset saves the current brief, duration, aspect ratio, system prompt, and media assets as a preset.
func (a *App) saveCurrentPreset(name string) {
	if name == "" {
		name = "Template " + time.Now().Format("Jan 02 15:04")
	}
	sysPrompt := ""
	if strings.TrimSpace(a.sysEditor.Text()) != "" {
		sysPrompt = a.sysEditor.Text()
	}

	var presetAssets []config.PresetAsset
	for _, asset := range a.engine.Store.List(a.session) {
		typeStr := "image"
		if asset.Type == media.Video {
			typeStr = "video"
		} else if asset.Type == media.Audio {
			typeStr = "audio"
		}
		presetAssets = append(presetAssets, config.PresetAsset{
			Type:     typeStr,
			Path:     asset.OriginalPath,
			Filename: asset.Filename,
		})
	}

	p, err := a.presetStore.AddOrUpdate(name, a.briefEditor.Text(), a.durationSeconds(), a.cfg.AspectRatio, sysPrompt, a.outputEditor.Text(), presetAssets, a.presetParts())
	if err != nil {
		a.mu.Lock()
		a.pushToast("Failed to save preset: "+err.Error(), "", true)
		a.mu.Unlock()
		return
	}
	a.savingPreset = false
	a.presetNameEditor.SetText("")
	presets := a.presetStore.List()
	for i, preset := range presets {
		if preset.ID == p.ID {
			a.presetIndex = i
			break
		}
	}
	a.mu.Lock()
	a.pushToast(fmt.Sprintf("Saved template '%s'", p.Name), "", false)
	a.mu.Unlock()
	a.window.Invalidate()
}

// loadPreset loads a preset into the editor fields and restores saved media assets.
func (a *App) loadPreset(index int) {
	presets := a.presetStore.List()
	if index < 0 || index >= len(presets) {
		return
	}
	p := presets[index]
	a.presetIndex = index
	a.briefEditor.SetText(p.Brief)
	a.cfg.CreativeBrief = p.Brief
	if p.Duration >= 1 && p.Duration <= 20 {
		a.duration.Value = float32(p.Duration-1) / 19.0
		a.cfg.DurationSeconds = float64(p.Duration)
	}
	if p.AspectRatio != "" {
		for i, opt := range aspectOptions {
			if opt == p.AspectRatio {
				a.aspectIndex = i
				a.cfg.AspectRatio = opt
				break
			}
		}
	}
	if p.SystemPrompt != "" {
		a.sysEditor.SetText(p.SystemPrompt)
		a.cfg.SystemPromptOverride = p.SystemPrompt
	} else {
		a.sysEditor.SetText("")
		a.cfg.SystemPromptOverride = ""
	}
	if len(p.Parts) > 0 {
		a.storyParts = nil
		for _, part := range p.Parts {
			a.storyParts = append(a.storyParts, storyPart{Prompt: part.Prompt, Brief: part.Brief, Refines: append([]string(nil), part.Refines...)})
		}
		a.partIndex = 0
		if len(a.storyParts) > 0 {
			a.outputEditor.SetText(a.storyParts[0].Prompt)
			a.lastAIMark = a.storyParts[0].Prompt
			a.originalOut = a.storyParts[0].Prompt
			a.hasResult = true
			a.highlightMode = true
		}
	} else if p.Output != "" {
		a.storyParts = []storyPart{{Prompt: p.Output, Brief: ""}}
		a.partIndex = 0
		a.outputEditor.SetText(p.Output)
		a.lastAIMark = p.Output
		a.originalOut = p.Output
		a.hasResult = true
		a.highlightMode = true
	} else {
		a.storyParts = nil
		a.partIndex = 0
		a.outputEditor.SetText("")
		a.lastAIMark = ""
		a.originalOut = ""
		a.hasResult = false
	}

	// Restore media assets if saved
	if len(p.Assets) > 0 {
		a.engine.Store.Clear(a.session)
		for _, asset := range p.Assets {
			if asset.Path != "" {
				if _, err := os.Stat(asset.Path); err == nil {
					loaded, err2 := a.engine.Store.Add(a.session, asset.Path)
					if err2 == nil && (asset.Role != "" || asset.Label != "") {
						_, _ = a.engine.Store.SetRole(a.session, loaded.ID, asset.Role, asset.Label, "")
					}
				}
			}
		}
		// Resolve LinkedAssetFilename → LinkedAssetID now that all assets are loaded
		for _, pa := range p.Assets {
			if pa.LinkedAssetFilename != "" {
				var voiceID, linkedID string
				for _, a2 := range a.engine.Store.List(a.session) {
					if a2.Filename == pa.Filename {
						voiceID = a2.ID
					}
					if a2.Filename == pa.LinkedAssetFilename {
						linkedID = a2.ID
					}
				}
				if voiceID != "" && linkedID != "" {
					_, _ = a.engine.Store.SetRole(a.session, voiceID, pa.Role, pa.Label, linkedID)
				}
			}
		}
	} else {
		a.engine.Store.Clear(a.session)
	}

	a.saveConfig()
	a.window.Invalidate()
}

// autoSaveCurrentPreset automatically syncs current editor & media state into the active preset (or DEFAULT).
func (a *App) autoSaveCurrentPreset() {
	presets := a.presetStore.List()
	presetName := "DEFAULT"
	if a.presetIndex >= 0 && a.presetIndex < len(presets) {
		presetName = presets[a.presetIndex].Name
	}

	brief := a.briefEditor.Text()
	dur := a.durationSeconds()
	aspect := "16:9"
	if a.aspectIndex >= 0 && a.aspectIndex < len(aspectOptions) {
		aspect = aspectOptions[a.aspectIndex]
	}
	sys := a.sysEditor.Text()
	output := a.outputEditor.Text()

	var presetAssets []config.PresetAsset
	for _, asset := range a.engine.Store.List(a.session) {
		pa := config.PresetAsset{
			Type:     string(asset.Type),
			Path:     asset.OriginalPath,
			Filename: asset.Filename,
			Role:     asset.Role,
			Label:    asset.Label,
		}
		// Resolve linked asset ID → filename for portability
		if asset.LinkedAssetID != "" {
			for _, a2 := range a.engine.Store.List(a.session) {
				if a2.ID == asset.LinkedAssetID {
					pa.LinkedAssetFilename = a2.Filename
					break
				}
			}
		}
		presetAssets = append(presetAssets, pa)
	}

	_, _ = a.presetStore.AddOrUpdate(presetName, brief, dur, aspect, sys, output, presetAssets, a.presetParts())
}

// deleteCurrentPreset deletes the currently selected preset (cannot delete DEFAULT).
func (a *App) deleteCurrentPreset() {
	presets := a.presetStore.List()
	if a.presetIndex < 0 || a.presetIndex >= len(presets) {
		return
	}
	p := presets[a.presetIndex]
	if p.Name == "DEFAULT" {
		return
	}
	_ = a.presetStore.Delete(p.ID)
	// Reset to DEFAULT
	presets = a.presetStore.List()
	for i, item := range presets {
		if item.Name == "DEFAULT" {
			a.loadPreset(i)
			break
		}
	}
	a.mu.Lock()
	a.pushToast(fmt.Sprintf("Deleted template '%s'", p.Name), "", false)
	a.mu.Unlock()
	a.window.Invalidate()
}
