package app

import (
	"path/filepath"
	"testing"

	"github.com/jonathanhecl/vWriter/internal/config"
	"github.com/jonathanhecl/vWriter/internal/engine"
	"github.com/jonathanhecl/vWriter/internal/media"
)

func testApp(t *testing.T) *App {
	t.Helper()
	store, _ := config.NewPresetStore(filepath.Join(t.TempDir(), "presets.json"))
	return &App{
		engine:      engine.NewEngine(media.NewStore(t.TempDir())),
		presetStore: store,
		presetIndex: 0,
	}
}

func TestApplyResultExtendAppendsParts(t *testing.T) {
	a := testApp(t)

	a.pendingAction = "generate"
	a.applyResult(&engine.Result{Prompt: "PART ONE"})
	if len(a.storyParts) != 1 || a.storyParts[0].Prompt != "PART ONE" {
		t.Fatalf("after generate: %+v", a.storyParts)
	}

	a.pendingAction = "extend"
	a.pendingBrief = "part two idea"
	a.applyResult(&engine.Result{Prompt: "PART TWO"})
	if len(a.storyParts) != 2 {
		t.Fatalf("after extend: %+v", a.storyParts)
	}
	if a.storyParts[0].Prompt != "PART ONE" || a.storyParts[1].Prompt != "PART TWO" {
		t.Fatalf("parts must both be kept: %+v", a.storyParts)
	}
	if a.storyParts[1].Brief != "part two idea" {
		t.Fatalf("brief not kept: %+v", a.storyParts[1])
	}

	text := a.storyText()
	if !containsAll(text, "PART ONE", "--- PART 2 ---", "PART TWO") {
		t.Fatalf("storyText = %q", text)
	}
}

func TestApplyResultRegenerateDropsFollowingParts(t *testing.T) {
	a := testApp(t)
	a.storyParts = []storyPart{{Prompt: "P1"}, {Prompt: "P2"}, {Prompt: "P3"}}
	a.pendingAction = "regenerate"
	a.pendingIndex = 0
	a.applyResult(&engine.Result{Prompt: "P1-NEW"})
	if len(a.storyParts) != 1 || a.storyParts[0].Prompt != "P1-NEW" {
		t.Fatalf("regenerate part 1 must drop the rest: %+v", a.storyParts)
	}
}

func TestSelectPartSyncsEditor(t *testing.T) {
	a := testApp(t)
	a.storyParts = []storyPart{{Prompt: "P1"}, {Prompt: "P2"}}
	a.partIndex = 1
	a.selectPart(0)
	if a.partIndex != 0 {
		t.Fatalf("partIndex = %d, want 0", a.partIndex)
	}
	if a.outputEditor.Text() != "P1" {
		t.Fatalf("editor text = %q, want P1", a.outputEditor.Text())
	}
	if a.originalOut != "P1" || a.lastAIMark != "P1" {
		t.Fatalf("editor marks not synced: original=%q lastAI=%q", a.originalOut, a.lastAIMark)
	}
}

func containsAll(text string, phrases ...string) bool {
	for _, phrase := range phrases {
		if !containsStr(text, phrase) {
			return false
		}
	}
	return true
}

func containsStr(text, sub string) bool {
	return len(text) >= len(sub) && (sub == "" || indexOf(text, sub) >= 0)
}

func indexOf(text, sub string) int {
	for i := 0; i+len(sub) <= len(text); i++ {
		if text[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
