package app

import (
	"strings"
	"testing"
)

func TestStoryBriefsText(t *testing.T) {
	a := &App{storyParts: []storyPart{
		{Prompt: "p1", Brief: "main brief", Refines: []string{"make it colder"}},
		{Prompt: "p2", Brief: "part 2 idea", Refines: []string{"add more tension", "slow the pacing"}},
		{Prompt: "p3", Brief: "part 3 idea"},
	}}
	got := a.storyBriefsText()
	for _, phrase := range []string{
		"Part 1",
		"main brief",
		"make it colder",
		"Part 2",
		"part 2 idea",
		"add more tension",
		"slow the pacing",
		"Part 3",
		"part 3 idea",
	} {
		if !strings.Contains(got, phrase) {
			t.Errorf("storyBriefsText missing %q", phrase)
		}
	}
	if !strings.HasPrefix(got, "Part 1") {
		t.Errorf("storyBriefsText must start with part 1, got %q", got)
	}
}

func TestStoryBriefsTextEmpty(t *testing.T) {
	if got := (&App{}).storyBriefsText(); strings.TrimSpace(got) != "" {
		t.Fatalf("empty story must yield empty text, got %q", got)
	}
}
