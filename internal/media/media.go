// Package media manages reference assets (images, videos, audio files),
// their on-disk derived artifacts, and the rules of full-reference mode.
package media

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Asset kinds.
const (
	Image AssetType = "image"
	Video AssetType = "video"
	Audio AssetType = "audio"
)

type AssetType string

// Frame-anchor types. When an image's Role is one of these, the model must
// treat the image as the required first or last frame of the output sequence.
const (
	RoleFirstFrame = "first_frame"
	RoleLastFrame  = "last_frame"
)

var (
	imageExtensions = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".bmp": true, ".tif": true, ".tiff": true}
	videoExtensions = map[string]bool{".mp4": true, ".mov": true, ".mkv": true, ".webm": true, ".avi": true, ".m4v": true}
	audioExtensions = map[string]bool{".wav": true, ".mp3": true, ".flac": true, ".m4a": true, ".ogg": true, ".aac": true, ".opus": true}
)

// MaxFileBytes caps one accepted media file.
const MaxFileBytes = 1024 * 1024 * 1024

// Error is a stable, user-facing media failure.
type Error struct {
	Code    string
	Message string
	Details any
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// Is reports whether err is a media Error, optionally with a specific code.
func Is(err error, code string) bool {
	var merr *Error
	return errors.As(err, &merr) && (code == "" || merr.Code == code)
}

// Frame is one sampled video frame.
type Frame struct {
	Timestamp float64 `json:"timestamp"`
	Path      string  `json:"path"`
}

// Asset describes one reference file and its derived artifacts.
type Asset struct {
	ID                string    `json:"id"`
	Type              AssetType `json:"type"`
	Filename          string    `json:"filename"`
	Size              int64     `json:"size"`
	Reference         string    `json:"reference"`
	AnalysisRequested bool      `json:"analysis_requested"`
	Width             int       `json:"width,omitempty"`
	Height            int       `json:"height,omitempty"`
	Duration          float64   `json:"duration,omitempty"`
	HasAudio          bool      `json:"has_audio,omitempty"`

	// Video sampling settings.
	FrameCountMode   string  `json:"frame_count_mode,omitempty"` // "auto", "6", or "8"
	FrameCount       int     `json:"frame_count,omitempty"`
	IncludeEndpoints bool    `json:"include_endpoints,omitempty"`
	SampleIndex      int     `json:"sample_index,omitempty"`
	Frames           []Frame `json:"frames,omitempty"`

	// User-assigned semantic metadata (optional; not required for generation).
	Role          string `json:"role,omitempty"`            // e.g. "person", "scene", "music", "voice", "first_frame", "last_frame"
	Label         string `json:"label,omitempty"`           // e.g. "John", "office background"
	LinkedAssetID string `json:"linked_asset_id,omitempty"` // for audio voice: the picture asset ID it belongs to

	// On-disk locations. OriginalPath is the user's file and is never deleted.
	OriginalPath     string `json:"original_path"`
	PreparedPath     string `json:"prepared_path,omitempty"`
	PreviewPath      string `json:"preview_path,omitempty"`
	ContactSheetPath string `json:"contact_sheet_path,omitempty"`
	artifactDir      string
}

// TypeOf classifies a file by its extension.
func TypeOf(filename string) (AssetType, bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch {
	case imageExtensions[ext]:
		return Image, true
	case videoExtensions[ext]:
		return Video, true
	case audioExtensions[ext]:
		return Audio, true
	}
	return "", false
}

// DefaultRoot is the cache root for derived artifacts.
func DefaultRoot() string {
	return filepath.Join(os.TempDir(), "vwriter")
}

// Store holds the media sessions of the running app.
type Store struct {
	mu       sync.Mutex
	root     string
	sessions map[string][]*Asset
}

// NewStore creates a store rooted at root (os temp dir based if empty).
func NewStore(root string) *Store {
	if root == "" {
		root = DefaultRoot()
	}
	return &Store{root: root, sessions: map[string][]*Asset{}}
}

// List returns the assets of a session in order.
func (s *Store) List(sessionID string) []*Asset {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*Asset(nil), s.sessions[sessionID]...)
}

// Get returns one asset or MEDIA_NOT_FOUND.
func (s *Store) Get(sessionID, assetID string) (*Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, asset := range s.sessions[sessionID] {
		if asset.ID == assetID {
			return asset, nil
		}
	}
	return nil, &Error{Code: "MEDIA_NOT_FOUND", Message: "The media asset was not found in this session."}
}

