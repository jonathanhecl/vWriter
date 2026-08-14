// Package app wires the Gio UI to the vWriter engine, media store, and
// settings.
package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/gesture"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
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
	created time.Time // set by pushToast; drives auto-dismissal
}

// pushToast queues a notification. Success toasts auto-dismiss after
// toastLifetime; error toasts persist until dismissed or copied.
func (a *App) pushToast(text, details string, isError bool) {
	a.toasts = append(a.toasts, toastMsg{text: text, details: details, isError: isError, created: time.Now()})
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
	busyTag       struct{}               // pointer tag of the full-window busy overlay

	// windowMode tracks the last ConfigEvent mode so frame() only persists
	// the window size while windowed (UI goroutine only).
	windowMode app.WindowMode

	// Widgets (UI goroutine only).
	urlEditor          widget.Editor
	modelDropdown      dropdown
	briefEditor        widget.Editor
	duration           widget.Float
	aspectDropdown     dropdown
	aspectIndex        int
	contextDropdown    dropdown
	contextIndex       int
	thinking           widget.Bool
	keepLoaded         widget.Bool
	sysEditor          widget.Editor
	sysOpen            widget.Bool
	advOpen            widget.Bool
	outputEditor       widget.Editor
	refineEditor       widget.Editor
	refineOpen         bool
	generateBtn        widget.Clickable
	cancelBtn          widget.Clickable
	copyBtn            widget.Clickable
	refineBtn          widget.Clickable
	rewriteBtn         widget.Clickable
	restoreBtn         widget.Clickable
	addFileBtn         widget.Clickable
	addMediaBtn        widget.Clickable
	addMediaCardBtn    widget.Clickable
	clearMediaBtn      widget.Clickable
	filterAllBtn       widget.Clickable
	filterImgBtn       widget.Clickable
	filterVidBtn       widget.Clickable
	filterAudBtn       widget.Clickable
	scrollLeftBtn      widget.Clickable
	scrollRightBtn     widget.Clickable
	scrollbarClickable widget.Clickable
	mediaDrag          gesture.Drag
	dividerDrag        gesture.Drag
	mediaFilter        string // "all", "image", "video", "audio"
	dragStartX         float32
	dragAccum          float32 // accumulated drag pixels since the last item snap
	mediaTotalItems    int     // item count of the media row from the last layout
	// Divider drag state and last window width (UI goroutine only).
	dividerPressX     float32
	dividerStartWidth int
	lastWidthDp       int
	connectBtn        widget.Clickable
	refreshBtn        widget.Clickable
	unloadBtn         widget.Clickable
	mediaList         layout.List
	filterList        layout.List
	outputList        widget.List
	leftList          widget.List
	assetWidgetSet    map[string]*assetWidgets
	modalStateSet     *modalState
	modalFrameIndex   int
	toastClicks       []toastClick

	presetStore      *config.PresetStore
	presetDropdown   dropdown
	presetIndex      int
	savePresetBtn    widget.Clickable
	deletePresetBtn  widget.Clickable
	newPresetBtn     widget.Clickable
	savingPreset     bool
	newPreset        bool
	presetNameEditor widget.Editor
	confirmSaveBtn   widget.Clickable
	cancelSaveBtn    widget.Clickable

	toasts      []toastMsg
	modal       *media.Asset     // non-nil while a preview modal is open
	assetModal  *assetModalState // non-nil while the role/label modal is open
	originalOut string           // pre-refine output for restore
	lastAIMark  string           // last AI-produced output for the Modified badge

	highlightMode  bool           // true = render highlighted view instead of plain editor
	highlightState highlightState // richtext state for highlighted output
	highlightBtn   widget.Clickable

	// Multi-part story state (UI goroutine only).
	storyParts []storyPart
	partIndex  int
	// pendingAction records which flow produced a pending result so applyAsync
	// knows where to write it: "generate", "extend", "regenerate", or "refine".
	pendingAction string
	pendingIndex  int
	pendingBrief  string
	pendingRefine string

	extendEditor  widget.Editor
	extendOpen    bool
	extendBtn     widget.Clickable
	genExtendBtn  widget.Clickable
	regenerateBtn widget.Clickable
	copyBriefBtn  widget.Clickable
	partChips     []widget.Clickable
}

// storyPart is one prompt of a multi-part story. Brief is the idea the user
// wrote for that part (for part 1, the main creative brief), and Refine is the
// accumulated revision instruction: refining an already-refined part starts
// from the existing text and replaces it.
type storyPart struct {
	Prompt string
	Brief  string
	Refine string
}

