// Package config persists app settings in a config.json file placed next to
// the executable, portable-app style.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/jonathanhecl/vWriter/internal/ollama"
)

// Config holds the user-adjustable settings of the app.
type Config struct {
	OllamaURL            string  `json:"ollama_url"`
	Model                string  `json:"model"`
	ContextProfile       string  `json:"context_profile"` // "auto", "low", "standard", "extended"
	Thinking             bool    `json:"thinking"`
	KeepModelLoaded      bool    `json:"keep_model_loaded"`
	DurationSeconds      float64 `json:"duration_seconds"`
	AspectRatio          string  `json:"aspect_ratio"`
	SystemPromptOverride string  `json:"system_prompt_override"` // empty means default
}

// Default returns the out-of-the-box settings.
func Default() *Config {
	return &Config{
		OllamaURL:       ollama.DefaultURL,
		ContextProfile:  "auto",
		DurationSeconds: 10,
		AspectRatio:     "16:9",
	}
}

// DefaultPath locates config.json next to the running executable, falling
// back to the working directory when the executable path is unavailable.
func DefaultPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}

// Load reads path, returning the defaults when the file does not exist.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return nil, err
	}
	config := Default()
	if err := json.Unmarshal(raw, config); err != nil {
		return nil, err
	}
	return config, nil
}

// Save writes the config atomically: temp file, then rename.
func (c *Config) Save(path string) error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SystemPrompt returns the override pointer expected by the prompt package,
// or nil when the default is in use.
func (c *Config) SystemPrompt() *string {
	if c.SystemPromptOverride == "" {
		return nil
	}
	return &c.SystemPromptOverride
}
