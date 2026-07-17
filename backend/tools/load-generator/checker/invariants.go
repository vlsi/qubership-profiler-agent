package main

import (
	"fmt"
	"strings"
	"time"
)

// history holds the run's metrics samples. Samples inside the warm-up are
// stored but excluded from checks; the trend invariants read only the
// trailing window.
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

// pairSample is one scrape's (first, second) values with the scrape time —
// the trend rules fit against real timestamps, not sample indexes, so
// scrape gaps do not distort the slope.
type pairSample struct {
	at            time.Time
	first, second float64
}

// pairedSeriesByTarget collects, per target, the value pairs of two series
// taken from the same scrape, so both sides share time points. Samples where
// the target reported only one of the two are skipped.
func (h *history) pairedSeriesByTarget(first, second string, samples []sample) map[string][]pairSample {
	out := map[string][]pairSample{}
	for _, s := range samples {
		for target, m := range s.targets {
			a, okA := lookupSeries(m, first)
			b, okB := lookupSeries(m, second)
			if okA && okB {
				out[target] = append(out[target], pairSample{at: s.at, first: a, second: b})
			}
		}
	}
	return out
}

// lookupSeries finds a series by bare name or name{...} in one scrape.
func lookupSeries(m metrics, name string) (float64, bool) {
	if v, ok := m[name]; ok {
		return v, true
	}
	for key, v := range m {
		if strings.HasPrefix(key, name+"{") {
			return v, true
		}
	}
	return 0, false
}

type invariantConfig struct {
	maxHotLag time.Duration
	// rssLimitBytes enables the §8.6 RSS check; the pod memory limit is not
	// exposed on /metrics, so it must come from the operator.
	rssLimitBytes int64
	// goroutineTolerance is the §8.6 relative fitted-growth tolerance for
	// goroutines while the connection count stays constant.
	goroutineTolerance float64
	// maxScrapeGap latches target-unavailable after this many consecutive
	// failed polls of one source target.
	maxScrapeGap int
}

// invariant is one automated §8 check: a pure predicate over the current
// state. Failures are latched by the caller (doc/checker.md), so check
// returns every currently-failing subject, not a single error.
type invariant struct {
	name  string
	plan  string // the load-testing-plan.md §8 clause this enforces
	check func(st *state) []finding
}

// state aggregates every enabled data source for one evaluation tick.
// Sources the operator did not enable are nil.
type state struct {
	metrics *history
	s3      *s3state
	api     *apiState
	pods    *podState
	gaps    *gapTracker
}

