package media

import "fmt"

const (
	// ReferenceDurationToleranceSeconds bounds reference video/audio clips.
	ReferenceDurationToleranceSeconds = 15.1
	minClipDurationSeconds            = 2.0
	limitImages                       = 9
	limitVideos                       = 3
	limitAudios                       = 3
	limitTotal                        = 12
)

func validateCapacity(assets []*Asset, kind AssetType) error {
	limits := map[AssetType]int{Image: limitImages, Video: limitVideos, Audio: limitAudios}
	count := 0
	for _, asset := range assets {
		if asset.Type == kind {
			count++
		}
	}
	if count >= limits[kind] {
		return &Error{
			Code:    "MEDIA_LIMIT_REACHED",
			Message: fmt.Sprintf("Reference mode accepts at most %d %s files.", limits[kind], kind),
		}
	}
	if len(assets) >= limitTotal {
		return &Error{Code: "MEDIA_LIMIT_REACHED", Message: "Reference mode accepts at most 12 files in total."}
	}
	return nil
}

func validateClipDuration(asset *Asset) error {
	if asset.Type != Video && asset.Type != Audio {
		return nil
	}
	if asset.Duration < minClipDurationSeconds || asset.Duration > ReferenceDurationToleranceSeconds {
		return &Error{
			Code:    "UNSUPPORTED_DURATION",
			Message: "Reference video and audio clips must be 2–15 seconds long.",
			Details: map[string]any{"duration": asset.Duration, "file": asset.Filename},
		}
	}
	return nil
}
