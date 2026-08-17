package app

import (
	"os"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"
)

func dbgLog(msg string) {
	f, err := os.OpenFile(`C:\Users\gense\AppData\Local\Temp\vwriter_test\click.log`, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(time.Now().Format("15:04:05.000") + " " + msg + "\n")
}

// handleEvents processes widget events for the current frame.
func (a *App) handleEvents(gtx layout.Context) {
	if a.connectBtn.Clicked(gtx) {
		a.connect()
	}
	if a.refreshBtn.Clicked(gtx) {
		a.refreshModels()
	}
	if a.addFileBtn.Clicked(gtx) {
		a.addFile()
	}
	if a.addMediaCardBtn.Clicked(gtx) {
		dbgLog("addMediaCardBtn Clicked fired")
		a.addMedia()
	}
	if a.addMediaBtn.Clicked(gtx) {
		a.addMedia()
	}
	if a.filterAllBtn.Clicked(gtx) {
		a.mediaFilter = "all"
	}
	if a.filterImgBtn.Clicked(gtx) {
		a.mediaFilter = "image"
	}
	if a.filterVidBtn.Clicked(gtx) {
		a.mediaFilter = "video"
	}
	if a.filterAudBtn.Clicked(gtx) {
		a.mediaFilter = "audio"
	}
	if a.scrollLeftBtn.Clicked(gtx) {
		a.scrollMediaBy(-1)
	}
	if a.scrollRightBtn.Clicked(gtx) {
		a.scrollMediaBy(1)
	}
	if a.clearMediaBtn.Clicked(gtx) {
		a.engine.Store.Clear(a.session)
		a.storyParts = nil
		a.partIndex = 0
		a.hasResult = false
		a.autoSaveCurrentPreset()
	}
	if a.savePresetBtn.Clicked(gtx) {
		a.newPreset = false
		a.savingPreset = true
		a.window.Invalidate()
	}
	if a.newPresetBtn.Clicked(gtx) {
		a.newPreset = true
		a.savingPreset = true
		a.presetNameEditor.SetText("")
		a.window.Invalidate()
	}
	if a.deletePresetBtn.Clicked(gtx) {
		a.deleteCurrentPreset()
	}
	if a.cancelSaveBtn.Clicked(gtx) {
		a.savingPreset = false
		a.newPreset = false
		a.window.Invalidate()
	}
	if a.confirmSaveBtn.Clicked(gtx) {
		if a.newPreset {
			a.createNewPreset(a.presetNameEditor.Text())
		} else {
			a.saveCurrentPreset(a.presetNameEditor.Text())
		}
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
		if a.refineOpen && a.partIndex >= 0 && a.partIndex < len(a.storyParts) {
			a.refineEditor.SetText(a.storyParts[a.partIndex].Refine)
		}
	}
	if a.rewriteBtn.Clicked(gtx) {
		a.refine()
	}
	if a.restoreBtn.Clicked(gtx) && a.originalOut != "" {
		a.outputEditor.SetText(a.originalOut)
	}
	if a.regenerateBtn.Clicked(gtx) && a.hasResult {
		a.regeneratePart()
	}
	if a.extendBtn.Clicked(gtx) && a.hasResult {
		a.extendOpen = !a.extendOpen
		a.window.Invalidate()
	}
	if a.genExtendBtn.Clicked(gtx) && a.hasResult && len(a.storyParts) > 0 {
		a.extendStory()
	}
	if a.copyBriefBtn.Clicked(gtx) && len(a.storyParts) > 0 {
		a.copyBrief(gtx)
	}
	for len(a.partChips) < len(a.storyParts) {
		a.partChips = append(a.partChips, widget.Clickable{})
	}
	for index := range a.partChips {
		if index >= len(a.storyParts) {
			break
		}
		if a.partChips[index].Clicked(gtx) {
			a.selectPart(index)
		}
	}
	if a.unloadBtn.Clicked(gtx) {
		a.unloadModel()
	}
	if a.advOpen.Update(gtx) || a.sysOpen.Update(gtx) {
		a.window.Invalidate()
	}
	if a.thinking.Update(gtx) || a.keepLoaded.Update(gtx) {
		a.saveConfig()
	}
	if a.duration.Update(gtx) {
		a.saveConfig()
		a.autoSaveCurrentPreset()
	}
	for _, editor := range []*widget.Editor{&a.urlEditor, &a.briefEditor, &a.sysEditor, &a.outputEditor} {
		for {
			event, ok := editor.Update(gtx)
			if !ok {
				break
			}
			if _, changed := event.(widget.ChangeEvent); changed {
				a.saveConfig()
				a.autoSaveCurrentPreset()
			}
		}
	}
}
