package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var t0 = time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

// series builds points at a fixed 15 s cadence.
func series(values ...float64) []Point {
	ps := make([]Point, len(values))
	for i, v := range values {
		ps[i] = Point{At: t0.Add(time.Duration(i) * 15 * time.Second), Value: v}
	}
	return ps
}

const w = 2 * time.Minute // plateau window: 8 samples at 15 s

func TestIsFlat(t *testing.T) {
	assert.True(t, isFlat(series(100, 101, 99, 100, 100, 101, 100, 99, 100), w, 0.05),
		"noise around a level is flat")
	assert.False(t, isFlat(series(100, 110, 121, 133, 146, 161, 177, 195, 214), w, 0.05),
		"10% growth per sample is not flat")
	assert.False(t, isFlat(series(100, 100), w, 0.05),
		"two samples cannot prove a plateau")
	assert.True(t, isFlat(series(0, 0, 0, 0, 0, 0, 0, 0), w, 0.05),
		"an all-zero series is flat")
}

func TestRelativeSlopeWindowing(t *testing.T) {
	// Growth long ago, flat lately: only the last window is judged.
	ps := series(10, 20, 40, 80, 160, 160, 161, 159, 160, 160, 161, 160, 159, 160)
	assert.True(t, isFlat(ps, w, 0.05), "a series that found its level is flat")
}

func TestMeanOfLastWindow(t *testing.T) {
	ps := series(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100, 100, 100, 100, 100, 100, 100, 100, 100)
	assert.InDelta(t, 100, meanOf(ps, w), 1, "the mean covers only the last window")
}

func TestStickyShare(t *testing.T) {
	d := Detector{Name: "ingest-paused", Kind: "sticky-share", Share: 0.05}
	assert.False(t, detectorFires(d, series(0, 0, 0, 0, 0, 0, 0, 0, 0, 0), w, 0.05, 0))
	assert.False(t, detectorFires(d, series(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1),
		w, 0.05, 0), "one blip in twenty samples stays under a 5% share")
	assert.True(t, detectorFires(d, series(0, 0, 1, 1, 0, 1, 0, 1, 0, 1), w, 0.05, 0),
		"a sticking gauge fires")
}

func TestMonotonicGrowth(t *testing.T) {
	d := Detector{Name: "pending-parquet", Kind: "monotonic-growth", MinGrowth: 0.10}
	assert.True(t, detectorFires(d, series(100, 120, 145, 172, 205, 246, 295, 350, 420, 500), w, 0.05, 0),
		"unbounded growth through the hold fires")
	assert.False(t, detectorFires(d, series(100, 120, 145, 160, 165, 166, 165, 166, 165, 166, 165, 166), w, 0.05, 0),
		"growth that found a plateau is a level shift, not saturation")
	assert.False(t, detectorFires(d, series(100, 101, 100, 102, 101, 100, 101, 100), w, 0.05, 0),
		"noise under minGrowth does not fire")
	assert.True(t, detectorFires(d, series(0, 10, 25, 45, 70, 100, 140, 190), w, 0.05, 0),
		"growth from zero fires")
}

func TestNonzero(t *testing.T) {
	d := Detector{Name: "refused-bytes", Kind: "nonzero"}
	assert.False(t, detectorFires(d, series(0, 0, 0), w, 0.05, 0))
	assert.True(t, detectorFires(d, series(0, 0, 0.5), w, 0.05, 0))
}

func TestBaselineRatio(t *testing.T) {
	d := Detector{Name: "ack-flush", Kind: "baseline-ratio", Ratio: 5}
	degraded := series(10, 10, 10, 10, 60, 62, 61, 63, 65, 64, 66, 65)
	assert.False(t, detectorFires(d, degraded, w, 0.05, 0),
		"no baseline yet: the detector stays silent")
	assert.True(t, detectorFires(d, degraded, w, 0.05, 10),
		"p95 at 6x the first-step baseline fires")
	assert.False(t, detectorFires(d, series(10, 11, 12, 11, 10, 12, 11, 12), w, 0.05, 10),
		"staying near the baseline does not fire")
}
