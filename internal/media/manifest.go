package media

import (
	"fmt"
	"strings"
)

// Manifest summarizes the assets of a session for prompt assembly.
type Manifest struct {
	SessionID  string              `json:"session_id"`
	Assets     []*Asset            `json:"assets"`
	Counts     map[AssetType]int   `json:"counts"`
	Violations []ManifestViolation `json:"violations"`
	Valid      bool                `json:"valid"`
}

// ManifestViolation blocks generation until resolved.
type ManifestViolation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Manifest builds the session manifest with validity checks.
func (s *Store) Manifest(sessionID string) Manifest {
	s.mu.Lock()
	defer s.mu.Unlock()
	assets := append([]*Asset(nil), s.sessions[sessionID]...)
	manifest := Manifest{
		SessionID: sessionID,
		Assets:    assets,
		Counts:    map[AssetType]int{Image: 0, Video: 0, Audio: 0},
		Valid:     true,
	}
	types := map[AssetType]bool{}
	for _, asset := range assets {
		manifest.Counts[asset.Type]++
		types[asset.Type] = true
	}
	if len(types) == 1 && types[Audio] {
		manifest.Violations = append(manifest.Violations, ManifestViolation{
			Code:    "AUDIO_REQUIRES_VISUAL_REFERENCE",
			Message: "Reference audio must be accompanied by an image or video.",
		})
	}
	manifest.Valid = len(manifest.Violations) == 0
	return manifest
}

// ReferenceLine renders the manifest text line for one asset, mirroring the
// official reference notation used in the prompt.
func ReferenceLine(asset *Asset) string {
	detail := string(asset.Type)
	if asset.Duration > 0 {
		detail += fmt.Sprintf(", %gs", asset.Duration)
	}
	if asset.Type == Video && len(asset.Frames) > 0 {
		times := make([]string, len(asset.Frames))
		for index, frame := range asset.Frames {
			times[index] = fmt.Sprintf("%gs", frame.Timestamp)
		}
		detail += ", sampled frames at " + strings.Join(times, ", ")
	}
	if asset.Type == Audio {
		detail += ", not analyzed by the local model; role must come only from the user's brief"
	}
	return fmt.Sprintf("%s: %s (%s)", asset.Reference, asset.Filename, detail)
}
