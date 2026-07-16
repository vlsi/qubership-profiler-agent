// Command checker watches a soak or contract run and fails with a report when
// a load-testing-plan.md §8 invariant breaks. It polls the collector and
// maintain /metrics endpoints on a fixed interval, keeps an in-memory history,
// and evaluates every invariant against that history after a warm-up period.
//
// Phase-1 skeleton: the metrics-driven invariants (§8.1–§8.4) are implemented;
// the S3 object-histogram, UI-sampling, and pod-restart checks (§8.5, §8.7,
// §8.8) are declared as TODO stubs so the harness shape is fixed before the
// soak campaign needs them.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	var (
		targets  = flag.String("targets", "", "comma-separated /metrics URLs to poll (collector replicas, maintain)")
		interval = flag.Duration("interval", 30*time.Second, "poll interval")
		warmup   = flag.Duration("warmup", 15*time.Minute, "run time excluded from invariant checks")
		window   = flag.Duration("window", 2*time.Hour, "sliding window for the trend invariants (§8.1)")
		maxLag   = flag.Duration("max-hot-lag", 15*time.Minute, "bound for hot_window_lag_seconds (§8.4): seal interval + grace budget")
		duration = flag.Duration("duration", 0, "total run time; 0 runs until SIGINT/SIGTERM")
	)
	flag.Parse()
	if *targets == "" {
		fmt.Fprintln(os.Stderr, "checker: -targets is required, e.g. -targets http://collector-0:8081/metrics,http://collector-1:8081/metrics")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	h := newHistory(*warmup, *window)
	invariants := allInvariants(invariantConfig{maxHotLag: *maxLag})

	fmt.Printf("checker: polling %s every %s; warmup %s, window %s\n",
		*targets, *interval, *warmup, *window)
	run(ctx, strings.Split(*targets, ","), *interval, h, invariants)

	violations := evaluate(h, invariants, true)
	if violations > 0 {
		fmt.Printf("checker: FAIL — %d invariant(s) violated\n", violations)
		os.Exit(1)
	}
	fmt.Println("checker: PASS — no invariant violations")
}

// run polls until ctx is done, evaluating invariants on every tick so a
// violation is visible live, not only in the final report.
func run(ctx context.Context, targets []string, interval time.Duration, h *history, invariants []invariant) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		s, errs := scrapeAll(ctx, targets)
		for _, err := range errs {
			fmt.Printf("%s scrape: %v\n", time.Now().Format(time.RFC3339), err)
		}
		h.append(s)
		evaluate(h, invariants, false)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// evaluate runs every invariant; final=true prints passes too and returns the
// violation count for the exit code.
func evaluate(h *history, invariants []invariant, final bool) int {
	violations := 0
	for _, inv := range invariants {
		err := inv.check(h)
		switch {
		case err != nil:
			violations++
			fmt.Printf("%s VIOLATION %s (%s): %v\n", time.Now().Format(time.RFC3339), inv.name, inv.plan, err)
		case final:
			fmt.Printf("PASS %s (%s)\n", inv.name, inv.plan)
		}
	}
	return violations
}
