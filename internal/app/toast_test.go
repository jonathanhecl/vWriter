package app

import (
	"testing"
	"time"
)

func TestPruneToastsExpiresNonErrors(t *testing.T) {
	a := &App{}
	a.pushToast("copied", "", false)
	a.pushToast("something failed", "", true)
	for len(a.toastClicks) < len(a.toasts) {
		a.toastClicks = append(a.toastClicks, toastClick{})
	}

	// Age only the success toast past the lifetime.
	a.toasts[0].created = time.Now().Add(-toastLifetime - time.Second)

	a.pruneToasts(time.Now())
	if len(a.toasts) != 1 {
		t.Fatalf("expected 1 toast after pruning, got %d", len(a.toasts))
	}
	if !a.toasts[0].isError || a.toasts[0].text != "something failed" {
		t.Fatalf("error toast must persist, got %+v", a.toasts[0])
	}
	if len(a.toastClicks) != 1 {
		t.Fatalf("toastClicks out of sync, got %d", len(a.toastClicks))
	}
}

func TestPruneToastsKeepsFresh(t *testing.T) {
	a := &App{}
	a.pushToast("fresh", "", false)
	a.pruneToasts(time.Now())
	if len(a.toasts) != 1 {
		t.Fatalf("fresh toast must not be pruned, got %d", len(a.toasts))
	}
}

func TestPruneToastsKeepsErrorsIndefinitely(t *testing.T) {
	a := &App{}
	a.pushToast("old error", "", true)
	a.toasts[0].created = time.Now().Add(-time.Hour)
	a.pruneToasts(time.Now())
	if len(a.toasts) != 1 || !a.toasts[0].isError {
		t.Fatalf("old error toast must persist, got %+v", a.toasts)
	}
}
