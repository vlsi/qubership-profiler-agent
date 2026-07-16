package main

import (
	"fmt"
	"strings"
	"time"
)

// history holds the run's samples. Samples inside the warm-up are stored but
// excluded from checks; the trend invariants read only the trailing window.
type history struct {
	warmup  time.Duration
	window  time.Duration
	started time.Time
	samples []sample
}

func newHistory(warmup, window time.Duration) *history {
	return &history{warmup: warmup, window: window, started: time.Now()}
}

func (h *history) append(s sample) {
	h.samples = append(h.samples, s)
	// Cap memory for multi-day soaks: everything older than the trend window
	// (plus warm-up slack) has been judged already.
	horizon := time.Now().Add(-h.window - h.warmup)
	for len(h.samples) > 1 && h.samples[0].at.Before(horizon) {
		h.samples = h.samples[1:]
	}
}

// checked returns the samples the invariants may judge: past warm-up.
func (h *history) checked() []sample {
	var out []sample
	for _, s := range h.samples {
		if s.at.Sub(h.started) >= h.warmup {
			out = append(out, s)
		}
	}
	return out
}

// seriesByTarget collects, per target, the values of every series whose key
// starts with name (exact bare name or name{...}), in sample order. Absent
// samples (target down) are skipped.
func (h *history) seriesByTarget(name string, samples []sample) map[string][]float64 {
	out := map[string][]float64{}
	for _, s := range samples {
		for target, m := range s.targets {
			for key, v := range m {
				if key == name || strings.HasPrefix(key, name+"{") {
					out[target] = append(out[target], v)
				}
			}
		}
	}
	return out
}

type invariantConfig struct {
	maxHotLag time.Duration
}

// invariant is one automated §8 check. check returns nil while the invariant
// holds and a descriptive error once it is violated.
type invariant struct {
	name  string
	plan  string // the load-testing-plan.md §8 clause this enforces
	check func(h *history) error
}

func allInvariants(cfg invariantConfig) []invariant {
	return []invariant{
		{
			name: "hot-store-not-growing",
			plan: "§8.1",
			// The hot-store footprint must oscillate, not grow monotonically
			// over the window. Judged on the sum of the disk-side gauges; the
			// PV-level check (kubelet volume stats) needs the metrics of a
			// different scrape target and stays a TODO.
			check: func(h *history) error {
				samples := h.checked()
				if !h.windowFull(samples) {
					return nil // not enough history to judge a trend yet
				}
				sums := sumSeriesByTarget(samples,
					"profiler_hotstore_segments_disk_bytes",
					"profiler_hotstore_partitions_disk_bytes",
					"profiler_hotstore_wal_disk_bytes",
					"profiler_hotstore_pending_parquet_bytes")
				for target, values := range sums {
					if err := monotonicGrowth(values); err != nil {
						return fmt.Errorf("%s: %w", target, err)
					}
				}
				return nil
			},
		},
		{
			name: "ingest-paused-not-sticky",
			plan: "§8.2",
			check: func(h *history) error {
				samples := h.checked()
				for target, values := range h.seriesByTarget("profiler_backpressure_ingest_paused", samples) {
					if ratio := pausedRatio(values); ratio >= 0.01 {
						return fmt.Errorf("%s: ingest paused %.1f%% of the run (budget 1%%)", target, ratio*100)
					}
				}
				return nil
			},
		},
		{
			name: "no-refused-bytes",
			plan: "§8.3",
			check: func(h *history) error {
				samples := h.checked()
				for target, values := range h.seriesByTarget("profiler_ingest_refused_bytes_total", samples) {
					if n := len(values); n > 0 && values[n-1] > 0 {
						return fmt.Errorf("%s: ingest_refused_bytes_total = %.0f (must stay 0 at contract load)", target, values[n-1])
					}
				}
				return nil
			},
		},
		{
			name: "hot-window-lag-bounded",
			plan: "§8.4",
			check: func(h *history) error {
				samples := h.checked()
				bound := cfg.maxHotLag.Seconds()
				for target, values := range h.seriesByTarget("profiler_hotstore_hot_window_lag_seconds", samples) {
					for _, v := range values {
						if v > bound {
							return fmt.Errorf("%s: hot_window_lag %.0fs exceeds the %.0fs budget", target, v, bound)
						}
					}
				}
				return nil
			},
		},
		// TODO(§8.5): S3 object count and small-file share per hour prefix,
		// via libs/s3 listing — needs the bucket credentials plumbed in.
		// TODO(§8.6): RSS below the pod limit and goroutines flat at constant
		// connection count — the limit must come from the chart values or the
		// downward API, not be guessed here.
		// TODO(§8.7): sampled /api/v1 queries — fresh data visible within the
		// hot window, pre-soak marker calls retrievable from cold until TTL.
		// TODO(§8.8): no backend pod restarts besides the injected ones —
		// needs the k8s API, not /metrics.
	}
}

// windowFull reports whether the checked samples span the full trend window.
func (h *history) windowFull(samples []sample) bool {
	return len(samples) >= 2 && samples[len(samples)-1].at.Sub(samples[0].at) >= h.window
}

// sumSeriesByTarget sums the named gauges per target per sample, skipping
// samples where a target reported none of them.
func sumSeriesByTarget(samples []sample, names ...string) map[string][]float64 {
	out := map[string][]float64{}
	for _, s := range samples {
		for target, m := range s.targets {
			sum, seen := 0.0, false
			for _, name := range names {
				if v, ok := m[name]; ok {
					sum, seen = sum+v, true
				}
			}
			if seen {
				out[target] = append(out[target], sum)
			}
		}
	}
	return out
}

// pausedRatio is the fraction of samples with a 0/1 gauge at 1.
func pausedRatio(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	paused := 0
	for _, v := range values {
		if v >= 1 {
			paused++
		}
	}
	return float64(paused) / float64(len(values))
}

// growthTolerance separates real monotonic growth from a flat series with
// jitter: below 5% net growth over the window nothing is judged.
const growthTolerance = 0.05

// monotonicGrowth fails when the series never decreases across the window AND
// gained more than the tolerance — the §8.1 "grows monotonically" signal.
// A series that oscillates (any single decrease) passes by definition.
func monotonicGrowth(values []float64) error {
	if len(values) < 2 {
		return nil
	}
	for i := 1; i < len(values); i++ {
		if values[i] < values[i-1] {
			return nil
		}
	}
	first, last := values[0], values[len(values)-1]
	if first <= 0 {
		if last > 0 {
			return fmt.Errorf("hot-store footprint grew monotonically from 0 to %.0f bytes over the window", last)
		}
		return nil
	}
	if growth := (last - first) / first; growth > growthTolerance {
		return fmt.Errorf("hot-store footprint grew monotonically by %.1f%% over the window", growth*100)
	}
	return nil
}
