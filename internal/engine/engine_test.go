package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonathanhecl/vWriter/internal/media"
	"github.com/jonathanhecl/vWriter/internal/ollama"
	"github.com/jonathanhecl/vWriter/internal/prompt"
)

// validPrompt is a structurally valid full-reference output with a reference
// tag matching the test manifest.
const validPrompt = `subject_definitions:
<Subject 1> comes from <Picture 1>.

summary:
[reference generation] A restrained shot.

retention_analysis:
<Subject 1>: fully_preserved.

detailed_description:
[Shot 1] ` + "%DETAILED%" + `

overall_soundscape:
N/A

non_diegetic_music:
N/A`

// brokenPrompt misses the [Shot 1] marker, so it must trigger a repair.
const brokenPrompt = `subject_definitions:
<Subject 1> comes from <Picture 1>.

summary:
[reference generation] A restrained shot.

retention_analysis:
<Subject 1>: fully_preserved.

detailed_description:
%DETAILED%

overall_soundscape:
N/A

non_diegetic_music:
N/A`

// continuationPrompt is a structurally valid continuation that references the
// real asset and the source video being continued.
const continuationPrompt = `subject_definitions:
<Subject 1> comes from <Picture 1>.

summary:
[video continuation] The man follows the clue.

retention_analysis:
<Subject 1>: fully_preserved.
<Video 2>: fully_preserved.

detailed_description:
[Shot 1] ` + "%DETAILED%" + `

overall_soundscape:
N/A

non_diegetic_music:
N/A`

func buildPrompt(template string, words int) string {
	return strings.Replace(template, "%DETAILED%", strings.TrimRight(strings.Repeat("visible ", words), " "), 1)
}

// fakeOllama serves /api/show with vision and streams the queued chat texts.
type fakeOllama struct {
	t            *testing.T
	mu           sync.Mutex
	responses    []string
	requests     [][]byte
	block        chan struct{}
	seenRepair   bool
	seenGenerate bool
}

func (f *fakeOllama) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"capabilities": []string{"completion", "vision"}})
	})
	mux.HandleFunc("/api/generate", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		if f.block != nil {
			<-f.block
		}
		body := make([]byte, 0)
		raw := new(strings.Builder)
		buf := make([]byte, 4096)
		for {
			n, err := r.Body.Read(buf)
			raw.Write(buf[:n])
			if err != nil {
				break
			}
		}
		body = []byte(raw.String())
		f.mu.Lock()
		f.requests = append(f.requests, body)
		var text string
		if len(f.responses) > 0 {
			text = f.responses[0]
			f.responses = f.responses[1:]
		}
		if strings.Contains(string(body), "DRAFT TO CORRECT") {
			f.seenRepair = true
		} else {
			f.seenGenerate = true
		}
		f.mu.Unlock()
		for _, chunk := range splitChunks(text) {
			fmt.Fprintf(w, `{"message":{"role":"assistant","content":%s},"done":false}`+"\n", mustJSON(chunk))
		}
		fmt.Fprintf(w, `{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":100,"eval_count":50,"total_duration":2000000000,"eval_duration":1000000000}`+"\n")
	})
	return mux
}

func splitChunks(text string) []string {
	const size = 64
	var chunks []string
	for len(text) > size {
		chunks = append(chunks, text[:size])
		text = text[size:]
	}
	return append(chunks, text)
}

func mustJSON(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

// testRig wires a store with one image asset to a fake Ollama engine.
func testRig(t *testing.T, fake *fakeOllama) (*Engine, string) {
	t.Helper()
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)

	store := media.NewStore(t.TempDir())
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for i := range img.Pix {
		img.Pix[i] = uint8(i % 255)
	}
	_ = color.Black
	path := filepath.Join(t.TempDir(), "hero.jpg")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(out, img, nil); err != nil {
		t.Fatal(err)
	}
	out.Close()
	if _, err := store.Add("s1", path); err != nil {
		t.Fatalf("Add: %v", err)
	}

	eng := NewEngine(store)
	if err := eng.SetOllamaURL(server.URL); err != nil {
		t.Fatal(err)
	}
	return eng, "s1"
}

func generateParams(session string) GenerateParams {
	return GenerateParams{
		SessionID:       session,
		Model:           "vision-model:latest",
		CreativeBrief:   "A quiet morning scene with <Picture 1>.",
		DurationSeconds: 10,
		AspectRatio:     "16:9",
		ContextProfile:  "auto",
		KeepModelLoaded: true,
	}
}

