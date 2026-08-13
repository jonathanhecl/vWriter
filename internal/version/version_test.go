package version

import "testing"

func TestStringVersionPrecedence(t *testing.T) {
	original := Version
	defer func() { Version = original }()

	Version = "v1.2.3"
	if got := String(); got != "v1.2.3" {
		t.Fatalf("release version must be shown, got %q", got)
	}

	Version = ""
	if got := String(); got == "" {
		t.Fatal("String must never be empty")
	}
}
