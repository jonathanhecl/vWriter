package media

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// requireFFmpeg skips the test when ffmpeg/ffprobe are not installed.
func requireFFmpeg(t *testing.T) {
	t.Helper()
	if !ffmpegAvailable() {
		t.Skip("ffmpeg/ffprobe not found in PATH")
	}
}

// makeTestVideo renders a 5-second synthetic clip with ffmpeg.
func makeTestVideo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "clip.mp4")
	cmd := exec.Command("ffmpeg", "-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc=duration=5:size=640x360:rate=24",
		"-pix_fmt", "yuv420p", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg testsrc: %v\n%s", err, out)
	}
	return path
}

// makeTestAudio renders a 3-second sine wave with ffmpeg.
func makeTestAudio(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "tone.wav")
	cmd := exec.Command("ffmpeg", "-y", "-v", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg sine: %v\n%s", err, out)
	}
	return path
}

func TestVideoPipeline(t *testing.T) {
	requireFFmpeg(t)
	store, session := testStore(t)
	dir := t.TempDir()

	asset, err := store.Add(session, makeTestVideo(t, dir))
	if err != nil {
		t.Fatalf("Add video: %v", err)
	}
	if asset.Reference != "<Video 1>" {
		t.Fatalf("reference = %q", asset.Reference)
	}
	if asset.Duration < 4.9 || asset.Duration > 5.1 {
		t.Fatalf("duration = %v", asset.Duration)
	}
	if asset.Width != 640 || asset.Height != 360 {
		t.Fatalf("dimensions = %dx%d", asset.Width, asset.Height)
	}
	if len(asset.Frames) != 8 {
		t.Fatalf("auto sampling must produce 8 frames, got %d", len(asset.Frames))
	}
	for _, frame := range asset.Frames {
		info, err := os.Stat(frame.Path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("frame missing: %v", frame)
		}
	}
	if info, err := os.Stat(asset.ContactSheetPath); err != nil || info.Size() == 0 {
		t.Fatalf("contact sheet missing: %v", err)
	}
	// The contact sheet is a valid image the UI can display.
	sheet, err := decodeImage(asset.ContactSheetPath)
	if err != nil {
		t.Fatalf("decode contact sheet: %v", err)
	}
	if sheet.Bounds().Dx() != 4*sheetCellWidth {
		t.Fatalf("sheet width = %d", sheet.Bounds().Dx())
	}

	// Resample with 6 frames, no endpoints.
	endpoints := false
	resampled, err := store.Resample(session, asset.ID, "6", &endpoints)
	if err != nil {
		t.Fatalf("Resample: %v", err)
	}
	if len(resampled.Frames) != 6 || resampled.SampleIndex != 0 {
		t.Fatalf("resampled = %+v", resampled)
	}
	// Same settings again advance the rotation and shift interior moments.
	firstPass := resampled.Frames[1].Timestamp
	again, err := store.Resample(session, asset.ID, "6", &endpoints)
	if err != nil {
		t.Fatalf("Resample again: %v", err)
	}
	if again.SampleIndex != 1 || again.Frames[1].Timestamp == firstPass {
		t.Fatalf("rotation did not advance: %+v", again)
	}
}

func TestAudioPipeline(t *testing.T) {
	requireFFmpeg(t)
	store, session := testStore(t)
	dir := t.TempDir()

	asset, err := store.Add(session, makeTestAudio(t, dir))
	if err != nil {
		t.Fatalf("Add audio: %v", err)
	}
	if asset.Reference != "<Audio 1>" || asset.Duration < 2.9 || asset.Duration > 3.1 {
		t.Fatalf("asset = %+v", asset)
	}
	// Audio alone is not a valid reference manifest.
	if manifest := store.Manifest(session); manifest.Valid {
		t.Fatal("audio-only manifest must be invalid")
	}
}

func TestLongVideoRejected(t *testing.T) {
	requireFFmpeg(t)
	store, session := testStore(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "long.mp4")
	cmd := exec.Command("ffmpeg", "-y", "-v", "error",
		"-f", "lavfi", "-i", "testsrc=duration=16:size=320x180:rate=8",
		"-pix_fmt", "yuv420p", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg testsrc: %v\n%s", err, out)
	}
	if _, err := store.Add(session, path); !Is(err, "UNSUPPORTED_DURATION") {
		t.Fatalf("err = %v, want UNSUPPORTED_DURATION", err)
	}
	if len(store.List(session)) != 0 {
		t.Fatal("rejected assets must not stay in the session")
	}
}