func TestGenerateHappyPath(t *testing.T) {
	fake := &fakeOllama{t: t, responses: []string{buildPrompt(validPrompt, 400)}}
	eng, session := testRig(t, fake)

	var phases []string
	result, err := eng.Generate(func() GenerateParams {
		p := generateParams(session)
		p.OnPhase = func(phase string) { phases = append(phases, phase) }
		return p
	}())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(result.Prompt, "subject_definitions") || !result.Audit.OfficialFormatPass {
		t.Fatalf("result = %+v", result)
	}
	if result.RepairAttempted {
		t.Fatal("no repair expected on a clean output")
	}
	if result.PromptTokens != 100 || result.OutputTokens != 50 || result.TokensPerSecond != 50 {
		t.Fatalf("metrics = %+v", result)
	}
	wantPhases := []string{PhaseLoadingModel, PhaseProcessingMedia, PhaseGenerating}
	if strings.Join(phases, ",") != strings.Join(wantPhases, ",") {
		t.Fatalf("phases = %v", phases)
	}
	if eng.CachedGeneration(session) != result.Prompt {
		t.Fatal("generation cache not filled")
	}
	// Media bindings must precede the brief in the streamed request.
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.requests) != 1 {
		t.Fatalf("requests = %d", len(fake.requests))
	}
	body := string(fake.requests[0])
	// Go's json.Marshal escapes angle brackets: <Picture 1> -> <Picture 1>.
	if !strings.Contains(body, `Picture 1\u003e: image reference.`) {
		t.Fatal("missing image binding")
	}
	if !strings.Contains(body, `"images":["`) {
		t.Fatal("the prepared image must be attached as base64")
	}
	if !strings.Contains(body, `"num_ctx":16384`) {
		t.Fatal("auto context must resolve to 16384")
	}
}

func TestGenerateKeepsBrokenOutputIntact(t *testing.T) {
	// No model-based repair runs; a structurally imperfect prompt is kept
	// as-is so content is never destroyed.
	fake := &fakeOllama{t: t, responses: []string{
		buildPrompt(brokenPrompt, 400), // missing [Shot 1]
	}}
	eng, session := testRig(t, fake)

	result, err := eng.Generate(generateParams(session))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.RepairAttempted {
		t.Fatal("no model repair is expected anymore")
	}
	if !strings.Contains(result.Prompt, "detailed_description") || !strings.Contains(result.Prompt, "visible") {
		t.Fatalf("prompt content must be preserved, got %q", result.Prompt)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.requests) != 1 {
		t.Fatalf("requests = %d, want a single generation (no repair)", len(fake.requests))
	}
}

func TestGenerateCancel(t *testing.T) {
	fake := &fakeOllama{t: t, block: make(chan struct{})}
	eng, session := testRig(t, fake)
	defer close(fake.block)

	done := make(chan error, 1)
	go func() {
		_, err := eng.Generate(generateParams(session))
		done <- err
	}()
	time.Sleep(200 * time.Millisecond)
	if !eng.Cancel() {
		t.Fatal("Cancel must report an active generation")
	}
	err := <-done
	var oerr *ollama.Error
	if !errors.As(err, &oerr) || oerr.Code != "GENERATION_CANCELLED" {
		t.Fatalf("err = %v, want GENERATION_CANCELLED", err)
	}
}

func TestGenerateBusy(t *testing.T) {
	fake := &fakeOllama{t: t, block: make(chan struct{})}
	eng, session := testRig(t, fake)
	defer close(fake.block)

	done := make(chan error, 1)
	go func() {
		_, err := eng.Generate(generateParams(session))
		done <- err
	}()
	time.Sleep(200 * time.Millisecond)
	if _, err := eng.Generate(generateParams(session)); !Is(err, "GENERATION_BUSY") {
		t.Fatalf("err = %v, want GENERATION_BUSY", err)
	}
	eng.Cancel()
	<-done
}