func allInvariants(cfg invariantConfig) []invariant {
	invs := []invariant{
		{
			name: "hot-store-not-growing",
			plan: "§8.1",
			// The hot-store footprint must oscillate, not grow monotonically
			// over the window. Judged on the sum of the disk-side gauges; the
			// PV-level check (kubelet volume stats) needs the metrics of a
			// different scrape target and stays a TODO.
			check: func(st *state) []finding {
				h := st.metrics
				samples := h.checked()
				if !h.windowFull(samples) {
					return nil // not enough history to judge a trend yet
				}
				var out []finding
				sums := sumSeriesByTarget(samples,
					"profiler_hotstore_segments_disk_bytes",
					"profiler_hotstore_partitions_disk_bytes",
					"profiler_hotstore_wal_disk_bytes",
					"profiler_hotstore_pending_parquet_bytes")
				for target, values := range sums {
					if err := monotonicGrowth(values); err != nil {
						out = append(out, finding{subject: target, msg: err.Error()})
					}
				}
				return out
			},
		},
		{
			name: "ingest-paused-not-sticky",
			plan: "§8.2",
			check: func(st *state) []finding {
				h := st.metrics
				var out []finding
				for target, values := range h.seriesByTarget("profiler_backpressure_ingest_paused", h.checked()) {
					if ratio := pausedRatio(values); ratio >= 0.01 {
						out = append(out, finding{subject: target,
							msg: fmt.Sprintf("ingest paused %.1f%% of the run (budget 1%%)", ratio*100)})
					}
				}
				return out
			},
		},
		{
			name: "no-refused-bytes",
			plan: "§8.3",
			check: func(st *state) []finding {
				h := st.metrics
				var out []finding
				for target, values := range h.seriesByTarget("profiler_ingest_refused_bytes_total", h.checked()) {
					if n := len(values); n > 0 && values[n-1] > 0 {
						out = append(out, finding{subject: target,
							msg: fmt.Sprintf("ingest_refused_bytes_total = %.0f (must stay 0 at contract load)", values[n-1])})
					}
				}
				return out
			},
		},
		{
			name: "hot-window-lag-bounded",
			plan: "§8.4",
			check: func(st *state) []finding {
				h := st.metrics
				bound := cfg.maxHotLag.Seconds()
				var out []finding
				for target, values := range h.seriesByTarget("profiler_hotstore_hot_window_lag_seconds", h.checked()) {
					if n := len(values); n > 0 && values[n-1] > bound {
						out = append(out, finding{subject: target,
							msg: fmt.Sprintf("hot_window_lag %.0fs exceeds the %.0fs budget", values[n-1], bound)})
					}
				}
				return out
			},
		},
		{
			name: "s3-compaction-keeps-up",
			plan: "§8.5",
			check: func(st *state) []finding {
				if st.s3 == nil {
					return nil
				}
				return st.s3.checkCompaction(time.Now())
			},
		},
		{
			name: "s3-small-file-share",
			plan: "§8.5",
			check: func(st *state) []finding {
				if st.s3 == nil {
					return nil
				}
				return st.s3.checkSmallFileShare(time.Now())
			},
		},
		{
			name: "rss-under-limit",
			plan: "§8.6",
			check: func(st *state) []finding {
				if cfg.rssLimitBytes <= 0 {
					return nil
				}
				h := st.metrics
				samples := h.checked()
				var out []finding
				for target, values := range h.seriesByTarget("process_resident_memory_bytes", samples) {
					n := len(values)
					if n == 0 {
						continue
					}
					if values[n-1] >= float64(cfg.rssLimitBytes) {
						out = append(out, finding{subject: target,
							msg: fmt.Sprintf("RSS %.0f bytes reached the %d-byte pod limit", values[n-1], cfg.rssLimitBytes)})
						continue
					}
					if h.windowFull(samples) {
						if err := monotonicGrowth(values); err != nil {
							out = append(out, finding{subject: target, msg: "RSS " + err.Error()})
						}
					}
				}
				return out
			},
		},
		{
			name: "goroutines-flat",
			plan: "§8.6",
			// The leak signal: goroutines must not TREND upward while the
			// connection count stays constant. Both series come from the same
			// scrapes of the same target, so they share time points; ticks
			// where connections move are not judged — a collector restart
			// drops the connection count, so restart-spanning windows fall
			// out automatically (doc/checker.md).
			check: func(st *state) []finding {
				h := st.metrics
				samples := h.checked()
				if !h.windowFull(samples) {
					return nil
				}
				var out []finding
				pairs := h.pairedSeriesByTarget("profiler_ingest_active_connections", "go_goroutines", samples)
				for target, ps := range pairs {
					conns := make([]float64, len(ps))
					gors := make([]tsPoint, len(ps))
					for i, p := range ps {
						conns[i] = p.first
						gors[i] = tsPoint{at: p.at, v: p.second}
					}
					if !seriesConstant(conns) {
						continue
					}
					if msg := goroutinesTrending(gors, cfg.goroutineTolerance); msg != "" {
						out = append(out, finding{subject: target, msg: msg})
					}
				}
				return out
			},
		},
		{
			name: "ui-queries-answering",
			plan: "§8.7",
			check: func(st *state) []finding {
				if st.api == nil {
					return nil
				}
				return st.api.findings()
			},
		},
		{
			name: "no-unexpected-restarts",
			plan: "§8.8",
			check: func(st *state) []finding {
				if st.pods == nil {
					return nil
				}
				return st.pods.findings()
			},
		},
		{
			name: "target-available",
			plan: "§8 (scrape gaps)",
			// A source that stays silent must not pass by absence: §8.1–§8.7
			// skip ticks they have no data for, and this rule is what makes
			// that skip safe (doc/checker.md).
			check: func(st *state) []finding {
				if st.gaps == nil {
					return nil
				}
				return st.gaps.findings(cfg.maxScrapeGap)
			},
		},
	}
	return invs
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
			return fmt.Errorf("grew monotonically from 0 to %.0f over the window", last)
		}
		return nil
	}
	if growth := (last - first) / first; growth > growthTolerance {
		return fmt.Errorf("grew monotonically by %.1f%% over the window", growth*100)
	}
	return nil
}

