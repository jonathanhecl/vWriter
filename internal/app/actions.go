package app

import (
	"context"
	"io"
	"net/url"
	"strings"

	"github.com/jonathanhecl/vWriter/internal/engine"
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
		a.toasts = append(a.toasts, toastMsg{text: errorText(err), isError: true})
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

// addMedia opens the native file picker and registers the chosen files.
func (a *App) addMedia() {
	go func() {
		files, err := a.explorer.ChooseFiles(
			".jpg", ".jpeg", ".png", ".webp", ".bmp", ".tif", ".tiff",
			".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4v",
			".wav", ".mp3", ".flac", ".m4a", ".ogg", ".aac", ".opus",
		)
		if err != nil {
			return // user cancelled the dialog
		}
		for _, file := range files {
			path := filePath(file)
			file.Close()
			if path == "" {
				continue
			}
			if _, err := a.engine.Store.Add(a.session, path); err != nil {
				a.mu.Lock()
				a.toasts = append(a.toasts, toastMsg{text: errorText(err), details: errorDetails(err), isError: true})
				a.mu.Unlock()
			}
		}
		a.window.Invalidate()
	}()
}

// filePath extracts a local filesystem path from a picked file.
func filePath(file io.ReadCloser) string {
	named, ok := file.(interface{ URI() string })
	if !ok {
		return ""
	}
	parsed, err := url.Parse(named.URI())
	if err != nil || parsed.Scheme != "file" {
		return ""
	}
	path := parsed.Path
	// Windows file URIs look like /F:/dir/file; strip the leading slash.
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return path
}

// generate launches a generation in the background.
func (a *App) generate() {
	model := a.selectedModel()
	if model == "" {
		a.toasts = append(a.toasts, toastMsg{text: "Select a model first.", isError: true})
		return
	}
	if strings.TrimSpace(a.briefEditor.Text()) == "" {
		a.toasts = append(a.toasts, toastMsg{text: "Creative brief is required.", isError: true})
		return
	}
	a.mu.Lock()
	a.generating = true
	a.streamTokens = 0
	a.mu.Unlock()
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

// refine launches a text-only refinement in the background.
func (a *App) refine() {
	instruction := strings.TrimSpace(a.refineEditor.Text())
	if instruction == "" || !a.hasResult {
		return
	}
	a.mu.Lock()
	a.generating = true
	a.streamTokens = 0
	a.mu.Unlock()
	a.originalOut = a.outputEditor.Text()
	params := engine.RefineParams{
		SessionID:            a.session,
		Model:                a.selectedModel(),
		CurrentPrompt:        a.outputEditor.Text(),
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
			a.toasts = append(a.toasts, toastMsg{text: errorText(err), isError: true})
			a.mu.Unlock()
		} else {
			a.mu.Lock()
			a.toasts = append(a.toasts, toastMsg{text: "Model unloaded from the server."})
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