func TestGenerateBoundaryBinding(t *testing.T) {
	for _, tc := range []struct {
		name   string
		role   string
		phrase string
	}{
		{"first frame", media.RoleFirstFrame, "MUST start with this exact image"},
		{"last frame", media.RoleLastFrame, "MUST end with this exact image"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeOllama{t: t, responses: []string{buildPrompt(validPrompt, 400)}}
			eng, session := testRig(t, fake)
			assets := eng.Store.List(session)
			if _, err := eng.Store.SetRole(session, assets[0].ID, tc.role, "", ""); err != nil {
				t.Fatalf("SetRole: %v", err)
			}
			if _, err := eng.Generate(generateParams(session)); err != nil {
				t.Fatalf("Generate: %v", err)
			}
			fake.mu.Lock()
			defer fake.mu.Unlock()
			if !strings.Contains(string(fake.requests[0]), tc.phrase) {
				t.Fatalf("missing %q in chat request", tc.phrase)
			}
		})
	}
}

func TestGenerateContinuationReusesSourceVideoLabel(t *testing.T) {
	fake := &fakeOllama{t: t, responses: []string{
		buildPrompt(validPrompt, 400),        // part 1
		buildPrompt(continuationPrompt, 400), // part 2
		buildPrompt(continuationPrompt, 400), // part 3 (second extend)
	}}
	eng, session := testRig(t, fake)

	part1, err := eng.Generate(generateParams(session))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	sourceLabel := "<Video 2>"
	part2, err := eng.GenerateContinuation(ContinuationParams{
		SessionID:        session,
		Model:            "vision-model:latest",
		PartBrief:        "The hero follows the clue into the warehouse.",
		DurationSeconds:  10,
		AspectRatio:      "16:9",
		PreviousPrompt:   part1.Prompt,
		PreviousEnding:   prompt.ExtractEndingState(part1.Prompt),
		SourceVideoLabel: sourceLabel,
		ContextProfile:   "auto",
		KeepModelLoaded:  true,
	})
	if err != nil {
		t.Fatalf("GenerateContinuation: %v", err)
	}
	part3, err := eng.GenerateContinuation(ContinuationParams{
		SessionID:        session,
		Model:            "vision-model:latest",
		PartBrief:        "He reaches the vault.",
		DurationSeconds:  10,
		AspectRatio:      "16:9",
		PreviousPrompt:   part2.Prompt,
		PreviousEnding:   prompt.ExtractEndingState(part2.Prompt),
		SourceVideoLabel: sourceLabel,
		ContextProfile:   "auto",
		KeepModelLoaded:  true,
	})
	if err != nil {
		t.Fatalf("GenerateContinuation: %v", err)
	}
	if part1.Prompt == "" || part2.Prompt == "" || part3.Prompt == "" {
		t.Fatal("continuation produced an empty prompt")
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(fake.requests))
	}
	req2 := string(fake.requests[1])
	for _, phrase := range []string{
		"video continuation",
		"Source video to continue",
		`Video 2\u003e`, // the source video label (json-escaped angle bracket)
		"warehouse",     // the user-written idea for this part
		`"images":["`,   // real media re-attached
	} {
		if !strings.Contains(req2, phrase) {
			t.Errorf("continuation request missing %q", phrase)
		}
	}
	if strings.Contains(req2, "MUST start with this exact image") {
		t.Error("continuation request must not carry the image first_frame anchor")
	}
	if strings.Contains(req2, "Picture 2\u003e") {
		t.Error("continuation request must not invent a synthetic <Picture N> first frame")
	}
	req3 := string(fake.requests[2])
	if !strings.Contains(req3, `Video 2\u003e`) {
		t.Fatal("the second extend must reuse the same source video label")
	}
}

func TestRefineWithoutMedia(t *testing.T) {
	fake := &fakeOllama{t: t, responses: []string{buildPrompt(validPrompt, 420)}}
	eng, session := testRig(t, fake)

	result, err := eng.Refine(RefineParams{
		SessionID:      session,
		Model:          "vision-model:latest",
		CurrentPrompt:  buildPrompt(validPrompt, 400),
		Instruction:    "Make the lighting colder.",
		ContextProfile: "auto",
	})
	if err != nil {
		t.Fatalf("Refine: %v", err)
	}
	if !result.Audit.OfficialFormatPass {
		t.Fatalf("audit = %+v", result.Audit)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	body := string(fake.requests[0])
	if !strings.Contains(body, "Make the lighting colder.") || !strings.Contains(body, "media is intentionally not attached") {
		t.Fatal("refinement request malformed")
	}
	if strings.Contains(body, `"images"`) {
		t.Fatal("refinement must not attach images")
	}
}
