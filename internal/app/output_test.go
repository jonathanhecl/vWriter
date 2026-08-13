package app

import (
	"strings"
	"testing"
)

func TestStoryPartBriefText(t *testing.T) {
	a := &App{storyParts: []storyPart{
		{Prompt: "p1", Brief: "main brief", Refine: "make it colder"},
		{Prompt: "p2", Brief: "part 2 idea", Refine: "add tension, slow the pacing"},
		{Prompt: "p3", Brief: "part 3 idea"},
	}}

	got := a.storyPartBriefText(1)
	for _, phrase := range []string{
		"Part 2",
		"part 2 idea",
		"add tension, slow the pacing",
	} {
		if !strings.Contains(got, phrase) {
			t.Errorf("storyPartBriefText missing %q", phrase)
		}
	}
	// Only the selected part's brief must appear, not the others.
	if strings.Contains(got, "main brief") || strings.Contains(got, "part 3 idea") {
		t.Errorf("storyPartBriefText leaked other parts: %q", got)
	}
}

func TestStoryPartBriefTextBounds(t *testing.T) {
	a := &App{storyParts: []storyPart{{Prompt: "p1", Brief: "b1"}}}
	if got := a.storyPartBriefText(-1); got != "" {
		t.Fatalf("negative index must be empty, got %q", got)
	}
	if got := a.storyPartBriefText(5); got != "" {
		t.Fatalf("out-of-range index must be empty, got %q", got)
	}
	if got := a.storyPartBriefText(0); !strings.Contains(got, "b1") {
		t.Fatalf("valid index must return the brief, got %q", got)
	}
}
