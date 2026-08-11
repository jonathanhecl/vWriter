package prompt

import (
	"strings"
	"testing"

	"github.com/jonathanhecl/vWriter/internal/media"
)

func strPtr(value string) *string { return &value }

func TestReferenceSystemWrapperContent(t *testing.T) {
	for _, phrase := range []string{
		"transfer only that role",
		"must not contribute its performer identity",
		"never invent or pad details solely",
		"preserve user-supplied dialogue, lyrics, and visible text verbatim",
		"unsupported subject actions, expressions, events, transitions",
	} {
		if !strings.Contains(ReferenceSystemWrapper, phrase) {
			t.Errorf("wrapper missing %q", phrase)
		}
	}
	for _, phrase := range []string{"never return fewer", "spins", "kisses", "GRWM"} {
		if strings.Contains(ReferenceSystemWrapper, phrase) {
			t.Errorf("wrapper must not contain %q", phrase)
		}
	}
}

func TestCustomPromptFullyReplacesDefault(t *testing.T) {
	prompt, custom, err := ResolveSystemPrompt(strPtr("  Custom instruction.  "))
	if err != nil || prompt != "Custom instruction." || !custom {
		t.Fatalf("got %q, %v, %v", prompt, custom, err)
	}
}

func TestOversizedCustomPromptRejected(t *testing.T) {
	_, _, err := ResolveSystemPrompt(strPtr(strings.Repeat("x", MaxSystemPromptChars+1)))
	if !Is(err, "SYSTEM_PROMPT_TOO_LONG") {
		t.Fatalf("err = %v", err)
	}
}

func TestReferenceContractBoundsCreativeCompletion(t *testing.T) {
	contract := finalContract("Use Video 1 only for motion. Add some music.")
	for _, phrase := range []string{
		"every explicitly assigned reference role as exclusive",
		"are not required",
		"may be designed as new target content",
		"never described as facts derived from a reference",
		"must not create audio-reference or audio-reuse semantics",
		"concrete visible object, character, scene, or effect",
		"through an appropriate <Subject N>",
		"do not automatically create a separate subject for ordinary motion transfer",
		"non_diegetic_music must be N/A",
	} {
		if !strings.Contains(contract, phrase) {
			t.Errorf("contract missing %q", phrase)
		}
	}
	if !strings.Contains(contract, "reference generation, not keyframe completion") {
		t.Error("brief without editing intent must classify as reference generation")
	}
	editContract := finalContract("Edit this video by extending it.")
	if !strings.Contains(editContract, "source-video editing or continuation") {
		t.Error("editing brief must classify as source-video editing")
	}
}

func validManifest() media.Manifest {
	return media.Manifest{
		SessionID: "s1",
		Valid:     true,
		Counts:    map[media.AssetType]int{media.Image: 1},
		Assets: []*media.Asset{{
			ID: "a1", Type: media.Image, Filename: "hero.jpg", Reference: "<Picture 1>",
			AnalysisRequested: true, PreparedPath: "prepared.jpg",
		}},
	}
}

func validGenerateRequest() GenerateRequest {
	return GenerateRequest{
		Manifest:        validManifest(),
		CreativeBrief:   "A quiet morning scene.",
		DurationSeconds: 10,
		AspectRatio:     "16:9",
	}
}

func TestAssembleRequest(t *testing.T) {
	assembled, err := AssembleRequest(validGenerateRequest())
	if err != nil {
		t.Fatalf("AssembleRequest: %v", err)
	}
	if len(assembled.Messages) != 4 {
		t.Fatalf("messages = %d, want 4", len(assembled.Messages))
	}
	user := assembled.Messages[3]
	if user.Role != "user" || !strings.Contains(user.Content, "Mode: Reference") ||
		!strings.Contains(user.Content, "<Picture 1>: hero.jpg (image)") {
		t.Fatalf("user message = %q", user.Content)
	}
	if len(assembled.MediaInputs) != 1 || assembled.MediaInputs[0].ImagePath != "prepared.jpg" {
		t.Fatalf("media inputs = %+v", assembled.MediaInputs)
	}
	if assembled.SystemPrompt.Custom || assembled.SystemPrompt.Content != ReferenceSystemWrapper {
		t.Fatal("default system prompt expected")
	}
	if assembled.Guide.Content != "" || assembled.Guide.ContentSHA256 == "" {
		t.Fatal("guide metadata must carry the digest but not the content")
	}
}

func TestAssembleRequestValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*GenerateRequest)
		code   string
	}{
		{"empty brief", func(r *GenerateRequest) { r.CreativeBrief = "  " }, "INVALID_REQUEST"},
		{"brief too long", func(r *GenerateRequest) { r.CreativeBrief = strings.Repeat("x", 2001) }, "BRIEF_TOO_LONG"},
		{"bad aspect ratio", func(r *GenerateRequest) { r.AspectRatio = "5:4" }, "INVALID_ASPECT_RATIO"},
		{"bad duration", func(r *GenerateRequest) { r.DurationSeconds = 25 }, "INVALID_DURATION"},
		{"invalid manifest", func(r *GenerateRequest) { r.Manifest.Valid = false }, "INVALID_MEDIA_MANIFEST"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validGenerateRequest()
			tc.mutate(&req)
			if _, err := AssembleRequest(req); !Is(err, tc.code) {
				t.Fatalf("err = %v, want %s", err, tc.code)
			}
		})
	}
}

func TestAnalysisToggleExcludesVisualInput(t *testing.T) {
	req := validGenerateRequest()
	req.Manifest.Assets[0].AnalysisRequested = false
	assembled, err := AssembleRequest(req)
	if err != nil {
		t.Fatalf("AssembleRequest: %v", err)
	}
	if len(assembled.MediaInputs) != 0 {
		t.Fatal("excluded asset must not reach the model")
	}
	if !strings.Contains(assembled.Messages[3].Content, "Reference manifest") {
		t.Fatal("manifest section must still exist")
	}
}

func TestAudioDeclaredButNeverAnalyzed(t *testing.T) {
	req := validGenerateRequest()
	req.Manifest.Assets = append(req.Manifest.Assets, &media.Asset{
		ID: "a2", Type: media.Audio, Filename: "voice.mp3", Reference: "<Audio 1>", Duration: 8,
	})
	assembled, err := AssembleRequest(req)
	if err != nil {
		t.Fatalf("AssembleRequest: %v", err)
	}
	if len(assembled.MediaInputs) != 1 {
		t.Fatal("audio must never become a media input")
	}
	if !strings.Contains(assembled.Messages[3].Content, "<Audio 1>: voice.mp3 (audio, 8s") {
		t.Fatal("audio must stay in the declared manifest")
	}
}

func TestBoundaryPropagatesToMediaInputAndManifest(t *testing.T) {
	req := validGenerateRequest()
	req.Manifest.Assets[0].Role = media.RoleFirstFrame
	assembled, err := AssembleRequest(req)
	if err != nil {
		t.Fatalf("AssembleRequest: %v", err)
	}
	if len(assembled.MediaInputs) != 1 || assembled.MediaInputs[0].Boundary != media.RoleFirstFrame {
		t.Fatalf("media inputs = %+v", assembled.MediaInputs)
	}
	if !strings.Contains(assembled.Messages[3].Content, "MUST start with this exact image") {
		t.Fatal("manifest must carry the first_frame constraint")
	}

	req = validGenerateRequest()
	req.Manifest.Assets[0].Role = media.RoleLastFrame
	assembled, err = AssembleRequest(req)
	if err != nil {
		t.Fatalf("AssembleRequest: %v", err)
	}
	if len(assembled.MediaInputs) != 1 || assembled.MediaInputs[0].Boundary != media.RoleLastFrame {
		t.Fatalf("media inputs = %+v", assembled.MediaInputs)
	}
	if !strings.Contains(assembled.Messages[3].Content, "MUST end with this exact image") {
		t.Fatal("manifest must carry the last_frame constraint")
	}
}

func validContinuationRequest() ContinuationRequest {
	manifest := validManifest()
	manifest.Assets[0].Role = media.RoleFirstFrame
	return ContinuationRequest{
		Manifest:               manifest,
		PartBrief:              "The hero follows the clue into the warehouse.",
		DurationSeconds:        10,
		AspectRatio:            "16:9",
		PreviousPrompt:         endingFixture,
		PreviousEnding:         ExtractEndingState(endingFixture),
		ContinuationFrameLabel: "<Picture 2>",
	}
}

