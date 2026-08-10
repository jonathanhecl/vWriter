package media

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
)

// ffmpegAvailable reports whether both ffmpeg and ffprobe are in PATH.
func ffmpegAvailable() bool {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return false
	}
	_, err := exec.LookPath("ffprobe")
	return err == nil
}

func ffmpegMissingError() *Error {
	return &Error{
		Code:    "FFMPEG_MISSING",
		Message: "ffmpeg and ffprobe must be installed and available in PATH to process video and audio files.",
	}
}

// probeResult is the parsed subset of ffprobe's JSON output.
type probeResult struct {
	Duration float64
	Width    int
	Height   int
	HasAudio bool
}

// probe reads media metadata with ffprobe.
func probe(path string) (probeResult, error) {
	if !ffmpegAvailable() {
		return probeResult{}, ffmpegMissingError()
	}
	cmd := exec.Command("ffprobe",
		"-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return probeResult{}, &Error{
			Code:    "MEDIA_DECODE_FAILED",
			Message: fmt.Sprintf("Could not probe the media file: %v", err),
			Details: stderr.String(),
		}
	}
	var parsed struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return probeResult{}, &Error{Code: "MEDIA_DECODE_FAILED", Message: "Could not parse ffprobe output.", Details: err.Error()}
	}
	result := probeResult{}
	if parsed.Format.Duration != "" {
		result.Duration, _ = strconv.ParseFloat(parsed.Format.Duration, 64)
	}
	for _, stream := range parsed.Streams {
		switch stream.CodecType {
		case "video":
			if result.Width == 0 {
				result.Width, result.Height = stream.Width, stream.Height
			}
		case "audio":
			result.HasAudio = true
		}
	}
	if result.Duration <= 0 {
		return probeResult{}, &Error{Code: "MEDIA_DECODE_FAILED", Message: "The media duration could not be determined."}
	}
	return result, nil
}

// grabFrame extracts the frame nearest to timestamp seconds, scaled so the
// long edge is at most frameMaxEdge pixels, and saves it as JPEG.
func grabFrame(source string, timestamp float64, dest string) error {
	if !ffmpegAvailable() {
		return ffmpegMissingError()
	}
	cmd := exec.Command("ffmpeg",
		"-y", "-v", "error",
		"-ss", fmt.Sprintf("%.3f", timestamp),
		"-i", source,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale='min(%[1]d,iw)':-2", frameMaxEdge),
		"-q:v", "3",
		dest)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return &Error{
			Code:    "MEDIA_DECODE_FAILED",
			Message: fmt.Sprintf("Could not sample a video frame: %v", err),
			Details: stderr.String(),
		}
	}
	return nil
}

// processAudio fills duration metadata for an audio asset. Audio content is
// never analyzed; only its declared role reaches the prompt.
func processAudio(asset *Asset) error {
	meta, err := probe(asset.OriginalPath)
	if err != nil {
		return err
	}
	asset.Duration = meta.Duration
	return nil
}

// processVideo probes a video asset, samples frames, and builds the contact
// sheet the model receives as the visual representation of the video.
func processVideo(asset *Asset) error {
	meta, err := probe(asset.OriginalPath)
	if err != nil {
		return err
	}
	asset.Duration = meta.Duration
	asset.Width, asset.Height = meta.Width, meta.Height
	asset.HasAudio = meta.HasAudio

	count := 8
	if asset.FrameCountMode != "auto" {
		parsed, err := strconv.Atoi(asset.FrameCountMode)
		if err != nil || (parsed != 6 && parsed != 8) {
			return &Error{Code: "INVALID_SAMPLE_COUNT", Message: "Frame count must be Auto, 6, or 8."}
		}
		count = parsed
	}
	asset.FrameCount = count
	timestamps := sampleTimestamps(asset.Duration, count, asset.IncludeEndpoints, asset.SampleIndex)

	asset.Frames = nil
	for index, timestamp := range timestamps {
		framePath := filepath.Join(asset.artifactDir, fmt.Sprintf("frame_%02d.jpg", index))
		if err := grabFrame(asset.OriginalPath, timestamp, framePath); err != nil {
			return err
		}
		asset.Frames = append(asset.Frames, Frame{Timestamp: timestamp, Path: framePath})
	}
	if len(asset.Frames) == 0 {
		return &Error{Code: "MEDIA_DECODE_FAILED", Message: "No video frames could be sampled."}
	}
	asset.ContactSheetPath = filepath.Join(asset.artifactDir, "contact_sheet.jpg")
	if err := buildContactSheet(asset.Frames, asset.ContactSheetPath); err != nil {
		return err
	}
	asset.PreviewPath = asset.Frames[0].Path
	return nil
}
