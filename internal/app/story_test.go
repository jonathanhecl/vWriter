package app

import (
	"path/filepath"
	"strings"
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

	if briefs := a.storyBriefsText(); !strings.Contains(briefs, "part two idea") {
		t.Fatalf("storyBriefsText = %q", briefs)
	}
}

func TestApplyResultRefineAppendsInstruction(t *testing.T) {
	a := testApp(t)
	a.storyParts = []storyPart{{Prompt: "P1"}, {Prompt: "P2"}}
	a.pendingAction = "refine"
	a.pendingIndex = 1
	a.pendingRefine = "make the lighting colder"
	a.applyResult(&engine.Result{Prompt: "P2-REFINED"})
	if a.storyParts[1].Prompt != "P2-REFINED" {
		t.Fatalf("refined prompt not applied: %+v", a.storyParts[1])
	}
	if len(a.storyParts[1].Refines) != 1 || a.storyParts[1].Refines[0] != "make the lighting colder" {
		t.Fatalf("refine instruction not recorded: %+v", a.storyParts[1])
	}
	// A second refine appends.
	a.pendingRefine = "speed up the pacing"
	a.applyResult(&engine.Result{Prompt: "P2-REFINED-2"})
	if len(a.storyParts[1].Refines) != 2 || a.storyParts[1].Refines[1] != "speed up the pacing" {
		t.Fatalf("second refine not appended: %+v", a.storyParts[1])
	}
	// Refines flow into the copy-brief text.
	briefs := a.storyBriefsText()
	if !strings.Contains(briefs, "make the lighting colder") || !strings.Contains(briefs, "speed up the pacing") {
		t.Fatalf("refines missing from briefs text: %q", briefs)
	}
}

func TestApplyResultRegenerateKeepsFollowingParts(t *testing.T) {
	a := testApp(t)
	a.storyParts = []storyPart{
		{Prompt: "P1", Brief: ""},
		{Prompt: "P2", Brief: "part 2 idea"},
		{Prompt: "P3", Brief: "part 3 idea"},
	}
	a.pendingAction = "regenerate"
	a.pendingIndex = 0
	a.applyResult(&engine.Result{Prompt: "P1-NEW"})
	if len(a.storyParts) != 3 {
		t.Fatalf("regenerate must keep all parts, got %+v", a.storyParts)
	}
	if a.storyParts[0].Prompt != "P1-NEW" {
		t.Fatalf("part 1 prompt not replaced: %+v", a.storyParts[0])
	}
	if a.storyParts[1].Prompt != "P2" || a.storyParts[1].Brief != "part 2 idea" {
		t.Fatalf("part 2 must be preserved with its brief: %+v", a.storyParts[1])
	}
	if a.storyParts[2].Prompt != "P3" || a.storyParts[2].Brief != "part 3 idea" {
		t.Fatalf("part 3 must be preserved with its brief: %+v", a.storyParts[2])
	}

	// Regenerating a middle part replaces only that part.
	a.pendingIndex = 1
	a.applyResult(&engine.Result{Prompt: "P2-NEW"})
	if len(a.storyParts) != 3 || a.storyParts[1].Prompt != "P2-NEW" {
		t.Fatalf("middle regenerate wrong: %+v", a.storyParts)
	}
	if a.storyParts[0].Prompt != "P1-NEW" || a.storyParts[2].Prompt != "P3" {
		t.Fatalf("other parts must remain: %+v", a.storyParts)
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
