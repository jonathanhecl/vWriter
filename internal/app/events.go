package app

import (
	"gioui.org/layout"
)

// handleEvents processes widget events for the current frame.
func (a *App) handleEvents(gtx layout.Context) {
	if a.connectBtn.Clicked(gtx) {
		a.connect()
	}
	if a.refreshBtn.Clicked(gtx) {
		a.refreshModels()
	}
	if a.addMediaBtn.Clicked(gtx) {
		a.addMedia()
	}
	if a.clearMediaBtn.Clicked(gtx) {
		a.engine.Store.Clear(a.session)
		a.hasResult = false
	}
	if a.generateBtn.Clicked(gtx) {
		a.mu.Lock()
		busy := a.generating
		a.mu.Unlock()
		if !busy {
			a.generate()
		}
	}
	if a.cancelBtn.Clicked(gtx) {
		a.engine.Cancel()
	}
	if a.copyBtn.Clicked(gtx) {
		a.copyOutput(gtx)
	}
	if a.refineBtn.Clicked(gtx) && a.hasResult {
		a.refineOpen = !a.refineOpen
	}
	if a.rewriteBtn.Clicked(gtx) {
		a.refine()
	}
	if a.restoreBtn.Clicked(gtx) && a.originalOut != "" {
		a.outputEditor.SetText(a.originalOut)
	}
	if a.unloadBtn.Clicked(gtx) {
		a.unloadModel()
	}
	if a.thinking.Update(gtx) || a.keepLoaded.Update(gtx) {
		a.saveConfig()
	}
	if a.duration.Update(gtx) {
		a.saveConfig()
	}
}
