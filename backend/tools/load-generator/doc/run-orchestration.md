# Run orchestration

Contract for the ramp-run layer of the load-testing harness (`load-testing-plan.md` §5.3): how a ceiling run is
specified, executed, judged, and archived. The implementation is `tools/load-generator/runner`; the traffic comes from
the k6 fleet scenario (`scripts/scenario.js`) driven over the k6 REST API. Fault injection is out of scope (T7,
phase 5).

## Model

One **run** is one k6 process executing one sweep along one axis (`load-testing-plan.md` §7.2–§7.3). The k6 scenario
uses the `externally-controlled` executor and starts at 0 VUs; the runner raises the VU count step by step, so the
connections of earlier steps stay alive — mandatory for T3, where a per-step reconnect storm would poison the
RAM-per-connection numbers.

Each **step** goes through four stages:

1. **Scale**: `PATCH /v1/status` on the k6 REST API with the step's VU target. A 200 response only means the request
   was accepted.
2. **Confirm**: wait until `k6_vus` reports the target, and — when `confirm.connectionsPerVU` is set (T3) — until
   `profiler_ingest_active_connections` reaches `level × connectionsPerVU`. A confirmation timeout marks the step
   `invalid` with the reason recorded, and ends the run.
3. **Hold**: sample the plateau series until every one of them is flat (see *Plateau*) but at least `hold.min`, at most
   `hold.max`. Detectors run on every sample.
4. **Verdict**: `ok` (plateau reached, no detector fired), `saturated` (a detector fired; the previous step is the
   ceiling candidate), or `invalid` (confirmation timeout, generator guard, query failures).

After a `saturated` verdict the runner re-holds at the pprof points (by default 70% and 100% of the last `ok` level,
rounded to whole VUs) and captures CPU, heap, and goroutine profiles at each.

## Run spec

The spec is one YAML file; the runner copies it verbatim into the artifact directory before the first scale, together
with everything needed to compare two runs:

- the **run label** (`run.testid`): a unique string, also set as the k6 deployment's `TESTID` env, so k6 series of
  different runs never mix in VictoriaMetrics. Collector series carry no run label; they are separated by the step
  time windows recorded in `steps.jsonl`.
- the **full workload** (`workload:`): every `scripts/scenario.js` env knob, mirroring `load-testing-plan.md` §4. The
  runner does not push these to the k6 pod; it records them and refuses to run when `TESTID` visible in `k6_vus`
  labels does not match `run.testid` (a stale deployment guard).
- the **images** (`images:`): backend and runner references with digests.
- the **Helm values snapshot** (`helmValues:`): path to the rendered values (resource limits included); copied into
  the artifacts.
- the **detector queries and thresholds**, PromQL verbatim.

```yaml
run:
  name: t2-bytes            # t2-bytes | t2-calls-small | t2-calls-large | t2-dict-churn | t3-connections
  testid: t2-bytes-20260716a
outputs: runs/              # the runner creates runs/<started>-<name>/

endpoints:
  k6: http://localhost:6565         # k6 REST API (kubectl port-forward svc/cdt-loader-service 6565:6565)
  vm: http://localhost:8429         # VictoriaMetrics HTTP API (port-forward vmsingle-k8s 8429:8429)
  collector: http://localhost:8081  # collector internal port: /metrics and /debug/pprof

images:
  backend: profiler-backend@sha256:...
  runner: cdt-load-generator@sha256:...
helmValues: ./values-snapshot.yaml

workload:                   # frozen copy of the k6 deployment env (scenario knobs)
  PODS_PER_VU: "1"
  THREADS_PER_POD: "8"
  CALLS_PER_SEC: "5"
  # ... every knob that differs from the scenario defaults

ramp:
  levels: [10, 20, 40, 80, 160]     # VUs per step, one axis at a time
  confirm:
    timeout: 3m
    connectionsPerVU: 0             # T3: pods per VU; 0 disables the connection check
  hold:
    min: 3m
    max: 15m
    sample: 15s                     # detector/plateau sampling cadence
    plateau:
      window: 2m
      slopeTolerance: 0.05          # relative growth per window that still counts as flat
      series:
        ingest-bytes: sum(rate(profiler_ingest_bytes_total{namespace="profiler-load"}[1m]))
        collector-rss: sum(container_memory_working_set_bytes{namespace="profiler-load", pod=~".*collector.*", container!=""})

detectors:                          # any hit => the step verdict is `saturated`
  - name: ingest-paused
    kind: sticky-share              # fires when the gauge is nonzero > `share` of the hold so far
    share: 0.05
    query: max(profiler_backpressure_ingest_paused{namespace="profiler-load"})
  - name: pending-parquet-growth    # seal/upload primary signal; see the note below
    kind: monotonic-growth
    minGrowth: 0.10                 # relative growth over the hold that counts as real
    query: sum(profiler_hotstore_pending_parquet_bytes{namespace="profiler-load"})
  - name: refused-bytes
    kind: nonzero
    query: sum(rate(profiler_ingest_refused_bytes_total{namespace="profiler-load"}[1m]))
  - name: ack-flush-degraded
    kind: baseline-ratio            # first `ok` step's mean is the baseline
    ratio: 5
    query: max(k6_vdumper_ack_flush_time_p95{testid="t2-bytes-20260716a"})
  - name: ack-errors
    kind: nonzero
    query: sum(rate(k6_vdumper_ack_errors_total{testid="t2-bytes-20260716a"}[1m]))

context:                            # sampled and archived, never a trigger
  upload-lag: max(profiler_upload_lag_seconds{namespace="profiler-load"})
  upload-backlog: sum(profiler_upload_backlog{namespace="profiler-load"})
  hot-window-lag: max(profiler_hotstore_hot_window_lag_seconds{namespace="profiler-load"})
  active-connections: sum(profiler_ingest_active_connections{namespace="profiler-load"})
  collector-goroutines: sum(go_goroutines{namespace="profiler-load", pod=~".*collector.*"})

guard:                              # plan §10: never trust a ceiling set by the generator
  generator-cpu:
    query: sum(rate(container_cpu_usage_seconds_total{namespace="profiler-load", pod=~"cdt-loader.*", container!=""}[1m]))
    limitCores: 2.0                 # the k6 pod's CPU limit from the Helm values
    maxShare: 0.7                   # above this the step is `invalid`, not a ceiling

pprof:
  points: [0.7, 1.0]                # shares of the ceiling level
  seconds: 30
  profiles: [profile, heap, goroutine]
```

