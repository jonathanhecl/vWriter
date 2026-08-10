package guides

import (
	"strings"
	"testing"
)

func TestLoadIntegrity(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	base, err := Base()
	if err != nil {
		t.Fatal(err)
	}
	reference, err := Reference()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(base.Content, "## ") || !strings.Contains(reference.Content, "subject_definitions") {
		t.Fatal("guide content looks wrong")
	}
	if base.SourceRevision != sourceRevision || !strings.HasPrefix(base.SourceURL, sourceRoot) {
		t.Fatal("provenance metadata missing")
	}
}

func TestReferenceBaseExcerpt(t *testing.T) {
	excerpt, err := ReferenceBaseExcerpt()
	if err != nil {
		t.Fatalf("ReferenceBaseExcerpt: %v", err)
	}
	for _, section := range []string{
		"4.2 Shots and Cuts",
		"4.3 Camera Motion: Motion Type + Amplitude + Speed",
		"4.4 Speakers, Dialogue, and Singing",
		"4.5 On-Screen Text",
		"4.6 overall_soundscape",
		"4.7 non_diegetic_music",
	} {
		if !strings.Contains(excerpt, "### "+section) {
			t.Errorf("excerpt missing section %q", section)
		}
	}
	if strings.Contains(excerpt, "```") {
		t.Error("excerpt must contain prose only, no code blocks")
	}
}
