package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/common/expfmt"
)

// metrics is one target's scrape, flattened to "name" or "name{k=v,...}" with
// label pairs sorted, so a series key is stable across scrapes.
type metrics map[string]float64

// sample is one poll of every target at one instant.
type sample struct {
	at      time.Time
	targets map[string]metrics
}

// scrapeAll polls every target once. A failed target is reported and simply
// absent from the sample — a soak must survive a collector restart without
// the checker dying, and §8.8 (no unexplained restarts) is a separate check.
func scrapeAll(ctx context.Context, targets []string) (sample, []error) {
	s := sample{at: time.Now(), targets: make(map[string]metrics, len(targets))}
	var errs []error
	for _, target := range targets {
		target = strings.TrimSpace(target)
		m, err := scrape(ctx, target)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", target, err))
			continue
		}
		s.targets[target] = m
	}
	return s, errs
}

func scrape(ctx context.Context, url string) (metrics, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", resp.Status)
	}

	var parser expfmt.TextParser
	families, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, err
	}
	out := make(metrics, len(families))
	for name, mf := range families {
		for _, m := range mf.GetMetric() {
			key := name
			if len(m.GetLabel()) > 0 {
				pairs := make([]string, 0, len(m.GetLabel()))
				for _, l := range m.GetLabel() {
					pairs = append(pairs, l.GetName()+"="+l.GetValue())
				}
				sort.Strings(pairs)
				key = name + "{" + strings.Join(pairs, ",") + "}"
			}
			switch {
			case m.GetCounter() != nil:
				out[key] = m.GetCounter().GetValue()
			case m.GetGauge() != nil:
				out[key] = m.GetGauge().GetValue()
			case m.GetUntyped() != nil:
				out[key] = m.GetUntyped().GetValue()
				// Histogram and summary families are skipped: no §8 invariant
				// reads one, and flattening buckets would bloat the history.
			}
		}
	}
	return out, nil
}
