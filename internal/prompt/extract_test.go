package prompt

import (
	"strings"
	"testing"
)

const endingFixture = `subject_definitions:
<Subject 1> comes from <Picture 1>.

summary:
[reference generation] A man enters a room and reads a letter.

retention_analysis:
<Subject 1>: fully_preserved.

detailed_description:
[Shot 1] The man walks into the dim office and closes the door. [Shot 2] He picks up the envelope, unfolds the letter, and the camera holds on his face as he reads.

overall_soundscape:
N/A

non_diegetic_music:
N/A`

func TestExtractEndingStateLastShot(t *testing.T) {
	got := ExtractEndingState(endingFixture)
	want := "He picks up the envelope, unfolds the letter, and the camera holds on his face as he reads."
	if !strings.Contains(got, "camera holds on his face") {
		t.Fatalf("ExtractEndingState = %q, want the text after the last shot", got)
	}
	if strings.Contains(got, "[Shot 2]") {
		t.Fatalf("ExtractEndingState must strip the shot marker, got %q", got)
	}
	if strings.Contains(got, "dim office") {
		t.Fatalf("ExtractEndingState must return only the last shot, got %q", got)
	}
	if len(got) < len(want)/2 {
		t.Fatalf("ending too short: %q", got)
	}
}

func TestExtractEndingStateNoShotsFallsBackToSummary(t *testing.T) {
	prompt := `summary:
[reference generation] A calm meadow at sunrise.

detailed_description:
A wide shot of grass swaying gently in the breeze.

overall_soundscape:
N/A

non_diegetic_music:
N/A`
	got := ExtractEndingState(prompt)
	if !strings.Contains(got, "calm meadow at sunrise") {
		t.Fatalf("no-shots fallback must use the summary, got %q", got)
	}
}

func TestExtractEndingStateMalformedEmpty(t *testing.T) {
	if got := ExtractEndingState(""); got != "" {
		t.Fatalf("empty prompt must yield empty ending, got %q", got)
	}
	if got := ExtractEndingState("no sections here"); got != "" {
		t.Fatalf("malformed prompt must yield empty ending, got %q", got)
	}
}

func TestReferenceContextExcerpt(t *testing.T) {
	got := ReferenceContextExcerpt(endingFixture)
	if !strings.Contains(got, "subject_definitions") || !strings.Contains(got, "retention_analysis") {
		t.Fatalf("excerpt must include the continuity sections, got %q", got)
	}
	if strings.Contains(got, "dim office") || strings.Contains(got, "detailed_description") {
		t.Fatalf("excerpt must exclude the shot timeline, got %q", got)
	}
}

func TestExtractEndingStateSingleLongShotStaysShort(t *testing.T) {
	filler := strings.Repeat("The hero keeps sprinting across the rain-slick rooftops, leaping from ledge to ledge. ", 8)
	longShot := "subject_definitions:\n<Subject 1> comes from <Picture 1>.\n\nsummary:\n[reference generation] A chase.\n\nretention_analysis:\n<Subject 1>: fully_preserved.\n\ndetailed_description:\n[Shot 1] " + filler + "He finally reaches the broken window where he stops to catch his breath.\n\noverall_soundscape:\nN/A\n\nnon_diegetic_music:\nN/A"
	got := ExtractEndingState(longShot)
	if len(got) > 250 {
		t.Fatalf("ending too long (%d), got %q", len(got), got)
	}
	if !strings.Contains(got, "broken window") {
		t.Fatalf("ending must keep the final state, got %q", got)
	}
	if len(got) >= len(filler) {
		t.Fatalf("ending must be only a tail of the description, got %q", got)
	}
}
