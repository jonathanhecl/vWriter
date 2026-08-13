package prompt

import (
	"strings"
	"testing"
)

func TestReferenceTagsNormalized(t *testing.T) {
	tags := ReferenceTags("Use <picture 1>, <Video 2>, and <AUDIO 3>.")
	for _, want := range []string{"<Picture 1>", "<Video 2>", "<Audio 3>"} {
		if !tags[want] {
			t.Errorf("missing %q in %v", want, tags)
		}
	}
	if len(tags) != 3 {
		t.Fatalf("tags = %v", tags)
	}
}

func TestAudioTaskWithoutAudioAssetIsHardError(t *testing.T) {
	expected := map[string]bool{"<Picture 1>": true, "<Video 1>": true}
	if !UnexpectedAudioTask("reference generation + audio reference", expected) {
		t.Fatal("audio reference without audio asset must be flagged")
	}
	if UnexpectedAudioTask("reference generation", expected) {
		t.Fatal("plain generation must not be flagged")
	}
	expected["<Audio 1>"] = true
	if UnexpectedAudioTask("reference generation + audio reference", expected) {
		t.Fatal("audio asset present must not be flagged")
	}
}

func TestMotionOnlyProvenanceCheckIsNarrow(t *testing.T) {
	brief := "Use Video 1 only as the motion reference."
	bad := "<Subject 2> is the studio environment from <Video 1>."
	good := "A new target street frames the choreography from <Video 1>."
	if len(ExplicitConstraintViolations(brief, bad)) == 0 {
		t.Fatal("excluded trait provenance must be flagged")
	}
	if got := ExplicitConstraintViolations(brief, good); len(got) != 0 {
		t.Fatalf("clean prompt flagged: %v", got)
	}
}

func TestExplicitNoCutsEnforcedButUnspecifiedCameraIsNot(t *testing.T) {
	prompt := "[Shot 1] A tracking shot. [Shot 2] Cut to a close-up."
	if len(ExplicitConstraintViolations("Use one continuous shot with no cuts.", prompt)) == 0 {
		t.Fatal("no-cuts violation expected")
	}
	if got := ExplicitConstraintViolations("Make it cinematic.", prompt); len(got) != 0 {
		t.Fatalf("unexpected violations: %v", got)
	}
}

func TestNarrowRepairReceivesOriginalRequestAndExactViolations(t *testing.T) {
	assembled := &Assembled{Messages: []Message{
		{Role: "system", Content: "guide"},
		{Role: "user", Content: "Creative brief:\nUse Video 1 only for motion. Add some music."},
	}}
	messages := NarrowRepairMessages(
		assembled,
		"[reference generation + audio reference] draft with <Audio 1>",
		[]string{"unexpected reference tags: <Audio 1>"},
		map[string]bool{"<Picture 1>": true, "<Video 1>": true},
		10,
	)
	if !strings.Contains(messages[0].Content, "not a new prompt-generation pass") {
		t.Error("system must frame a narrow correction pass")
	}
	if !strings.Contains(messages[0].Content, "unexpected reference tags: <Audio 1>") {
		t.Error("system must list the exact violations")
	}
	if !strings.Contains(messages[0].Content, "<Picture 1>, <Video 1>") {
		t.Error("system must list the allowed tags")
	}
	if !strings.Contains(messages[1].Content, "Use Video 1 only for motion") {
		t.Error("user must contain the original request")
	}
}

func TestFailureSummaryContainsOnlyObjectiveChecks(t *testing.T) {
	failures := AuditFailures(&Audit{
		SectionOrderValid:         true,
		MissingTaskLabel:          true,
		UnexpectedReferenceTags:   []string{"<Audio 1>"},
		UnexpectedAudioTask:       true,
		DetailedDescriptionLength: "short_internal_warning",
	})
	joined := strings.Join(failures, "\n")
	if !strings.Contains(joined, "missing summary task label") ||
		!strings.Contains(joined, "unexpected reference tags: <Audio 1>") {
		t.Fatalf("failures = %v", failures)
	}
	if strings.Contains(joined, "word") {
		t.Fatal("quality length warnings must not appear in repair failures")
	}
}

