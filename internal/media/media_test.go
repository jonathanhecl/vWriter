package media

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testStore(t *testing.T) (*Store, string) {
	t.Helper()
	return NewStore(t.TempDir()), "session-test"
}

// writeJPEG creates a real JPEG image on disk.
func writeJPEG(t *testing.T, dir, name string, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x % 255), G: uint8(y % 255), B: 128, A: 255})
		}
	}
	path := filepath.Join(dir, name)
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := jpeg.Encode(out, img, nil); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTypeOf(t *testing.T) {
	cases := map[string]AssetType{
		"photo.JPG": Image, "a.png": Image, "b.webp": Image, "c.tiff": Image,
		"clip.mp4": Video, "d.MOV": Video, "e.webm": Video,
		"song.mp3": Audio, "f.flac": Audio, "g.opus": Audio,
	}
	for name, want := range cases {
		got, ok := TypeOf(name)
		if !ok || got != want {
			t.Errorf("TypeOf(%q) = %v, %v; want %v", name, got, ok, want)
		}
	}
	if _, ok := TypeOf("notes.txt"); ok {
		t.Error("TypeOf(notes.txt) must be unsupported")
	}
}

func TestAddImageAndNumbering(t *testing.T) {
	store, session := testStore(t)
	dir := t.TempDir()
	first := writeJPEG(t, dir, "one.jpg", 2000, 1000)
	second := writeJPEG(t, dir, "two.jpg", 100, 100)

	asset, err := store.Add(session, first)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if asset.Reference != "<Picture 1>" || asset.Type != Image {
		t.Fatalf("asset = %+v", asset)
	}
	if asset.Width != 2000 || asset.Height != 1000 {
		t.Fatalf("dimensions = %dx%d", asset.Width, asset.Height)
	}
	// Prepared rendition must fit within 1536 and exist on disk.
	info, err := os.Stat(asset.PreparedPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("prepared missing: %v", err)
	}
	prepared, err := decodeImage(asset.PreparedPath)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Bounds().Dx() != 1536 || prepared.Bounds().Dy() != 768 {
		t.Fatalf("prepared size = %v", prepared.Bounds())
	}
	if _, err := os.Stat(asset.PreviewPath); err != nil {
		t.Fatalf("preview missing: %v", err)
	}

	if _, err := store.Add(session, second); err != nil {
		t.Fatalf("Add: %v", err)
	}
	assets := store.List(session)
	if assets[1].Reference != "<Picture 2>" {
		t.Fatalf("reference = %q", assets[1].Reference)
	}

	// Reorder swaps references.
	_, err = store.Reorder(session, []string{assets[1].ID, assets[0].ID})
	if err != nil {
		t.Fatalf("Reorder: %v", err)
	}
	assets = store.List(session)
	if assets[0].Reference != "<Picture 1>" || assets[0].Filename != "two.jpg" {
		t.Fatalf("after reorder: %+v", assets[0])
	}
}

func TestCapacityLimits(t *testing.T) {
	store, session := testStore(t)
	dir := t.TempDir()
	for i := 0; i < limitImages; i++ {
		if _, err := store.Add(session, writeJPEG(t, dir, fmt.Sprintf("img%d.jpg", i), 10, 10)); err != nil {
			t.Fatalf("Add %d: %v", i, err)
		}
	}
	err := func() error {
		_, err := store.Add(session, writeJPEG(t, dir, "overflow.jpg", 10, 10))
		return err
	}()
	if !Is(err, "MEDIA_LIMIT_REACHED") {
		t.Fatalf("err = %v, want MEDIA_LIMIT_REACHED", err)
	}
}

func TestUnsupportedType(t *testing.T) {
	store, session := testStore(t)
	path := filepath.Join(t.TempDir(), "notes.txt")
	os.WriteFile(path, []byte("hello"), 0o644)
	if err := func() error { _, err := store.Add(session, path); return err }(); !Is(err, "UNSUPPORTED_MEDIA") {
		t.Fatalf("err = %v, want UNSUPPORTED_MEDIA", err)
	}
}

