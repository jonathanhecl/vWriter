package media

import (
	"math"
	"testing"
)

func TestSampleTimestampsWithEndpoints(t *testing.T) {
	times := sampleTimestamps(10.0, 8, true, 0)
	if len(times) != 8 {
		t.Fatalf("len = %d", len(times))
	}
	// margin = min(0.25, 0.3) = 0.25; endpoints land exactly on the margins.
	if math.Abs(times[0]-0.25) > 0.001 || math.Abs(times[7]-9.75) > 0.001 {
		t.Fatalf("endpoints = %v, %v", times[0], times[7])
	}
	for index := 1; index < len(times); index++ {
		if times[index] <= times[index-1] {
			t.Fatalf("not strictly increasing: %v", times)
		}
	}
}

func TestSampleTimestampsWithoutEndpoints(t *testing.T) {
	times := sampleTimestamps(10.0, 6, false, 0)
	if len(times) != 6 {
		t.Fatalf("len = %d", len(times))
	}
	if times[0] <= 0.25 || times[5] >= 9.75 {
		t.Fatalf("should stay inside the margins: %v", times)
	}
}

func TestSampleRotationChangesMoments(t *testing.T) {
	base := sampleTimestamps(10.0, 8, true, 0)
	rotated := sampleTimestamps(10.0, 8, true, 1)
	same := true
	for index := range base {
		if base[index] != rotated[index] {
			same = false
		}
	}
	if same {
		t.Fatal("a rotated sample must pick different interior moments")
	}
	// Endpoints stay fixed regardless of rotation.
	if base[0] != rotated[0] || base[7] != rotated[7] {
		t.Fatal("endpoints must not move")
	}
}

func TestShortVideoMargin(t *testing.T) {
	// duration*0.03 < 0.25: margin shrinks proportionally.
	times := sampleTimestamps(2.0, 6, true, 0)
	if math.Abs(times[0]-0.06) > 0.001 {
		t.Fatalf("first = %v, want 0.06", times[0])
	}
}
