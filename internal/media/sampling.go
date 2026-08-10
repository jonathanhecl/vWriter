package media

import "math"

// frameMaxEdge caps the long edge of one sampled video frame.
const frameMaxEdge = 768

// sampleOffsets rotates a slight bias through successive re-samples so each
// resample observes different moments of the same video.
var sampleOffsets = []float64{0.0, -0.22, 0.22, -0.11, 0.11}

// sampleTimestamps computes the ordered, uniformly spaced sampling moments of
// a video. Endpoints are exact when included; interior positions receive a
// small rotating offset per sampleIndex.
func sampleTimestamps(duration float64, count int, includeEndpoints bool, sampleIndex int) []float64 {
	margin := math.Min(0.25, duration*0.03)
	span := duration - 2*margin
	offset := sampleOffsets[sampleIndex%len(sampleOffsets)]
	clamp := func(value float64) float64 {
		return math.Min(0.999, math.Max(0.001, value))
	}
	positions := make([]float64, 0, count)
	if includeEndpoints {
		positions = append(positions, 0.0)
		for index := 1; index < count-1; index++ {
			positions = append(positions, clamp(float64(index)/float64(count-1)+offset/float64(count-1)))
		}
		positions = append(positions, 1.0)
	} else {
		for index := 0; index < count; index++ {
			positions = append(positions, clamp((float64(index)+0.5+offset)/float64(count)))
		}
	}
	times := make([]float64, len(positions))
	for index, position := range positions {
		times[index] = math.Round((margin+span*position)*1000) / 1000
	}
	return times
}
