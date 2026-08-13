package config

import (
	"path/filepath"
	"testing"
)

func TestPresetStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "presets.json")

	store, err := NewPresetStore(path)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if len(store.List()) != 1 || store.List()[0].Name != "DEFAULT" {
		t.Fatalf("expected DEFAULT preset in new store")
	}

	p1, err := store.AddOrUpdate("Action Sequence", "Use video 1 as motion reference", 10, "16:9", "", "", nil, []PresetPart{{Prompt: "part one", Brief: "opening", Refine: "make it colder"}})
	if err != nil {
		t.Fatalf("failed to add preset: %v", err)
	}

	if p1.Name != "Action Sequence" {
		t.Errorf("unexpected name: %s", p1.Name)
	}

	if len(p1.Parts) != 1 || p1.Parts[0].Prompt != "part one" || p1.Parts[0].Brief != "opening" {
		t.Errorf("parts not persisted: %+v", p1.Parts)
	}
	if p1.Parts[0].Refine != "make it colder" {
		t.Errorf("part refine not persisted: %+v", p1.Parts[0].Refine)
	}

	if len(store.List()) != 2 {
		t.Fatalf("expected 2 presets")
	}

	// Reload from disk
	store2, err := NewPresetStore(path)
	if err != nil {
		t.Fatalf("failed to reload store: %v", err)
	}
	if len(store2.List()) != 2 {
		t.Fatalf("expected 2 presets after reload")
	}

	// Delete preset
	if err := store2.Delete(p1.ID); err != nil {
		t.Fatalf("failed to delete preset: %v", err)
	}
	if len(store2.List()) != 1 {
		t.Fatalf("expected 1 preset (DEFAULT) after delete")
	}
}
