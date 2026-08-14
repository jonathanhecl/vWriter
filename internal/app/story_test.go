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

	if brief := a.storyPartBriefText(1); !strings.Contains(brief, "part two idea") {
		t.Fatalf("storyPartBriefText = %q", brief)
	}
}

func TestApplyResultRefineReplacesInstruction(t *testing.T) {
	a := testApp(t)
	a.storyParts = []storyPart{{Prompt: "P1"}, {Prompt: "P2"}}
	a.pendingAction = "refine"
	a.pendingIndex = 1
	a.pendingRefine = "make the lighting colder"
	a.applyResult(&engine.Result{Prompt: "P2-REFINED"})
	if a.storyParts[1].Prompt != "P2-REFINED" {
		t.Fatalf("refined prompt not applied: %+v", a.storyParts[1])
	}
	if a.storyParts[1].Refine != "make the lighting colder" {
		t.Fatalf("refine not recorded: %+v", a.storyParts[1])
	}
	// Refining an already-refined part replaces the previous instruction.
	a.pendingRefine = "make it darker and add rain"
	a.applyResult(&engine.Result{Prompt: "P2-REFINED-2"})
	if a.storyParts[1].Refine != "make it darker and add rain" {
		t.Fatalf("refine-of-refine must replace, got %+v", a.storyParts[1])
	}
	// Only the final refine appears in the selected part's copy-brief text.
	brief := a.storyPartBriefText(1)
	if !strings.Contains(brief, "make it darker and add rain") {
		t.Fatalf("refine missing from brief text: %q", brief)
	}
	if strings.Contains(brief, "make the lighting colder") {
		t.Fatalf("superseded refine must not appear: %q", brief)
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
	a.storyParts = []storyPart{{Prompt: "P1", Refine: "make it colder"}, {Prompt: "P2"}}
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
	if a.refineEditor.Text() != "make it colder" {
		t.Fatalf("refine editor must pre-fill with the part refine, got %q", a.refineEditor.Text())
	}
}

func TestScrollMediaByReturnsToFirstItem(t *testing.T) {
	a := testApp(t)
	a.mediaTotalItems = 4

	a.scrollMediaBy(1)
	a.scrollMediaBy(1)
	if a.mediaList.Position.First != 2 || a.mediaList.Position.Offset != 0 {
		t.Fatalf("after scrolling right: First=%d Offset=%d", a.mediaList.Position.First, a.mediaList.Position.Offset)
	}

	a.scrollMediaBy(-1)
	a.scrollMediaBy(-1)
	if a.mediaList.Position.First != 0 || a.mediaList.Position.Offset != 0 {
		t.Fatalf("must reach the first item: First=%d Offset=%d", a.mediaList.Position.First, a.mediaList.Position.Offset)
	}

	// A residual wheel/drag offset at the start must be snapped so the first
	// card is never left clipped and unreachable.
	a.mediaList.Position.Offset = 30
	a.scrollMediaBy(-1)
	if a.mediaList.Position.First != 0 || a.mediaList.Position.Offset != 0 {
		t.Fatalf("residual offset must be snapped at the start: First=%d Offset=%d", a.mediaList.Position.First, a.mediaList.Position.Offset)
	}

	// The row must never be scrolled past the last item.
	a.mediaList.Position.First = 3
	a.scrollMediaBy(1)
	if a.mediaList.Position.First != 3 {
		t.Fatalf("must clamp at the last item: First=%d", a.mediaList.Position.First)
	}
}
