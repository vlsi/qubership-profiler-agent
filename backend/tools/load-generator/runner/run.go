package main

import (
	"context"
	"fmt"
	"math"
	"time"
)

// stepRecord is one steps.jsonl line (doc/run-orchestration.md).
type stepRecord struct {
	Level       int                `json:"level"`
	StartedAt   time.Time          `json:"startedAt"`
	ConfirmedAt *time.Time         `json:"confirmedAt,omitempty"`
	EndedAt     time.Time          `json:"endedAt"`
	Verdict     string             `json:"verdict"` // ok | saturated | invalid
	Reasons     []string           `json:"reasons,omitempty"`
	Plateau     bool               `json:"plateau"`
	Measurement map[string]float64 `json:"measurements"`
	Generator   *generatorRecord   `json:"generator,omitempty"`
	// Pprof marks the re-hold steps that exist only to capture profiles.
	Pprof string `json:"pprof,omitempty"`
}

type generatorRecord struct {
	CPUCores float64 `json:"cpuCores"`
	CPUShare float64 `json:"cpuShare"`
	Valid    bool    `json:"valid"`
}

// runResult is result.json.
type runResult struct {
	Name            string    `json:"name"`
	TestID          string    `json:"testid"`
	StartedAt       time.Time `json:"startedAt"`
	EndedAt         time.Time `json:"endedAt"`
	Verdict         string    `json:"verdict"` // completed | saturated | invalid
	CeilingLevel    int       `json:"ceilingLevel,omitempty"`
	SaturatedLevel  int       `json:"saturatedLevel,omitempty"`
	FiringDetectors []string  `json:"firingDetectors,omitempty"`
	InvalidReasons  []string  `json:"invalidReasons,omitempty"`
	Steps           int       `json:"steps"`
}

type runner struct {
	spec *Spec
	k6   *k6Client
	vm   *vmClient
	art  *artifacts

	// baselines holds the first-ok-step mean per baseline-ratio detector.
	baselines map[string]float64
	startedAt time.Time
}

// namedQueries is every series the run samples and archives.
func (r *runner) namedQueries() map[string]string {
	qs := map[string]string{}
	for name, q := range r.spec.Ramp.Hold.Plateau.Series {
		qs[name] = q
	}
	if ing := r.spec.Ramp.Confirm.Ingest; ing.BytesPerVU > 0 {
		qs["ingest-confirm"] = ing.Query
	}
	for _, d := range r.spec.Detectors {
		qs[d.Name] = d.Query
	}
	for name, q := range r.spec.Context {
		qs[name] = q
	}
	if g := r.spec.Guard.GeneratorCPU; g.Query != "" {
		qs["generator-cpu"] = g.Query
	}
	return qs
}

func (r *runner) vusQuery() string {
	return fmt.Sprintf("max(k6_vus{testid=%q})", r.spec.Run.TestID)
}

func (r *runner) connectionsQuery() string {
	if q := r.spec.Ramp.Confirm.ConnectionsQuery; q != "" {
		return q
	}
	return "sum(profiler_ingest_active_connections)"
}

