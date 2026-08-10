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

	if len(store.List()) != 0 {
		t.Fatalf("expected empty store")
	}

	p1, err := store.AddOrUpdate("Action Sequence", "Use video 1 as motion reference", 10, "16:9", "")
	if err != nil {
		t.Fatalf("failed to add preset: %v", err)
	}

	if p1.Name != "Action Sequence" {
		t.Errorf("unexpected name: %s", p1.Name)
	}

	if len(store.List()) != 1 {
		t.Fatalf("expected 1 preset")
	}

	// Reload from disk
	store2, err := NewPresetStore(path)
	if err != nil {
		t.Fatalf("failed to reload store: %v", err)
	}
	if len(store2.List()) != 1 {
		t.Fatalf("expected 1 preset after reload")
	}

	// Delete preset
	if err := store2.Delete(p1.ID); err != nil {
		t.Fatalf("failed to delete preset: %v", err)
	}
	if len(store2.List()) != 0 {
		t.Fatalf("expected 0 presets after delete")
	}
}