// seriesConstant is the §8.6 connection-count gate:
// range ≤ max(1% of mean, 2).
func seriesConstant(values []float64) bool {
	if len(values) < 2 {
		return false
	}
	lo, hi, sum := values[0], values[0], 0.0
	for _, v := range values {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
		sum += v
	}
	mean := sum / float64(len(values))
	allowed := 0.01 * mean
	if allowed < 2 {
		allowed = 2
	}
	return hi-lo <= allowed
}

// tsPoint is one timestamped observation for the trend rules.
type tsPoint struct {
	at time.Time
	v  float64
}

// §8.6 trend parameters: a trend needs enough points to mean anything, and
// tiny fitted growth (near-idle processes, worker-pool jitter) is noise, not
// a leak.
const (
	minTrendPoints    = 8
	goroutineAbsFloor = 10.0
)

// fittedGrowth least-squares-fits a line through the points (against real
// timestamps — scrape gaps are legal) and reports the fitted growth over the
// span plus the mean level. Under two points there is nothing to fit.
func fittedGrowth(points []tsPoint) (growth, mean float64) {
	if len(points) < 2 {
		return 0, 0
	}
	t0 := points[0].at
	var n, sumX, sumY, sumXY, sumXX float64
	for _, p := range points {
		x := p.at.Sub(t0).Seconds()
		n++
		sumX += x
		sumY += p.v
		sumXY += x * p.v
		sumXX += x * x
	}
	den := n*sumXX - sumX*sumX
	if den == 0 {
		return 0, sumY / n
	}
	slope := (n*sumXY - sumX*sumY) / den
	span := points[len(points)-1].at.Sub(t0).Seconds()
	return slope * span, sumY / n
}

// goroutinesTrending returns a violation message when the goroutine count
// shows a sustained upward trend at a constant connection count: the fitted
// growth over the window exceeds max(tolerance × mean, absolute floor) AND
// the tail quarter of the window is still climbing (its proportional share
// of the allowance). Oscillation around a flat baseline fits to ~zero growth
// and passes; growth that found a level (flat tail) passes too — the leak
// signal is growth that keeps going (doc/checker.md).
func goroutinesTrending(points []tsPoint, tolerance float64) string {
	if len(points) < minTrendPoints {
		return ""
	}
	growth, mean := fittedGrowth(points)
	allowed := tolerance * mean
	if allowed < goroutineAbsFloor {
		allowed = goroutineAbsFloor
	}
	if growth <= allowed {
		return ""
	}
	span := points[len(points)-1].at.Sub(points[0].at)
	cut := points[len(points)-1].at.Add(-span / 4)
	tail := points
	for i, p := range points {
		if !p.at.Before(cut) {
			tail = points[i:]
			break
		}
	}
	// A sparse tail (long scrape gap) cannot prove the growth stopped; the
	// full-window verdict stands.
	if len(tail) >= 3 {
		tailGrowth, _ := fittedGrowth(tail)
		if tailGrowth <= allowed/4 {
			return "" // grew, then found its level — not a leak signal
		}
	}
	return fmt.Sprintf("goroutines trend +%.0f over the window (allowed %.0f) at a constant connection count, still climbing",
		growth, allowed)
}
