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
	return referenceLine(asset, true)
}

// ReferenceLineSansAnchors renders a reference line without the first/last
// frame MUST markers. Continuation parts continue a previous part's video, so
// image anchors must not constrain them.
func ReferenceLineSansAnchors(asset *Asset) string {
	return referenceLine(asset, false)
}

func referenceLine(asset *Asset, withAnchors bool) string {
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
	line := fmt.Sprintf("%s: %s (%s)", asset.Reference, asset.Filename, detail)

	// Frame-anchor types are hard start/end constraints, not semantic roles.
	if withAnchors {
		switch asset.Role {
		case RoleFirstFrame:
			return line + " [first_frame: the output sequence MUST start with this exact image]"
		case RoleLastFrame:
			return line + " [last_frame: the output sequence MUST end with this exact image]"
		}
	}

	// Append user-assigned semantic context if provided.
	if asset.Role != "" {
		meta := "role: " + asset.Role
		if asset.Label != "" {
			meta += ", label: " + asset.Label
		}
		line += " [" + meta + "]"
	} else if asset.Label != "" {
		line += " [label: " + asset.Label + "]"
	}
	return line
}

// ReferenceLineWithLinked renders a ReferenceLine augmented with linked-asset info.
// assets is the full session list used to resolve LinkedAssetID.
func ReferenceLineWithLinked(asset *Asset, all []*Asset) string {
	return referenceLineWithLinked(asset, all, true)
}

// ReferenceLineWithLinkedSansAnchors is ReferenceLineWithLinked without the
// first/last frame MUST markers (for continuation requests).
func ReferenceLineWithLinkedSansAnchors(asset *Asset, all []*Asset) string {
	return referenceLineWithLinked(asset, all, false)
}

func referenceLineWithLinked(asset *Asset, all []*Asset, withAnchors bool) string {
	line := referenceLine(asset, withAnchors)
	if asset.LinkedAssetID != "" {
		for _, a := range all {
			if a.ID == asset.LinkedAssetID {
				line += " [voice for " + a.Reference + "]"
				break
			}
		}
	}
	return line
}