func TestUndeclaredMediaMentions(t *testing.T) {
	imagesOnly := map[string]bool{"<Picture 1>": true, "<Picture 2>": true}

	hallucinated := "The camera follows the motion of the reference video while the source audio plays."
	if got := UndeclaredMediaMentions(hallucinated, imagesOnly); len(got) == 0 {
		t.Fatal("undeclared video and audio mentions must be flagged")
	}

	bareTag := "The scene continues the action of Video 1."
	if got := UndeclaredMediaMentions(bareTag, imagesOnly); len(got) == 0 {
		t.Fatal("bare numbered mention of an undeclared kind must be flagged")
	}

	declared := map[string]bool{"<Picture 1>": true, "<Video 1>": true}
	legit := "The reference video drives the motion; the pictures stay consistent."
	if got := UndeclaredMediaMentions(legit, declared); len(got) != 0 {
		t.Fatalf("declared kinds must not be flagged: %v", got)
	}

	clean := "A dancer turns in an empty room. non_diegetic_music: N/A"
	if got := UndeclaredMediaMentions(clean, imagesOnly); len(got) != 0 {
		t.Fatalf("clean prompt flagged: %v", got)
	}
}

func TestSanitizePrompt(t *testing.T) {
	allowed := map[string]bool{"<Picture 1>": true, "<Subject 1>": true}
	prompt := "subject_definitions:\n<Subject 1> comes from <Picture 1>.\n\nretention_analysis:\n<Picture 1>: fully_preserved.\n<Audio 1>: fully_preserved - the invented voice track.\n\noverall_soundscape:\n<Video 2> drives the rhythm.\n<Subject 3> appears unexpectedly.\n\nnon_diegetic_music:\nN/A"

	got := SanitizePrompt(prompt, allowed)
	if strings.Contains(got, "<Audio 1>") {
		t.Fatalf("invented audio tag must be removed, got %q", got)
	}
	if strings.Contains(got, "<Video 2>") {
		t.Fatalf("invented video tag must be removed, got %q", got)
	}
	if strings.Contains(got, "<Subject 3>") {
		t.Fatalf("invented subject tag must be removed, got %q", got)
	}
	if !strings.Contains(got, "<Picture 1>") || !strings.Contains(got, "<Subject 1>") {
		t.Fatalf("declared references must survive, got %q", got)
	}
}

func TestSubjectTags(t *testing.T) {
	got := SubjectTags("subject_definitions:\n<Subject 1> comes from <Picture 1>.\n<Subject 2> follows.\n")
	if len(got) != 2 || !got["<Subject 1>"] || !got["<Subject 2>"] {
		t.Fatalf("SubjectTags = %v", got)
	}
	if got := SubjectTags("<Picture 1> and <Audio 2> only"); len(got) != 0 {
		t.Fatalf("media tags must not be counted as subjects: %v", got)
	}
}

func TestSanitizePromptNoChange(t *testing.T) {
	expected := map[string]bool{"<Picture 1>": true, "<Audio 1>": true}
	text := "retention_analysis:\n<Picture 1>: fully_preserved.\n<Audio 1>: fully_preserved.\n"
	if got := SanitizePrompt(text, expected); got != text {
		t.Fatalf("sanitize must not change a clean prompt, got %q", got)
	}
}

func TestFinalTextCleansSpecialTokens(t *testing.T) {
	if got := FinalText("  prompt body<|end_of_turn|>  "); got != "prompt body" {
		t.Fatalf("got %q", got)
	}
	if got := FinalText("text<eos>"); got != "text" {
		t.Fatalf("got %q", got)
	}
}
