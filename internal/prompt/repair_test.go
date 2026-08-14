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

func TestSanitizeMediaPrompt(t *testing.T) {
	allowedMedia := map[string]bool{"<Picture 1>": true}
	prompt := "subject_definitions:\n<Subject 1> comes from <Picture 1>.\n\nretention_analysis:\n<Picture 1>: fully_preserved.\n<Audio 1>: fully_preserved - the invented voice track.\n\noverall_soundscape:\n<Video 2> drives the rhythm.\n<Video None> is not specified as a structural source video.\n<Audio None> is not specified for audio content.\n\ndetailed_description:\n[Shot 1] The hero crosses the long warehouse floor, stepping past a long row of crates, and this full prose line must never be removed because it carries the story.\n\nnon_diegetic_music:\nN/A"

	got := SanitizeMediaPrompt(prompt, allowedMedia)
	for _, invented := range []string{"<Audio 1>", "<Video 2>", "<Video None>", "<Audio None>"} {
		if strings.Contains(got, invented) {
			t.Fatalf("invented tag %q must be removed, got %q", invented, got)
		}
	}
	if !strings.Contains(got, "<Picture 1>") || !strings.Contains(got, "<Subject 1>") {
		t.Fatalf("declared references must survive, got %q", got)
	}
	if !strings.Contains(got, "The hero crosses the long warehouse floor") {
		t.Fatalf("long prose must never be removed, got %q", got)
	}
}

func TestSanitizeMediaPromptKeepsMixedAllowedLine(t *testing.T) {
	// A line that cites an allowed reference (the source video) alongside an
	// invented one must be preserved so valid references are never lost.
	allowedMedia := map[string]bool{"<Video 1>": true}
	text := "retention_analysis:\n<Video 1> is continued while <Audio 3> plays underneath.\n"
	got := SanitizeMediaPrompt(text, allowedMedia)
	if !strings.Contains(got, "<Video 1>") {
		t.Fatalf("allowed reference must survive even on a mixed line, got %q", got)
	}
}

func TestSanitizeMediaPromptNoChange(t *testing.T) {
	allowedMedia := map[string]bool{"<Picture 1>": true, "<Audio 1>": true}
	text := "retention_analysis:\n<Picture 1>: fully_preserved.\n<Audio 1>: fully_preserved.\n"
	if got := SanitizeMediaPrompt(text, allowedMedia); got != text {
		t.Fatalf("sanitize must not change a clean prompt, got %q", got)
	}
}

func TestSanitizeMediaPromptKeepsLongProse(t *testing.T) {
	// A long prose line mentioning an undeclared tag must never be dropped.
	allowedMedia := map[string]bool{}
	long := strings.Repeat("The camera pans slowly across the entire set ", 12) + "with <Video 9> in the background."
	got := SanitizeMediaPrompt(long, allowedMedia)
	if got != long {
		t.Fatalf("long prose must be kept untouched, got %q", got)
	}
}

func TestEnforceContinuationContract(t *testing.T) {
	// The model omitted both the continuation declaration and the source video.
	text := "subject_definitions:\n<Subject 1> comes from <Picture 1>.\n\nsummary:\n[reference generation] The man follows the clue.\n\nretention_analysis:\n<Subject 1>: fully_preserved.\n\ndetailed_description:\n[Shot 1] The man walks through the warehouse door.\n\noverall_soundscape:\nN/A\n\nnon_diegetic_music:\nN/A"
	got := EnforceContinuationContract(text, "<Video 2>")
	if !strings.Contains(got, "<Video 2>") {
		t.Fatalf("source video must be cited, got %q", got)
	}
	if !strings.Contains(got, "[video continuation + reference generation]") {
		t.Fatalf("continuation must be named in the task label, got %q", got)
	}
	if !strings.Contains(got, "The man walks through the warehouse door") {
		t.Fatalf("existing content must be preserved, got %q", got)
	}
}

func TestEnforceContinuationContractIdempotent(t *testing.T) {
	text := "subject_definitions:\n<Video 2> is the source video being continued.\n\nsummary:\n[video continuation] The man follows the clue.\n"
	if got := EnforceContinuationContract(text, "<Video 2>"); got != text {
		t.Fatalf("clean continuation must be left unchanged, got %q", got)
	}
}

func TestEnforceContinuationContractAddsLabelWhenMissing(t *testing.T) {
	text := "summary:\nThe man follows the clue.\n"
	got := EnforceContinuationContract(text, "<Video 2>")
	if !strings.Contains(got, "[video continuation] The man follows the clue.") {
		t.Fatalf("missing task label must be added, got %q", got)
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

func TestFixPlaceholderLabels(t *testing.T) {
	text := "The shot continues from <Video N> while <Subject N> follows <Picture N> and <Audio N> plays."
	got := FixPlaceholderLabels(text, "<Video 1>")
	if !strings.Contains(got, "<Video 1>") {
		t.Fatalf("video placeholder must become the source label, got %q", got)
	}
	for _, placeholder := range []string{"<Video N>", "<Subject N>", "<Picture N>", "<Audio N>"} {
		if strings.Contains(got, placeholder) {
			t.Fatalf("placeholder %q must be removed, got %q", placeholder, got)
		}
	}
	// Without a source label, <Video N> is dropped too.
	if got := FixPlaceholderLabels("By <Video N> the end.", ""); strings.Contains(got, "<Video N>") {
		t.Fatalf("video placeholder must be dropped without a source, got %q", got)
	}
}

func TestMalformedTags(t *testing.T) {
	got := MalformedTags("<Video None> is not specified. <Audio N> too. <Picture 1> is fine. <Subject None> here.")
	for _, want := range []string{"<Video None>", "<Audio N>", "<Subject None>"} {
		found := false
		for _, tag := range got {
			if tag == want {
				found = true
			}
		}
		if !found {
			t.Errorf("MalformedTags must include %q, got %v", want, got)
		}
	}
	for _, tag := range got {
		if tag == "<Picture 1>" {
			t.Errorf("valid tag must not be reported as malformed: %v", got)
		}
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
