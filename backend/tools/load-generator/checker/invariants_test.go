package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// histAt builds a history whose samples carry one collector target with the
// given series values, spaced a second apart (inside the retention window)
// and past the warm-up.
func histAt(t *testing.T, name string, values []float64) *history {
	t.Helper()
	h := newHistory(0, time.Hour)
	base := time.Now().Add(-time.Duration(len(values)) * time.Second)
	h.started = base // samples are backdated; keep them past the warm-up
	for i, v := range values {
		h.append(sample{
			at:      base.Add(time.Duration(i) * time.Second),
			targets: map[string]metrics{"http://c0:8081/metrics": {name: v}},
		})
	}
	return h
}

func findInvariant(t *testing.T, name string) invariant {
	t.Helper()
	for _, inv := range allInvariants(invariantConfig{maxHotLag: 15 * time.Minute}) {
		if inv.name == name {
			return inv
		}
	}
	t.Fatalf("invariant %q is not defined", name)
	return invariant{}
}

func TestIngestPausedRatio(t *testing.T) {
	inv := findInvariant(t, "ingest-paused-not-sticky")

	flat := make([]float64, 200)
	assert.NoError(t, inv.check(histAt(t, "profiler_backpressure_ingest_paused", flat)),
		"never paused must pass")

	flat[10] = 1 // one paused sample out of 200 = 0.5% < 1%
	assert.NoError(t, inv.check(histAt(t, "profiler_backpressure_ingest_paused", flat)))

	for i := 0; i < 5; i++ { // 6/200 = 3%
		flat[20+i] = 1
	}
	assert.Error(t, inv.check(histAt(t, "profiler_backpressure_ingest_paused", flat)),
		"paused 3%% of the run must violate the 1%% budget")
}

func TestRefusedBytesInvariant(t *testing.T) {
	inv := findInvariant(t, "no-refused-bytes")
	assert.NoError(t, inv.check(histAt(t, "profiler_ingest_refused_bytes_total", []float64{0, 0, 0})))
	assert.Error(t, inv.check(histAt(t, "profiler_ingest_refused_bytes_total", []float64{0, 0, 4096})))
}

func TestHotWindowLagInvariant(t *testing.T) {
	inv := findInvariant(t, "hot-window-lag-bounded")
	assert.NoError(t, inv.check(histAt(t, "profiler_hotstore_hot_window_lag_seconds", []float64{30, 120, 300})))
	assert.Error(t, inv.check(histAt(t, "profiler_hotstore_hot_window_lag_seconds", []float64{30, 1200})),
		"20 min lag must exceed the 15 min budget")
}

func TestMonotonicGrowth(t *testing.T) {
	assert.NoError(t, monotonicGrowth([]float64{100, 120, 90, 130, 95}),
		"an oscillating series passes")
	assert.NoError(t, monotonicGrowth([]float64{100, 100, 102, 103}),
		"3%% net growth sits inside the tolerance")
	assert.Error(t, monotonicGrowth([]float64{100, 110, 120, 150}),
		"never-decreasing 50%% growth is the leak signal")
	assert.Error(t, monotonicGrowth([]float64{0, 0, 4096}),
		"growth from an empty store still counts")
}

func TestHotStoreGrowthNeedsFullWindow(t *testing.T) {
	inv := findInvariant(t, "hot-store-not-growing")
	// Growing steadily, but the samples span less than the 1h window: the
	// trend is not judged yet.
	h := histAt(t, "profiler_hotstore_segments_disk_bytes", []float64{100, 200, 300})
	require.NoError(t, inv.check(h))
}

func TestSumSeriesByTarget(t *testing.T) {
	s := sample{at: time.Now(), targets: map[string]metrics{
		"c0": {"a": 1, "b": 10},
		"c1": {"a": 2},
	}}
	sums := sumSeriesByTarget([]sample{s}, "a", "b")
	assert.Equal(t, []float64{11}, sums["c0"])
	assert.Equal(t, []float64{2}, sums["c1"])
}