**Why `pending_parquet_bytes` is the seal/upload primary.** `profiler_upload_backlog` counts *files* and parquet files
vary in size, so the count alone misreads mixed workloads; a hard AND across three series would miss saturation when
bytes hit the budget before the file count moves. The byte gauge growing monotonically through the whole hold is the
trigger; `upload_lag_seconds` and `upload_backlog` are archived as confirming context for the report.

## Detector kinds

All detectors evaluate over the samples of the current hold only.

- `sticky-share`: the instant value is nonzero in more than `share` of the samples so far.
- `monotonic-growth`: the series grew by more than `minGrowth` (relative) over the hold *and* the last plateau window
  shows no flattening (its relative slope stays above `slopeTolerance`).
- `nonzero`: any sample above zero.
- `baseline-ratio`: the mean over the last plateau window exceeds `ratio ×` the baseline; the baseline is the mean of
  the same query over the first `ok` step's hold. Until a baseline exists the detector stays silent.

**Plateau**: a series is flat when the relative growth of a least-squares fit over the last `plateau.window` is within
`slopeTolerance`. The hold ends at the first sample where every plateau series is flat (but not before `hold.min`);
`hold.max` ends the hold with verdict `ok` and `plateau: false` recorded — a hint the tolerance is too tight, not a
saturation signal by itself.

## Artifacts

Everything lands under `runs/<started-UTC>-<name>/` (gitignored; runs never enter the repository):

```text
spec.yaml               the spec, copied before the first scale
values-snapshot.yaml    the Helm values referenced by helmValues
steps.jsonl             one line per step (see below)
series/<name>.json      query_range export of every plateau/detector/context/guard series over the whole run
pprof/<profile>-<point>pct.pb.gz   e.g. cpu-70pct.pb.gz, heap-100pct.pb.gz, goroutine-100pct.pb.gz
result.json             run verdict: ceiling level, firing detector, invalid flags, step index
```

A `steps.jsonl` line:

```json
{"level": 80, "startedAt": "...", "confirmedAt": "...", "endedAt": "...", "verdict": "saturated",
 "reasons": ["pending-parquet-growth"], "plateau": true,
 "measurements": {"ingest-bytes": 5.2e6, "collector-rss": 4.1e8, "upload-lag": 42.0},
 "generator": {"cpuCores": 0.9, "cpuShare": 0.45, "valid": true}}
```

`measurements` carries the mean of each series over the last plateau window — the numbers the report quotes.
`generator.valid: false` (CPU share above `maxShare`) marks the step `invalid`: the run stops and no ceiling is
reported, because past that point the harness measures the generator, not the collector.

## Runner CLI

```bash
go run ./tools/load-generator/runner -spec t2-bytes.yaml
```

The runner takes ready URLs and never manages port-forwards; the runbook (`doc/ceiling-runs.md`) lists the
`kubectl port-forward` lines per stand. Exit code 0 means the run completed with a verdict (including `saturated` —
finding the ceiling is the point); nonzero means the run aborted (`invalid` step, endpoint failure, spec error).