func TestAssembleContinuation(t *testing.T) {
	assembled, err := AssembleContinuation(validContinuationRequest())
	if err != nil {
		t.Fatalf("AssembleContinuation: %v", err)
	}
	user := assembled.Messages[len(assembled.Messages)-1].Content
	for _, phrase := range []string{
		"video continuation",
		"<Picture 2>",
		"camera holds on his face",
		"Previous part",
		"The hero follows the clue into the warehouse.",
		"MUST open with the continuation frame",
	} {
		if !strings.Contains(user, phrase) {
			t.Errorf("continuation user message missing %q", phrase)
		}
	}
	// Real media is re-attached for consistency.
	if len(assembled.MediaInputs) != 1 || assembled.MediaInputs[0].ImagePath != "prepared.jpg" {
		t.Fatalf("media inputs = %+v", assembled.MediaInputs)
	}
	// First/last frame image anchors must be neutralized in continuations.
	if strings.Contains(user, "MUST start with this exact image") {
		t.Fatal("continuation must not carry the image first_frame MUST anchor")
	}
	if assembled.MediaInputs[0].Boundary != "" {
		t.Fatalf("continuation media input must not carry a boundary, got %q", assembled.MediaInputs[0].Boundary)
	}
	// The manifest still lists the asset but without the anchor marker.
	if !strings.Contains(user, "role: first_frame") {
		t.Fatal("asset role must still be declared (without the MUST wording)")
	}
}

func TestAssembleContinuationValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ContinuationRequest)
		code   string
	}{
		{"empty brief", func(r *ContinuationRequest) { r.PartBrief = "  " }, "INVALID_REQUEST"},
		{"brief too long", func(r *ContinuationRequest) { r.PartBrief = strings.Repeat("x", 2001) }, "BRIEF_TOO_LONG"},
		{"bad aspect ratio", func(r *ContinuationRequest) { r.AspectRatio = "5:4" }, "INVALID_ASPECT_RATIO"},
		{"bad duration", func(r *ContinuationRequest) { r.DurationSeconds = 25 }, "INVALID_DURATION"},
		{"invalid manifest", func(r *ContinuationRequest) { r.Manifest.Valid = false }, "INVALID_MEDIA_MANIFEST"},
		{"missing previous prompt", func(r *ContinuationRequest) { r.PreviousPrompt = "" }, "INVALID_REQUEST"},
		{"missing ending", func(r *ContinuationRequest) { r.PreviousEnding = "" }, "INVALID_REQUEST"},
		{"missing frame label", func(r *ContinuationRequest) { r.ContinuationFrameLabel = "" }, "INVALID_REQUEST"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validContinuationRequest()
			tc.mutate(&req)
			if _, err := AssembleContinuation(req); !Is(err, tc.code) {
				t.Fatalf("err = %v, want %s", err, tc.code)
			}
		})
	}
}

func TestAssembleRefinement(t *testing.T) {
	assembled, err := AssembleRefinement(RefineRequest{
		Manifest:          validManifest(),
		CurrentPrompt:     "subject_definitions:\n...\n\n",
		Instruction:       "Make the lighting colder.",
		CachedObservation: "first-pass observation",
	})
	if err != nil {
		t.Fatalf("AssembleRefinement: %v", err)
	}
	user := assembled.Messages[len(assembled.Messages)-1].Content
	for _, phrase := range []string{"Rewrite the current H3 prompt", "media is intentionally not attached",
		"first-pass observation", "Make the lighting colder."} {
		if !strings.Contains(user, phrase) {
			t.Errorf("refinement user message missing %q", phrase)
		}
	}
	if len(assembled.MediaInputs) != 0 {
		t.Fatal("refinement must not attach media")
	}
	if _, err := AssembleRefinement(RefineRequest{
		Manifest: validManifest(), CurrentPrompt: strings.Repeat("x", 20001), Instruction: "ok",
	}); !Is(err, "PROMPT_TOO_LONG") {
		t.Fatal("oversized prompt must be rejected")
	}
	if _, err := AssembleRefinement(RefineRequest{
		Manifest: validManifest(), CurrentPrompt: "ok", Instruction: strings.Repeat("x", 2001),
	}); !Is(err, "INSTRUCTION_TOO_LONG") {
		t.Fatal("oversized instruction must be rejected")
	}
}