// Add registers the file at sourcePath as a reference asset. The original
// file is never modified or deleted; derived artifacts live under the store
// root.
func (s *Store) Add(sessionID, sourcePath string) (*Asset, error) {
	kind, ok := TypeOf(sourcePath)
	if !ok {
		return nil, &Error{Code: "UNSUPPORTED_MEDIA", Message: "This file type is not supported."}
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, &Error{Code: "MEDIA_NOT_FOUND", Message: "The file could not be read.", Details: err.Error()}
	}
	if info.Size() > MaxFileBytes {
		return nil, &Error{Code: "MEDIA_TOO_LARGE", Message: "A media file cannot exceed 1 GB."}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	assets := s.sessions[sessionID]
	if err := validateCapacity(assets, kind); err != nil {
		return nil, err
	}

	asset := &Asset{
		ID:                newID(),
		Type:              kind,
		Filename:          filepath.Base(sourcePath),
		Size:              info.Size(),
		AnalysisRequested: true,
		OriginalPath:      sourcePath,
		artifactDir:       filepath.Join(s.root, sessionID, newID()),
	}
	if err := os.MkdirAll(asset.artifactDir, 0o755); err != nil {
		return nil, err
	}
	var processErr error
	switch kind {
	case Image:
		processErr = processImage(asset)
	case Video:
		asset.FrameCountMode = "auto"
		asset.IncludeEndpoints = true
		processErr = processVideo(asset)
	case Audio:
		processErr = processAudio(asset)
	}
	if processErr != nil {
		os.RemoveAll(asset.artifactDir)
		return nil, processErr
	}
	if err := validateClipDuration(asset); err != nil {
		os.RemoveAll(asset.artifactDir)
		return nil, err
	}
	s.sessions[sessionID] = append(assets, asset)
	s.renumber(sessionID)
	return asset, nil
}

// Remove deletes one asset's derived artifacts. The original file is kept.
func (s *Store) Remove(sessionID, assetID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	assets := s.sessions[sessionID]
	for index, asset := range assets {
		if asset.ID == assetID {
			s.sessions[sessionID] = append(assets[:index], assets[index+1:]...)
			os.RemoveAll(asset.artifactDir)
			s.renumber(sessionID)
			return nil
		}
	}
	return &Error{Code: "MEDIA_NOT_FOUND", Message: "The media asset was not found in this session."}
}

// Clear drops a whole session and its artifacts.
func (s *Store) Clear(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, asset := range s.sessions[sessionID] {
		os.RemoveAll(asset.artifactDir)
	}
	delete(s.sessions, sessionID)
	os.RemoveAll(filepath.Join(s.root, sessionID))
}

// SetRole sets the semantic role, label, and optional linked asset on an asset.
// Frame-anchor roles (first_frame, last_frame) are exclusive: assigning one
// clears the same role from the previous image, so the sequence has at most
// one required first frame and one required last frame.
func (s *Store) SetRole(sessionID, assetID, role, label, linkedAssetID string) (*Asset, error) {
	asset, err := s.Get(sessionID, assetID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if role == RoleFirstFrame || role == RoleLastFrame {
		for _, other := range s.sessions[sessionID] {
			if other.ID != assetID && other.Role == role {
				other.Role = ""
			}
		}
	}
	asset.Role = role
	asset.Label = label
	asset.LinkedAssetID = linkedAssetID
	s.mu.Unlock()
	return asset, nil
}

// SetAnalysis toggles whether an asset is sent to the model for analysis.
func (s *Store) SetAnalysis(sessionID, assetID string, enabled bool) (*Asset, error) {
	asset, err := s.Get(sessionID, assetID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	asset.AnalysisRequested = enabled
	s.mu.Unlock()
	return asset, nil
}

// Reorder sets the asset order of a session; renumbers references.
func (s *Store) Reorder(sessionID string, orderedIDs []string) ([]*Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	assets := s.sessions[sessionID]
	if len(orderedIDs) != len(assets) {
		return nil, &Error{Code: "INVALID_MEDIA_ORDER", Message: "The media order does not match the session assets."}
	}
	byID := make(map[string]*Asset, len(assets))
	for _, asset := range assets {
		byID[asset.ID] = asset
	}
	ordered := make([]*Asset, 0, len(assets))
	for _, id := range orderedIDs {
		asset, ok := byID[id]
		if !ok {
			return nil, &Error{Code: "INVALID_MEDIA_ORDER", Message: "The media order does not match the session assets."}
		}
		ordered = append(ordered, asset)
	}
	s.sessions[sessionID] = ordered
	s.renumber(sessionID)
	return ordered, nil
}

// renumber assigns <Picture N>, <Video N>, and <Audio N> references per type.
// Callers must hold the lock.
func (s *Store) renumber(sessionID string) {
	names := map[AssetType]string{Image: "Picture", Video: "Video", Audio: "Audio"}
	counters := map[AssetType]int{}
	for _, asset := range s.sessions[sessionID] {
		counters[asset.Type]++
		asset.Reference = fmt.Sprintf("<%s %d>", names[asset.Type], counters[asset.Type])
	}
}

func newID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(raw[:])
}
