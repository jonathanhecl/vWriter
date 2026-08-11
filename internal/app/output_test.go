package app

import (
	"strings"
	"testing"
)

func TestStoryText(t *testing.T) {
	a := &App{storyParts: []storyPart{
		{Prompt: "subject_definitions:\n<Subject 1>", Brief: ""},
		{Prompt: "summary:\n[video continuation]", Brief: "part 2 idea"},
		{Prompt: "detailed_description:\n[Shot 1]", Brief: "part 3 idea"},
	}}
	got := a.storyText()
	for _, phrase := range []string{
		"subject_definitions",
		"--- PART 2 ---",
		"summary:",
		"--- PART 3 ---",
		"detailed_description:",
	} {
		if !strings.Contains(got, phrase) {
			t.Errorf("storyText missing %q", phrase)
		}
	}
	if !strings.HasPrefix(got, "subject_definitions") {
		t.Errorf("storyText must start with part 1, got %q", got)
	}
}

func TestStoryTextEmpty(t *testing.T) {
	if got := (&App{}).storyText(); got != "" {
		t.Fatalf("empty story must yield empty text, got %q", got)
	}
}