// Run starts the window event loop.
func Run(window *app.Window) error {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		cfg = config.Default()
	}
	window.Option(
		app.Size(unit.Dp(cfg.WindowWidth), unit.Dp(cfg.WindowHeight)),
		app.MinSize(unit.Dp(1000), unit.Dp(800)),
	)
	// Restore the maximized state in a separate Option call: Gio batches all
	// options into one config, and on Windows the maximized mode ignores the
	// size (SWP_NOMOVE|SWP_NOSIZE), so the restored size must be applied first.
	if cfg.WindowMaximized {
		window.Option(app.Maximized.Option())
	}
	presetStore, _ := config.NewPresetStore(config.DefaultPresetsPath())
	a := &App{
		window:        window,
		theme:         newTheme(),
		explorer:      explorer.NewExplorer(window),
		engine:        engine.NewEngine(media.NewStore("")),
		cfg:           cfg,
		cfgPath:       config.DefaultPath(),
		images:        map[string]image.Image{},
		presetStore:   presetStore,
		presetIndex:   -1,
		highlightMode: true,
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

	// Always select and load DEFAULT preset on startup
	presets := a.presetStore.List()
	defaultIndex := 0
	for i, p := range presets {
		if p.Name == "DEFAULT" {
			defaultIndex = i
			break
		}
	}
	a.loadPreset(defaultIndex)
	a.thinking.Value = cfg.Thinking
	a.keepLoaded.Value = cfg.KeepModelLoaded
	a.sysEditor.SetText(cfg.SystemPromptOverride)

	if err := a.engine.SetOllamaURL(cfg.OllamaURL); err == nil {
		a.refreshModels()
	}

	var ops op.Ops
	for {
		ev := window.Event()
		if !cfg.WindowMaximized {
			centerWindowOnFirstView(window, ev)
		}
		a.explorer.ListenEvents(ev)
		switch e := ev.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.ConfigEvent:
			a.windowMode = e.Config.Mode
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.frame(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

// frame applies pending async results, handles widget events, and lays out.
func (a *App) frame(gtx layout.Context) {
	a.lastWidthDp = int(gtx.Metric.PxToDp(gtx.Constraints.Max.X))
	// Persist the windowed (restored) size only while windowed; a maximized
	// window reports its full-screen size, which must not overwrite it.
	switch a.windowMode {
	case app.Maximized:
		if !a.cfg.WindowMaximized {
			a.cfg.WindowMaximized = true
			a.saveConfig()
		}
	case app.Windowed:
		wDp := int(gtx.Metric.PxToDp(gtx.Constraints.Max.X))
		hDp := int(gtx.Metric.PxToDp(gtx.Constraints.Max.Y))
		if wDp >= 1000 && hDp >= 800 &&
			(a.cfg.WindowWidth != wDp || a.cfg.WindowHeight != hDp || a.cfg.WindowMaximized) {
			a.cfg.WindowWidth = wDp
			a.cfg.WindowHeight = hDp
			a.cfg.WindowMaximized = false
			a.saveConfig()
		}
	}

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
			a.pushToast(errorText(a.pendingErr), errorDetails(a.pendingErr), true)
		} else {
			a.applyResult(a.pendingResult)
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

// applyResult writes a finished generation into the editor and the multi-part
// story according to the flow that produced it. Callers hold a.mu.
func (a *App) applyResult(res *engine.Result) {
	switch a.pendingAction {
	case "extend":
		a.storyParts = append(a.storyParts, storyPart{Prompt: res.Prompt, Brief: a.pendingBrief})
		a.partIndex = len(a.storyParts) - 1
	case "regenerate":
		if a.pendingIndex >= 0 && a.pendingIndex < len(a.storyParts) {
			a.storyParts[a.pendingIndex].Prompt = res.Prompt
			if a.pendingIndex == 0 {
				a.storyParts[0].Brief = a.briefEditor.Text()
			}
			a.partIndex = a.pendingIndex
		}
	case "refine":
		if a.pendingIndex >= 0 && a.pendingIndex < len(a.storyParts) {
			a.storyParts[a.pendingIndex].Prompt = res.Prompt
			if refine := strings.TrimSpace(a.pendingRefine); refine != "" {
				a.storyParts[a.pendingIndex].Refine = refine
			}
			a.partIndex = a.pendingIndex
		}
	default: // "generate"
		a.storyParts = []storyPart{{Prompt: res.Prompt, Brief: a.briefEditor.Text()}}
		a.partIndex = 0
	}
	a.originalOut = res.Prompt
	a.outputEditor.SetText(res.Prompt)
	a.lastAIMark = res.Prompt
	a.hasResult = true
	a.highlightMode = true
	if res.RepairApplied {
		a.pushToast("One automatic format repair was applied.", "", false)
	}
	if res.RepairAttempted && !res.RepairApplied {
		a.pushToast("Automatic format repair could not fix all issues. Review the prompt before using it.", "", true)
	}
	a.autoSaveCurrentPreset()
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

// copyOutput schedules a clipboard write of the selected story part (or the
// current editor text when no story is loaded).
func (a *App) copyOutput(gtx layout.Context) {
	text := a.outputEditor.Text()
	label := "Prompt"
	if a.partIndex >= 0 && a.partIndex < len(a.storyParts) {
		text = a.storyParts[a.partIndex].Prompt
		label = fmt.Sprintf("Part %d", a.partIndex+1)
	}
	if text == "" {
		return
	}
	gtx.Execute(clipboard.WriteCmd{Data: nopCloser{strings.NewReader(text)}})
	a.pushToast(label+" copied to the clipboard.", "", false)
	if a.window != nil {
		a.window.Invalidate()
	}
}

func newSessionID() string {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	return hex.EncodeToString(raw[:])
}