// preflight verifies the endpoints and the stale-deployment guards: the k6
// deployment must already emit k6_vus under this run's testid, and its
// workload fingerprint must equal the spec's frozen workload block
// (doc/run-orchestration.md, "Workload wiring").
func (r *runner) preflight(ctx context.Context) error {
	if _, err := r.k6.Status(ctx); err != nil {
		return fmt.Errorf("k6 REST API: %w", err)
	}
	deadline := time.Now().Add(r.spec.Ramp.Confirm.Timeout.std())
	vusSeen := false
	var lastErr error
	for {
		if !vusSeen {
			_, seen, err := r.vm.Instant(ctx, r.vusQuery())
			if err != nil {
				return fmt.Errorf("VictoriaMetrics: %w", err)
			}
			vusSeen = seen
			lastErr = fmt.Errorf("no k6_vus series with testid=%q — the k6 deployment's TESTID does not match the spec (stale deployment?)",
				r.spec.Run.TestID)
		}
		if vusSeen {
			if len(r.spec.Workload) == 0 {
				return fmt.Errorf("spec.workload is empty — the workload block is a complete freeze and is required")
			}
			fp, err := r.fetchFingerprint(ctx)
			if err != nil {
				return fmt.Errorf("VictoriaMetrics: %w", err)
			}
			incomplete, err := verifyWorkload(r.spec.Workload, fp)
			if err == nil {
				return nil
			}
			if !incomplete {
				return err // hard mismatch: waiting will not fix it
			}
			lastErr = err // knobs not exported yet: remote write lags
		}
		if time.Now().After(deadline) {
			return lastErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

// confirmLevel waits until k6 reports the target VUs and, for T3, until the
// collector holds the expected connection count.
func (r *runner) confirmLevel(ctx context.Context, level int) error {
	deadline := time.Now().Add(r.spec.Ramp.Confirm.Timeout.std())
	wantConns := float64(level * r.spec.Ramp.Confirm.ConnectionsPerVU)
	for {
		vus, seen, err := r.vm.Instant(ctx, r.vusQuery())
		if err != nil {
			return err
		}
		ok := seen && vus >= float64(level)
		if ok && wantConns > 0 {
			conns, seenConns, err := r.vm.Instant(ctx, r.connectionsQuery())
			if err != nil {
				return err
			}
			ok = seenConns && conns >= wantConns
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			if wantConns > 0 {
				return fmt.Errorf("level %d not confirmed within %s (vus=%.0f, want connections >= %.0f)",
					level, r.spec.Ramp.Confirm.Timeout.std(), vus, wantConns)
			}
			return fmt.Errorf("level %d not confirmed within %s (vus=%.0f)",
				level, r.spec.Ramp.Confirm.Timeout.std(), vus)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(r.spec.Ramp.Hold.Sample.std()):
		}
	}
}

// holdStep samples every named series until the plateau criterion, a
// detector, the guard, or hold.max ends the hold.
func (r *runner) holdStep(ctx context.Context, level int) (stepRecord, error) {
	spec := r.spec
	rec := stepRecord{Level: level, StartedAt: time.Now(), Verdict: "ok", Measurement: map[string]float64{}}

	if err := r.k6.ScaleVUs(ctx, level); err != nil {
		return rec, err
	}
	if err := r.confirmLevel(ctx, level); err != nil {
		rec.EndedAt = time.Now()
		rec.Verdict = "invalid"
		rec.Reasons = append(rec.Reasons, "confirm-timeout: "+err.Error())
		return rec, nil
	}
	now := time.Now()
	rec.ConfirmedAt = &now

	queries := r.namedQueries()
	points := map[string][]Point{}
	holdStart := time.Now()
	plateauW := spec.Ramp.Hold.Plateau.Window.std()
	tol := spec.Ramp.Hold.Plateau.SlopeTolerance
	ingestChecked := false

	for {
		select {
		case <-ctx.Done():
			return rec, ctx.Err()
		case <-time.After(spec.Ramp.Hold.Sample.std()):
		}
		at := time.Now()
		for name, q := range queries {
			v, seen, err := r.vm.Instant(ctx, q)
			if err != nil {
				rec.EndedAt = time.Now()
				rec.Verdict = "invalid"
				rec.Reasons = append(rec.Reasons, fmt.Sprintf("query %s: %v", name, err))
				return rec, nil
			}
			if seen {
				points[name] = append(points[name], Point{At: at, Value: v})
			}
		}

		// Actual-vs-declared load check, once per hold, over the first full
		// plateau window after confirm: a run whose measured ingest does not
		// match the spec would describe a load the spec does not
		// (doc/run-orchestration.md, "Workload wiring").
		if ing := spec.Ramp.Confirm.Ingest; ing.BytesPerVU > 0 && !ingestChecked &&
			at.Sub(*rec.ConfirmedAt) >= plateauW {
			ingestChecked = true
			if err := checkIngest(level, ing.BytesPerVU, ing.Tolerance,
				meanOf(points["ingest-confirm"], plateauW)); err != nil {
				rec.EndedAt = time.Now()
				rec.Verdict = "invalid"
				rec.Reasons = append(rec.Reasons, err.Error())
				r.finishMeasurements(&rec, points, plateauW)
				return rec, nil
			}
		}

		// Generator guard first: past it the numbers measure the generator.
		if g := spec.Guard.GeneratorCPU; g.Query != "" && g.LimitCores > 0 {
			cores := meanOf(points["generator-cpu"], plateauW)
			share := cores / g.LimitCores
			rec.Generator = &generatorRecord{CPUCores: cores, CPUShare: share, Valid: share <= g.MaxShare}
			if !rec.Generator.Valid {
				rec.EndedAt = time.Now()
				rec.Verdict = "invalid"
				rec.Reasons = append(rec.Reasons,
					fmt.Sprintf("generator-cpu: %.2f cores is %.0f%% of the %.1f-core limit (max %.0f%%)",
						cores, share*100, g.LimitCores, g.MaxShare*100))
				r.finishMeasurements(&rec, points, plateauW)
				return rec, nil
			}
		}

		for _, d := range spec.Detectors {
			if detectorFires(d, afterGrace(points[d.Name], holdStart, d.Grace.std()), plateauW, tol, r.baselines[d.Name]) {
				rec.Verdict = "saturated"
				rec.Reasons = append(rec.Reasons, d.Name)
			}
		}
		if rec.Verdict == "saturated" {
			rec.EndedAt = time.Now()
			r.finishMeasurements(&rec, points, plateauW)
			return rec, nil
		}

		held := time.Since(holdStart)
		if held >= spec.Ramp.Hold.Min.std() {
			flat := true
			for name := range spec.Ramp.Hold.Plateau.Series {
				if !isFlat(points[name], plateauW, tol) {
					flat = false
					break
				}
			}
			if flat {
				rec.Plateau = true
				rec.EndedAt = time.Now()
				r.finishMeasurements(&rec, points, plateauW)
				return rec, nil
			}
		}
		if held >= spec.Ramp.Hold.Max.std() {
			// Not a saturation signal: the plateau tolerance may be too
			// tight. Recorded as ok with plateau=false.
			rec.EndedAt = time.Now()
			r.finishMeasurements(&rec, points, plateauW)
			return rec, nil
		}
	}
}

// finishMeasurements stores the last-window means the report quotes, and the
// baselines after the first ok step.
func (r *runner) finishMeasurements(rec *stepRecord, points map[string][]Point, plateauW time.Duration) {
	for name, ps := range points {
		rec.Measurement[name] = meanOf(ps, plateauW)
	}
	if rec.Verdict == "ok" && len(r.baselines) == 0 {
		for _, d := range r.spec.Detectors {
			if d.Kind == "baseline-ratio" {
				r.baselines[d.Name] = meanOf(points[d.Name], plateauW)
			}
		}
	}
}

// capturePoint re-holds at a share of the ceiling and captures the profiles.
func (r *runner) capturePoint(ctx context.Context, ceiling int, point float64) (stepRecord, error) {
	level := int(math.Max(1, math.Round(float64(ceiling)*point)))
	rec := stepRecord{Level: level, StartedAt: time.Now(), Verdict: "ok",
		Measurement: map[string]float64{}, Pprof: fmt.Sprintf("%.0f%%", point*100)}

	if err := r.k6.ScaleVUs(ctx, level); err != nil {
		return rec, err
	}
	if err := r.confirmLevel(ctx, level); err != nil {
		rec.Verdict = "invalid"
		rec.Reasons = append(rec.Reasons, "confirm-timeout: "+err.Error())
		rec.EndedAt = time.Now()
		return rec, nil
	}
	now := time.Now()
	rec.ConfirmedAt = &now

	// Let the level settle before profiling — but cap the wait: a fixed-hold
	// soak sets hold.min to hours, and the capture level was just held for
	// that long anyway (doc/run-orchestration.md).
	settle := r.spec.Ramp.Hold.Min.std()
	if settle > 5*time.Minute {
		settle = 5 * time.Minute
	}
	select {
	case <-ctx.Done():
		return rec, ctx.Err()
	case <-time.After(settle):
	}
	for _, profile := range r.spec.Pprof.Profiles {
		path, err := capturePprof(ctx, r.spec.Endpoints.Collector, r.art.pprofDir(),
			profile, r.spec.Pprof.Seconds, point)
		if err != nil {
			rec.Verdict = "invalid"
			rec.Reasons = append(rec.Reasons, fmt.Sprintf("pprof %s: %v", profile, err))
			rec.EndedAt = time.Now()
			return rec, nil
		}
		fmt.Printf("runner: captured %s\n", path)
	}
	rec.EndedAt = time.Now()
	return rec, nil
}

// Run executes the whole ramp; see doc/run-orchestration.md for the model.
func (r *runner) Run(ctx context.Context) (runResult, error) {
	r.startedAt = time.Now()
	res := runResult{Name: r.spec.Run.Name, TestID: r.spec.Run.TestID, StartedAt: r.startedAt, Verdict: "completed"}

	if err := r.preflight(ctx); err != nil {
		return res, err
	}
	// Leave the fleet at 0 when the run ends, whatever happened.
	defer func() {
		scaleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		_ = r.k6.ScaleVUs(scaleCtx, 0)
	}()

	lastOK := 0
	for _, level := range r.spec.Ramp.Levels {
		fmt.Printf("runner: step -> %d VUs\n", level)
		rec, err := r.holdStep(ctx, level)
		if err != nil {
			return res, err
		}
		if err := r.art.appendStep(rec); err != nil {
			return res, err
		}
		res.Steps++
		fmt.Printf("runner: step %d VUs -> %s %v\n", level, rec.Verdict, rec.Reasons)

		switch rec.Verdict {
		case "ok":
			lastOK = level
		case "saturated":
			res.Verdict = "saturated"
			res.SaturatedLevel = level
			res.CeilingLevel = lastOK
			res.FiringDetectors = rec.Reasons
		case "invalid":
			res.Verdict = "invalid"
			res.InvalidReasons = rec.Reasons
		}
		if rec.Verdict != "ok" {
			break
		}
	}

	// Profile the ceiling: re-hold at the configured shares of the last
	// stable level. Without a ceiling (saturated on the first step, or the
	// ramp completed without saturation) profile the last ok level as 100%.
	profileBase := res.CeilingLevel
	if res.Verdict == "completed" {
		profileBase = lastOK
		res.CeilingLevel = 0 // no ceiling found — the ramp never saturated
	}
	if res.Verdict != "invalid" && profileBase > 0 {
		for _, point := range r.spec.Pprof.Points {
			rec, err := r.capturePoint(ctx, profileBase, point)
			if err != nil {
				return res, err
			}
			if err := r.art.appendStep(rec); err != nil {
				return res, err
			}
			if rec.Verdict != "ok" {
				res.InvalidReasons = append(res.InvalidReasons, rec.Reasons...)
			}
		}
	}

	res.EndedAt = time.Now()
	if err := r.art.exportSeries(ctx, r.vm, r.namedQueries(), r.startedAt, res.EndedAt,
		r.spec.Ramp.Hold.Sample.std()); err != nil {
		fmt.Printf("runner: series export: %v\n", err)
	}
	return res, r.art.writeResult(res)
}
