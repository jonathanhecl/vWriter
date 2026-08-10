// Package app wires the Gio UI to the vWriter engine, media store, and
// settings.
package app

import (
	"crypto/rand"
	"encoding/hex"
	"image"
	"strings"
	"sync"

	"gioui.org/app"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"

	"github.com/jonathanhecl/vWriter/internal/config"
	"github.com/jonathanhecl/vWriter/internal/engine"
	"github.com/jonathanhecl/vWriter/internal/media"
)

// modelEntry is one installed Ollama model offered in the picker.
type modelEntry struct {
	Name string
	Size string
}

// toastMsg is one transient notification.
type toastMsg struct {
	text    string
	details string
	isError bool
}

// App is the root UI state.
type App struct {
	window   *app.Window
	theme    *material.Theme
	explorer *explorer.Explorer
	engine   *engine.Engine
	session  string
	cfg      *config.Config
	cfgPath  string

	// Async state, guarded by mu.
	mu            sync.Mutex
	models        []modelEntry
	modelsLoading bool
	modelsError   string
	generating    bool
	phase         string
	streamTokens  int
	pendingResult *engine.Result
	pendingErr    error
	hasResult     bool
	dirtyOutput   bool
	images        map[string]image.Image // asset preview path -> decoded

	// Widgets (UI goroutine only).
	urlEditor       widget.Editor
	modelDropdown   dropdown
	briefEditor     widget.Editor
	duration        widget.Float
	aspectDropdown  dropdown
	aspectIndex     int
	contextDropdown dropdown
	contextIndex    int
	thinking        widget.Bool
	keepLoaded      widget.Bool
	sysEditor       widget.Editor
	sysOpen         widget.Bool
	advOpen         widget.Bool
	outputEditor    widget.Editor
	refineEditor    widget.Editor
	refineOpen      bool
	generateBtn     widget.Clickable
	cancelBtn       widget.Clickable
	copyBtn         widget.Clickable
	refineBtn       widget.Clickable
	rewriteBtn      widget.Clickable
	restoreBtn      widget.Clickable
	addFileBtn      widget.Clickable
	addMediaBtn     widget.Clickable
	addMediaCardBtn widget.Clickable
	clearMediaBtn   widget.Clickable
	filterAllBtn    widget.Clickable
	filterImgBtn    widget.Clickable
	filterVidBtn    widget.Clickable
	filterAudBtn    widget.Clickable
	mediaFilter     string // "all", "image", "video", "audio"
	connectBtn      widget.Clickable
	refreshBtn      widget.Clickable
	unloadBtn       widget.Clickable
	mediaList       layout.List
	outputList      layout.List
	leftList        layout.List
	assetWidgetSet  map[string]*assetWidgets
	modalStateSet   *modalState
	modalFrameIndex int
	toastClicks     []toastClick

	toasts      []toastMsg
	modal       *media.Asset // non-nil while a preview modal is open
	originalOut string       // pre-refine output for restore
	lastAIMark  string       // last AI-produced output for the Modified badge
}

// Run starts the window event loop.
func Run(window *app.Window) error {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		cfg = config.Default()
	}
	a := &App{
		window:   window,
		theme:    newTheme(),
		explorer: explorer.NewExplorer(window),
		engine:   engine.NewEngine(media.NewStore("")),
		cfg:      cfg,
		cfgPath:  config.DefaultPath(),
		images:   map[string]image.Image{},
	}
	a.session = newSessionID()
	a.mediaList.Axis = layout.Vertical
	a.leftList.Axis = layout.Vertical
	a.outputList.Axis = layout.Vertical

	a.urlEditor.SingleLine = true
	a.urlEditor.SetText(cfg.OllamaURL)
	a.briefEditor.SingleLine = false
	a.briefEditor.SetText(cfg.CreativeBrief)
	a.outputEditor.ReadOnly = false
	a.duration.Value = float32(cfg.DurationSeconds-1) / 19 // slider 0..1 maps to 1..20 s
	for index, option := range aspectOptions {
		if option == cfg.AspectRatio {
			a.aspectIndex = index
		}
	}
	for index, profile := range contextProfiles {
		if profile == cfg.ContextProfile {
			a.contextIndex = index
		}
	}
	a.thinking.Value = cfg.Thinking
	a.keepLoaded.Value = cfg.KeepModelLoaded
	a.sysEditor.SetText(cfg.SystemPromptOverride)

	if err := a.engine.SetOllamaURL(cfg.OllamaURL); err == nil {
		a.refreshModels()
	}

	var ops op.Ops
	for {
		ev := window.Event()
		centerWindowOnFirstView(window, ev)
		a.explorer.ListenEvents(ev)
		switch e := ev.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.frame(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

// frame applies pending async results, handles widget events, and lays out.
func (a *App) frame(gtx layout.Context) {
	a.applyAsync()
	a.handleEvents(gtx)
	a.layout(gtx)
}

// applyAsync moves goroutine results into UI state and widgets.
func (a *App) applyAsync() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pendingResult != nil || a.pendingErr != nil {
		if a.pendingErr != nil {
			a.toasts = append(a.toasts, toastMsg{text: errorText(a.pendingErr), details: errorDetails(a.pendingErr), isError: true})
		} else {
			res := a.pendingResult
			a.originalOut = res.Prompt
			a.outputEditor.SetText(res.Prompt)
			a.lastAIMark = res.Prompt
			a.hasResult = true
			if res.RepairApplied {
				a.toasts = append(a.toasts, toastMsg{text: "One automatic format repair was applied."})
			}
		}
		a.pendingResult = nil
		a.pendingErr = nil
		a.generating = false
		a.phase = engine.PhaseIdle
	}
	if a.dirtyOutput {
		a.dirtyOutput = false
	}
}

// saveConfig persists the current settings.
func (a *App) saveConfig() {
	a.cfg.OllamaURL = a.urlEditor.Text()
	a.cfg.DurationSeconds = float64(a.durationSeconds())
	a.cfg.AspectRatio = aspectOptions[a.aspectIndex]
	a.cfg.ContextProfile = contextProfiles[a.contextIndex]
	a.cfg.Thinking = a.thinking.Value
	a.cfg.KeepModelLoaded = a.keepLoaded.Value
	a.cfg.CreativeBrief = a.briefEditor.Text()
	a.cfg.SystemPromptOverride = a.sysEditor.Text()
	_ = a.cfg.Save(a.cfgPath)
}

// copyOutput schedules a clipboard write of the current output.
func (a *App) copyOutput(gtx layout.Context) {
	text := a.outputEditor.Text()
	if text == "" {
		return
	}
	gtx.Execute(clipboard.WriteCmd{Data: nopCloser{strings.NewReader(text)}})
	a.toasts = append(a.toasts, toastMsg{text: "Prompt copied to the clipboard."})
	a.window.Invalidate()
}

func newSessionID() string {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	return hex.EncodeToString(raw[:])
}
