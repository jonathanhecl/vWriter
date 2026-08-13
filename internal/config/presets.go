package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PresetAsset represents a reference asset stored within a preset.
type PresetAsset struct {
	Type                string `json:"type"` // "image", "video", "audio"
	Path                string `json:"path"` // file path
	Filename            string `json:"filename"`
	Role                string `json:"role,omitempty"`                  // e.g. "person", "scene", "voice", "first_frame", "last_frame"
	Label               string `json:"label,omitempty"`                 // e.g. "John"
	LinkedAssetFilename string `json:"linked_asset_filename,omitempty"` // for audio voice: filename of the linked image
}

// PresetPart is one prompt of a multi-part story stored in a preset.
type PresetPart struct {
	Prompt  string   `json:"prompt"`
	Brief   string   `json:"brief,omitempty"`   // the idea the user wrote for this part
	Refines []string `json:"refines,omitempty"` // revision instructions applied to this part
}

// Preset represents a saved creative prompt template.
type Preset struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Brief        string        `json:"brief"`
	Duration     int           `json:"duration"`
	AspectRatio  string        `json:"aspect_ratio"`
	SystemPrompt string        `json:"system_prompt,omitempty"`
	Output       string        `json:"output,omitempty"`
	Parts        []PresetPart  `json:"parts,omitempty"`
	Assets       []PresetAsset `json:"assets,omitempty"`
	CreatedAt    string        `json:"created_at"`
}

// PresetStore manages saving and loading creative templates in presets.json.
type PresetStore struct {
	mu      sync.Mutex
	path    string
	Presets []*Preset `json:"presets"`
}

// DefaultPresetsPath locates presets.json next to the running executable.
func DefaultPresetsPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "presets.json"
	}
	return filepath.Join(filepath.Dir(exe), "presets.json")
}

// NewPresetStore creates or loads a PresetStore from path.
func NewPresetStore(path string) (*PresetStore, error) {
	if path == "" {
		path = DefaultPresetsPath()
	}
	store := &PresetStore{path: path, Presets: []*Preset{}}
	_ = store.Load()
	store.EnsureDefault()
	return store, nil
}

// EnsureDefault makes sure a DEFAULT preset exists.
func (s *PresetStore) EnsureDefault() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, p := range s.Presets {
		if p.Name == "DEFAULT" {
			return
		}
	}
	defaultPreset := &Preset{
		ID:          newPresetID(),
		Name:        "DEFAULT",
		Duration:    10,
		AspectRatio: "16:9",
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
	s.Presets = append([]*Preset{defaultPreset}, s.Presets...)
	_ = s.saveLocked()
}

// Load reads presets.json into memory.
func (s *PresetStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.Presets = []*Preset{}
		return nil
	}
	if err != nil {
		return err
	}
	var data struct {
		Presets []*Preset `json:"presets"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	s.Presets = data.Presets
	return nil
}

// Save writes the current presets to disk.
func (s *PresetStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveLocked()
}

func (s *PresetStore) saveLocked() error {
	raw, err := json.MarshalIndent(struct {
		Presets []*Preset `json:"presets"`
	}{Presets: s.Presets}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}

// List returns a copy of all saved presets.
func (s *PresetStore) List() []*Preset {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]*Preset(nil), s.Presets...)
}

// AddOrUpdate saves a preset. If a preset with the same Name exists, it updates it.
func (s *PresetStore) AddOrUpdate(name, brief string, duration int, aspectRatio, systemPrompt, output string, assets []PresetAsset, parts []PresetPart) (*Preset, error) {
	if name == "" {
		name = "Untitled Template"
	}
	p := &Preset{
		ID:           newPresetID(),
		Name:         name,
		Brief:        brief,
		Duration:     duration,
		AspectRatio:  aspectRatio,
		SystemPrompt: systemPrompt,
		Output:       output,
		Parts:        parts,
		Assets:       assets,
		CreatedAt:    time.Now().Format(time.RFC3339),
	}

	s.mu.Lock()
	found := false
	for i, existing := range s.Presets {
		if existing.Name == name {
			p.ID = existing.ID
			s.Presets[i] = p
			found = true
			break
		}
	}
	if !found {
		s.Presets = append(s.Presets, p)
	}
	s.mu.Unlock()

	if err := s.Save(); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete removes a preset by ID or Name.
func (s *PresetStore) Delete(idOrName string) error {
	if idOrName == "DEFAULT" {
		return errors.New("cannot delete DEFAULT preset")
	}
	s.mu.Lock()
	updated := []*Preset{}
	for _, p := range s.Presets {
		if p.ID != idOrName && p.Name != idOrName {
			updated = append(updated, p)
		}
	}
	s.Presets = updated
	s.mu.Unlock()

	return s.Save()
}

func newPresetID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
