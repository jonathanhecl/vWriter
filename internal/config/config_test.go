package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefaults(t *testing.T) {
	config, err := Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if config.OllamaURL == "" || config.ContextProfile != "auto" || config.DurationSeconds != 10 {
		t.Fatalf("defaults = %+v", config)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	config := Default()
	config.OllamaURL = "http://192.168.1.50:11434"
	config.Model = "gemma3:12b"
	config.Thinking = true
	config.SystemPromptOverride = "Custom."
	if err := config.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file must be renamed away")
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.OllamaURL != config.OllamaURL || loaded.Model != config.Model || !loaded.Thinking {
		t.Fatalf("loaded = %+v", loaded)
	}
	if loaded.SystemPrompt() == nil || *loaded.SystemPrompt() != "Custom." {
		t.Fatal("system prompt override lost")
	}
	def := Default()
	if def.SystemPrompt() != nil {
		t.Fatal("default config must use the default system prompt")
	}
}

func TestLoadPartialKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte(`{"model":"gemma3:4b"}`), 0o644)
	config, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if config.Model != "gemma3:4b" || config.OllamaURL == "" {
		t.Fatalf("config = %+v", config)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte(`{broken`), 0o644)
	if _, err := Load(path); err == nil {
		t.Fatal("invalid JSON must fail")
	}
}
