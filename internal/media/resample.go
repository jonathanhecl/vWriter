package media

import "os"

// Resample regenerates a video asset's frames and contact sheet. An empty
// frameCountMode or a nil includeEndpoints keeps the current setting. Changed
// settings reset the sample rotation; unchanged settings advance it, so a
// repeated resample observes different moments of the same video.
func (s *Store) Resample(sessionID, assetID, frameCountMode string, includeEndpoints *bool) (*Asset, error) {
	asset, err := s.Get(sessionID, assetID)
	if err != nil {
		return nil, err
	}
	if asset.Type != Video {
		return nil, &Error{Code: "UNSUPPORTED_MEDIA", Message: "Only video assets can be resampled."}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	selectedCount := asset.FrameCountMode
	if frameCountMode != "" {
		selectedCount = frameCountMode
	}
	if selectedCount != "auto" && selectedCount != "6" && selectedCount != "8" {
		return nil, &Error{Code: "INVALID_SAMPLE_COUNT", Message: "Frame count must be Auto, 6, or 8."}
	}
	selectedEndpoints := asset.IncludeEndpoints
	if includeEndpoints != nil {
		selectedEndpoints = *includeEndpoints
	}
	settingsChanged := selectedCount != asset.FrameCountMode || selectedEndpoints != asset.IncludeEndpoints
	if settingsChanged {
		asset.SampleIndex = 0
	} else {
		asset.SampleIndex++
	}
	asset.FrameCountMode = selectedCount
	asset.IncludeEndpoints = selectedEndpoints

	for _, frame := range asset.Frames {
		os.Remove(frame.Path)
	}
	if asset.ContactSheetPath != "" {
		os.Remove(asset.ContactSheetPath)
	}
	if err := processVideo(asset); err != nil {
		return nil, err
	}
	return asset, nil
}