func TestRemoveAndClear(t *testing.T) {
	store, session := testStore(t)
	dir := t.TempDir()
	first, _ := store.Add(session, writeJPEG(t, dir, "a.jpg", 10, 10))
	second, _ := store.Add(session, writeJPEG(t, dir, "b.jpg", 10, 10))

	artifactDir := filepath.Dir(first.PreparedPath)
	if err := store.Remove(session, first.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(artifactDir); !os.IsNotExist(err) {
		t.Error("artifact dir must be deleted")
	}
	if _, err := os.Stat(first.OriginalPath); err != nil {
		t.Error("original file must be kept")
	}
	assets := store.List(session)
	if len(assets) != 1 || assets[0].Reference != "<Picture 1>" || assets[0].ID != second.ID {
		t.Fatalf("after remove: %+v", assets)
	}

	store.Clear(session)
	if len(store.List(session)) != 0 {
		t.Error("Clear must empty the session")
	}
	if err := store.Remove(session, "missing"); !Is(err, "MEDIA_NOT_FOUND") {
		t.Fatalf("err = %v, want MEDIA_NOT_FOUND", err)
	}
}

func TestManifest(t *testing.T) {
	store, session := testStore(t)
	manifest := store.Manifest(session)
	if !manifest.Valid || len(manifest.Violations) != 0 {
		t.Fatalf("empty manifest = %+v", manifest)
	}
	dir := t.TempDir()
	store.Add(session, writeJPEG(t, dir, "a.jpg", 10, 10))
	manifest = store.Manifest(session)
	if manifest.Counts[Image] != 1 || !manifest.Valid {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestAudioOnlyManifestViolation(t *testing.T) {
	store, session := testStore(t)
	// Inject an audio asset directly to avoid the ffmpeg dependency.
	store.sessions[session] = []*Asset{{
		ID: "a1", Type: Audio, Filename: "voice.mp3", Duration: 5, AnalysisRequested: true,
	}}
	store.renumber(session)
	manifest := store.Manifest(session)
	if manifest.Valid || len(manifest.Violations) != 1 ||
		manifest.Violations[0].Code != "AUDIO_REQUIRES_VISUAL_REFERENCE" {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestSetAnalysis(t *testing.T) {
	store, session := testStore(t)
	dir := t.TempDir()
	asset, _ := store.Add(session, writeJPEG(t, dir, "a.jpg", 10, 10))
	asset, err := store.SetAnalysis(session, asset.ID, false)
	if err != nil || asset.AnalysisRequested {
		t.Fatalf("SetAnalysis: %+v, %v", asset, err)
	}
}

func TestFrameAnchorRoleExclusivity(t *testing.T) {
	store, session := testStore(t)
	dir := t.TempDir()
	a, _ := store.Add(session, writeJPEG(t, dir, "a.jpg", 10, 10))
	b, _ := store.Add(session, writeJPEG(t, dir, "b.jpg", 10, 10))

	if _, err := store.SetRole(session, a.ID, RoleFirstFrame, "", ""); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	// A new first_frame assignment must clear the previous one.
	if _, err := store.SetRole(session, b.ID, RoleFirstFrame, "", ""); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	assets := store.List(session)
	if assets[0].Role != "" || assets[1].Role != RoleFirstFrame {
		t.Fatalf("exclusivity violated: %+v", assets)
	}
	// first_frame and last_frame can coexist on different images.
	if _, err := store.SetRole(session, a.ID, RoleLastFrame, "", ""); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	assets = store.List(session)
	if assets[0].Role != RoleLastFrame || assets[1].Role != RoleFirstFrame {
		t.Fatalf("both anchors expected: %+v", assets)
	}
	// Clearing works.
	if _, err := store.SetRole(session, a.ID, "", "", ""); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	// A non-anchor role on one image must not clear another anchor.
	if _, err := store.SetRole(session, a.ID, "scene", "", ""); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	assets = store.List(session)
	if assets[0].Role != "scene" || assets[1].Role != RoleFirstFrame {
		t.Fatalf("scene role must not clear the first_frame anchor: %+v", assets)
	}
}

func TestResampleValidation(t *testing.T) {
	store, session := testStore(t)
	dir := t.TempDir()
	imageAsset, _ := store.Add(session, writeJPEG(t, dir, "a.jpg", 10, 10))
	if _, err := store.Resample(session, imageAsset.ID, "", nil); !Is(err, "UNSUPPORTED_MEDIA") {
		t.Fatalf("err = %v, want UNSUPPORTED_MEDIA", err)
	}
}

func TestClipDurationRule(t *testing.T) {
	for _, duration := range []float64{0, 1.9, 15.2, 30} {
		asset := &Asset{Type: Video, Duration: duration, Filename: "clip.mp4"}
		if err := validateClipDuration(asset); !Is(err, "UNSUPPORTED_DURATION") {
			t.Errorf("duration %v: err = %v, want UNSUPPORTED_DURATION", duration, err)
		}
	}
	for _, duration := range []float64{2, 10, 15.1} {
		asset := &Asset{Type: Audio, Duration: duration, Filename: "a.mp3"}
		if err := validateClipDuration(asset); err != nil {
			t.Errorf("duration %v: %v", duration, err)
		}
	}
	if err := validateClipDuration(&Asset{Type: Image}); err != nil {
		t.Errorf("images are exempt: %v", err)
	}
}

func TestReferenceLine(t *testing.T) {
	imageAsset := &Asset{Type: Image, Reference: "<Picture 1>", Filename: "a.jpg"}
	if got := ReferenceLine(imageAsset); got != "<Picture 1>: a.jpg (image)" {
		t.Errorf("got %q", got)
	}
	videoAsset := &Asset{
		Type: Video, Reference: "<Video 1>", Filename: "clip.mp4", Duration: 8,
		Frames: []Frame{{Timestamp: 0.24}, {Timestamp: 7.76}},
	}
	got := ReferenceLine(videoAsset)
	if !strings.Contains(got, "video, 8s") || !strings.Contains(got, "sampled frames at 0.24s, 7.76s") {
		t.Errorf("got %q", got)
	}
	audioAsset := &Asset{Type: Audio, Reference: "<Audio 1>", Filename: "song.mp3", Duration: 12}
	got = ReferenceLine(audioAsset)
	if !strings.Contains(got, "not analyzed by the local model") {
		t.Errorf("got %q", got)
	}
	firstAsset := &Asset{Type: Image, Reference: "<Picture 1>", Filename: "open.jpg", Role: RoleFirstFrame}
	got = ReferenceLine(firstAsset)
	if !strings.Contains(got, "MUST start with this exact image") {
		t.Errorf("got %q", got)
	}
	lastAsset := &Asset{Type: Image, Reference: "<Picture 2>", Filename: "close.jpg", Role: RoleLastFrame}
	got = ReferenceLine(lastAsset)
	if !strings.Contains(got, "MUST end with this exact image") {
		t.Errorf("got %q", got)
	}
}
