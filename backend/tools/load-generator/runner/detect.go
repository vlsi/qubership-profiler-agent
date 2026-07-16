package main

import (
	"time"
)

// Point is one sample of a series during a hold.
type Point struct {
	At    time.Time
	Value float64
}

// relativeSlope fits a least-squares line through the points and reports the
// fitted growth over the span, relative to the fitted mean level — "the
// series grew by X% over this window". A flat or empty window reports 0; a
// window around a zero mean reports 0 too (nothing to grow relative to).
func relativeSlope(points []Point) float64 {
	if len(points) < 2 {
		return 0
	}
	t0 := points[0].At
	var n, sumX, sumY, sumXY, sumXX float64
	for _, p := range points {
		x := p.At.Sub(t0).Seconds()
		n++
		sumX += x
		sumY += p.Value
		sumXY += x * p.Value
		sumXX += x * x
	}
	den := n*sumXX - sumX*sumX
	if den == 0 {
		return 0
	}
	slope := (n*sumXY - sumX*sumY) / den
	mean := sumY / n
	if mean == 0 {
		return 0
	}
	span := points[len(points)-1].At.Sub(t0).Seconds()
	return slope * span / mean
}

// window trims the series to samples within d of the last sample.
func window(points []Point, d time.Duration) []Point {
	if len(points) == 0 {
		return nil
	}
	cut := points[len(points)-1].At.Add(-d)
	for i, p := range points {
		if !p.At.Before(cut) {
			return points[i:]
		}
	}
	return points[len(points)-1:]
}

// isFlat reports whether the last plateau window of the series grew by less
// than tolerance (absolute value: a controlled decline is flat too, e.g. RSS
// after a GC).
func isFlat(points []Point, plateauWindow time.Duration, tolerance float64) bool {
	w := window(points, plateauWindow)
	if len(w) < 3 {
		return false // not enough samples to call anything flat
	}
	s := relativeSlope(w)
	return s < tolerance && s > -tolerance
}

// meanOf reports the mean of the last plateau window — the number the report
// quotes for a step.
func meanOf(points []Point, plateauWindow time.Duration) float64 {
	w := window(points, plateauWindow)
	if len(w) == 0 {
		return 0
	}
	sum := 0.0
	for _, p := range w {
		sum += p.Value
	}
	return sum / float64(len(w))
}

// detectorFires evaluates one detector over the current hold's samples.
// baseline is the first-ok-step mean for baseline-ratio (0 = no baseline yet,
// the detector stays silent). Kinds are defined in doc/run-orchestration.md.
func detectorFires(d Detector, points []Point, plateauWindow time.Duration,
	slopeTolerance, baseline float64) bool {

	if len(points) == 0 {
		return false
	}
	switch d.Kind {
	case "sticky-share":
		nonzero := 0
		for _, p := range points {
			if p.Value > 0 {
				nonzero++
			}
		}
		return float64(nonzero)/float64(len(points)) > d.Share
	case "monotonic-growth":
		first, last := points[0].Value, points[len(points)-1].Value
		grown := false
		switch {
		case first == 0:
			grown = last > 0
		default:
			grown = (last-first)/first > d.MinGrowth
		}
		// Still-climbing check: a series that grew and then flattened found
		// its level; only growth with no plateau in the last window fires.
		return grown && !isFlat(points, plateauWindow, slopeTolerance) &&
			relativeSlope(window(points, plateauWindow)) > 0
	case "nonzero":
		for _, p := range points {
			if p.Value > 0 {
				return true
			}
		}
		return false
	case "baseline-ratio":
		if baseline <= 0 {
			return false
		}
		return meanOf(points, plateauWindow) > d.Ratio*baseline
	}
	return false
}
