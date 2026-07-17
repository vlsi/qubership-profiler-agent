# Load-testing report: Go profiler backend

Status: **draft — harness and lifecycle invariants validated on the local stand; final numbers await the large
cluster** (`load-testing-plan.md` §5.2). Owner: @vlsi.

Every number in this report must cite its run: the artifact directory (`runs/<ts>-<name>/`, kept outside the
repository), the image digests, and the frozen run spec inside it. A number without a run citation is a placeholder,
not a result. Numbers from the local stand (OrbStack) validate the harness and are marked *(local, not a ceiling)* —
they must never be quoted as collector limits. One class of local outcome IS a result: the accelerated-timer soak's
lifecycle verdict (§5) is functional, not numeric, and carries to the cluster with the plan-§10 caveat that slow
leaks need the real-timer run.

## 1. Scope and method

This report covers the ceiling campaign (plan §9.3): the throughput ceiling (T2, plan §7.2) and the connection-count
ceiling (T3, plan §7.3) of a single collector replica, and the contract + soak campaign (plan §9.4): the contract
run (T1, §7.1), the soaks (T4, §7.4), and the background query load (T6, §7.6). Ceilings are measured with:

- traffic: the virtual dumper (`virtual-dumper.md`), calibrated against the real Java agent (plan §12);
- generator: the k6 fleet module (`tools/load-generator/pkg/cdt`), externally-controlled executor;
- orchestration: `tools/load-generator/runner` (`doc/run-orchestration.md`) — stepwise VU ramp with holds until the
  key series flatten, saturation detectors, pprof capture at 70% / 100% of the ceiling, artifact archival;
- runbooks: `tools/load-generator/doc/ceiling-runs.md`.

A step counts toward a ceiling only when the generator guard held (k6 pod CPU under 70% of its limit) and the level
was confirmed (`k6_vus`, and for T3 the `profiler_ingest_active_connections` gauge) before the hold started.

## 2. Stands

| Stand | Purpose | Disk / S3 | Status |
| --- | --- | --- | --- |
| OrbStack (local) | harness development, smoke runs | local SSD, in-cluster MinIO | used for everything below |
| Large k8s cluster | final numbers | network PVs, real node limits | **not yet available** |

MinIO understates real object-storage latency on both stands (plan §10); cold-read conclusions will carry that
caveat when T6 runs.

## 3. T2: single-replica throughput ceiling

> Placeholder — to be filled from large-cluster runs. Per sweep (bytes/s, calls/s small, calls/s large, dictionary
> churn): the ceiling in MB/s and calls/s, the firing detector, the limiting stage (ingest decode / SQLite index /
> seal / upload) read from the CPU profiles, and the CPU/RAM/disk-I/O curves over the ramp.

| Sweep | Ceiling | Firing detector | Limiting stage | Run |
| --- | --- | --- | --- | --- |
| bytes/s (pods) | — | — | — | — |
| calls/s, small calls | — | — | — | — |
| calls/s, large calls | — | — | — | — |
| dictionary churn | — | — | — | — |

### Harness validation *(local, not a ceiling)*

Two OrbStack runs on 2026-07-16, images `profiler-backend:dev@sha256:8a7ecf…` and
`cdt-load-generator:dev@sha256:1c580c…`:

- **Ramp mechanics** (`runs/20260716T111912Z-t2-bytes-smoke`): 2 → 4 → 8 pods (1 pod/VU, 8 threads, 5 calls/s).
  Every step confirmed through `k6_vus` (~40 s remote-write lag), reached its plateau, and scaled linearly —
  65 / 130 / 255 KB/s ingest. All six pprof profiles (CPU, heap, goroutine at 70% and 100%) landed, `steps.jsonl`,
  the series exports, and `result.json` are complete. Generator CPU stayed at 1–2% of its limit.
- **Saturation detection** (`runs/20260716T113242Z-t2-bytes-sat`): with `PROFILER_PENDING_UPLOAD_MAX_BYTES` shrunk
  to 8 MiB the first step fired three detectors at once — `ingest-paused`, `refused-bytes`, and the generator-side
  `ack-errors` — proving the whole backpressure chain end to end: the gate refuses `RCV_DATA`, the virtual dumpers
  see `ACK_ERROR_MAGIC` and enter the agent's reconnect path, and the runner stops the ramp with verdict
  `saturated`.

## 4. T3: connection-count ceiling

> Placeholder — to be filled from large-cluster runs: RAM and goroutines per idle connection, cost per tracked
> pod-restart, where accept latency / `PROFILER_MEM_BUDGET` / the fd limit bites first, and what the failure looks
> like to the agent (there is no accept-side connection cap today, `libs/server/services.go`).

| Metric | Value | Run |
| --- | --- | --- |
| RAM per idle connection | — | — |
| Goroutines per connection | — | — |
| RAM per tracked pod-restart | — | — |
| First limit hit | — | — |

The idle-pod profile is not empty (handshake + seven streams + initial dictionary/params, then keep-alive flush
cycles); the per-connection cost separates the one-time setup burst from the steady keep-alive cost
(`doc/ceiling-runs.md`).

### Harness validation *(local, not a ceiling)*

`runs/20260716T113525Z-t3-connections-smoke` (OrbStack, 2026-07-16): 3 → 6 fleets × 100 idle pods
(`THREADS_PER_POD=0`, `DICT_INITIAL=100`). The confirm phase waited on the new
`profiler_ingest_active_connections` gauge, which tracked the fleet size exactly (300, then 600). Step deltas
*(local, not a ceiling)*: +300 connections cost +3595 goroutines (~12 per connection), +2394 fds (~8 per
connection), and +78 MB RSS. Heap and goroutine profiles captured at the top level. One measurement nuance recorded
for real runs: k6 exports Time-typed trends in **seconds** over remote write, and connect-time trends only update
while connects happen, so accept-latency reads come from the confirm phase of each step, not from the plateau
window.

## 5. T4: soak

### Accelerated-timer lifecycle validation (local — a functional result)

Run `runs/20260716T144315Z-t4-soak-accelerated` (OrbStack, 2026-07-16; `local-soak` environment, images and frozen
spec in the run directory): 3 collector replicas, 20 pods × 8 threads × 3 calls/s ≈ 393 KB/s ingest, 1 m time
buckets, 15–45 m class TTLs, the full §8 checker (`checker.md`) plus the T6 UI profile in the background.

**Verdict: FAIL — and the failure is the deliverable.** After 52 healthy minutes, collector-2 crashed with
`panic: index out of range [-1]` in `libs/parser/pipe.CallsPipeReader` (`calls.go:70`: a negative thread index
from ignored mid-record read errors; stack in `collector-2-panic.log`, fix tracked separately). The checker and
the runner caught the crash and its whole cascade, each through its own §8 clause:

- §8.8 latched the collector-2 restart (restart budget 0);
- §8.5 latched compaction lateness while maintain drowned in the crash-recovery backlog (the `any_error` hour
  prefix grew to ~400 small objects);
- the runner's `pending-parquet-growth` detector ended the hold (`result.json`: `saturated`);
- the query API degraded during recovery (latched as a scrape gap).

During the healthy window every lifecycle stage cycled repeatedly and cleanly under the checker: 1 m buckets
sealed and uploaded (~100 small objects/hour/class), maintain merged 20–50-file groups per pass and TTL-deleted
expired objects (its pass logs show `CompactedGroups:5` at a ~1 m cadence), §8.7 markers stayed retrievable, and
settled hour prefixes drained to single-digit object counts. The functional conclusion carries to the cluster:
the lifecycle machinery works, the invariants catch real failures, and **the real-timer soak is blocked on the
`CallsPipeReader` panic** — under this traffic shape a collector dies in under two hours.

### Real-timer soak (pending cluster, blocked on the parser fix)

> Placeholder — the mandatory 24–48 h run on real timers (`specs/t4-soak.yaml`): checker verdict over the full §8
> set, RSS/goroutine trends, S3 object-count trends, and the slow-leak check the accelerated run cannot see
> (plan §10). Runs with the T6 UI profile in the background (`k6-query`).

## 6. T1: contract-level run (pending cluster)

> Placeholder — `specs/t1-contract.yaml` on 3 replicas, 500 pods, ~6 MB/s total, fixed 2 h hold. Deliverable:
> the baseline utilization table (collector CPU/RSS/PV I/O, query CPU/RSS, S3 traffic) and the headroom estimate
> of plan goal (a).

| Metric | Value | Run |
| --- | --- | --- |
| ingest bytes/s (cluster total) | — | — |
| collector CPU / replica | — | — |
| collector RSS / replica | — | — |
| PV write bytes/s | — | — |
| query CPU / RSS | — | — |
| ack-flush p95 | — | — |

### Spec-mechanics smoke *(local, not a contract number)*

`runs/20260716T155045Z-t1-contract` (OrbStack, 2026-07-16): the T1 spec scaled to `levels: [10]` and a fixed 12 m
hold, on the `local-soak` stand. Verdict `completed`: the confirm phase waited on
`profiler_ingest_active_connections` (connectionsPerVU: 1), the hold ran its full fixed length with every
§8-shaped detector silent (191 KB/s ingest, plateau reached, generator at 1% of its CPU limit), and the capture
step took CPU/heap/goroutine profiles at 100% of the level. The contract-run mechanics — fixed hold, detector
set, connection-gated confirm, `pprof.points: [1.0]` — are ready for the cluster.

## 7. T6: query load

> Cluster numbers are placeholders until T4 runs there; the local profiles below establish the qualitative
> behavior of the guards, the pagination cost model, and the read-vs-ingest interaction. Absolute latency and
> LIST/GET volumes are not portable from MinIO on a local SSD (plan §10).

### Read-path memory: concurrent wide queries OOM the query pod *(local, mechanism portable)*

The sharpest T6 finding. `PROFILER_MAX_SCAN_BYTES` (default 2 GB) is a **per-request** budget: concurrent
guard-passing wide-range queries multiply it, and the read path has no global memory budget. Observed on the
2026-07-16 stand (pod events in the k8s log; §8.8 caught every restart):

- 2 Gi limit, mixed cold + incident load: OOMKilled after ~29 min;
- 3 Gi limit, 3 UI + 5 incident VUs on wide ranges: OOMKilled after **34 seconds**;
- 3 Gi limit with the scan budget cut to 256 MB *and* incident off: stable.

Sizing the pod around the guard is backwards — the guard must be sized to the pod, and even then concurrency
multiplies it. Follow-up: a global scan budget (or admission semaphore) on the query read path.

### Guards and deep pagination *(local)*

- **Span guard** (probe `cold-probe-b`, 2 VUs, 400 m range > the 6 h limit): 36/36 requests rejected fail-closed
  with HTTP 400 in ~4 ms average — no I/O spent on rejected ranges.
- **Cost path** (probe `cold-probe-a`, 2 VUs, 350 m range, paging every `next_cursor`, against a bucket holding
  ~400 small pre-compaction objects in the hot hour): 21 pages in 4 min — **~23 s per page average, p95 59 s**,
  query CPU at 0.84 cores (84% of its 1-core limit), RSS ~1 GB. Every page re-resolves the fan-out and re-lists
  S3 by design (02 §2.3.1); the probe confirms the cost model directly.
- **Accelerated-density lesson**: guards scale with data density. At ~0.5 MB/s ingest the 256 MB scan budget
  rejected even a 7-minute freshness window; accelerated-timer stands must scale read windows and budgets
  together with the timers (`doc/soak-runs.md`).

### UI profile during the soak *(local)*

3 UI VUs (10 m windows at accelerated density, trace + tree per row) ran through the soak's healthy window with
no §8.7 freshness violations and no effect on ingest: 393 KB/s ingest stayed flat, `ingest_paused` never fired.
Hot-read pressure at UI levels does not push ingest toward backpressure on this stand; the incident/cold levels
that *do* hurt hit the query pod's memory first (above).

## 8. Invariant checker coverage (plan §8)

| Invariant | Source | Status |
| --- | --- | --- |
| §8.1 hot store not growing | /metrics | implemented (phase 1), latched |
| §8.2 ingest-paused not sticky | /metrics | implemented (phase 1), latched |
| §8.3 no refused bytes | /metrics | implemented (phase 1), latched |
| §8.4 hot-window lag bounded | /metrics | implemented (phase 1), latched |
| §8.5 S3 objects per hour prefix | S3 listing | implemented (phase 4): compaction-keeps-up + small-file share |
| §8.6 RSS under limit, goroutines flat | /metrics | implemented (phase 4) |
| §8.7 sampled UI queries | query API | implemented (phase 4): freshness + markers + optional TTL deletion |
| §8.8 no unexpected restarts | k8s API | implemented (phase 4): restart budget + replacement accounting |

Violations latch: a failure after warm-up fails the run even when the final tick looks healthy
(`doc/checker.md`). Every clause fired at least once for a real cause during the phase-4 bring-up (§5, §7):
§8.5 on the crash-recovery compaction backlog, §8.7 on a probe defect it exposed (405 on HEAD), §8.8 on genuine
OOM and panic restarts, and the scrape-gap rule on real query-API outages — none of the final latches were false
positives.

## 9. Generator headroom (plan §10)

Every step records the k6 pod's CPU share against its limit (`steps.jsonl`, `generator` field); runs stop as
`invalid` past 70%. Large-cluster runs additionally pin the runner to dedicated nodes.

> Placeholder — headroom observed at the T2/T3 ceilings, and the runner sizing that keeps it. On the local smoke
> levels the generator used 1–2% of a 2-core limit — the guard machinery is verified, the sizing question is not.

## 10. Follow-ups

- **Fix the `CallsPipeReader` panic** (`libs/parser/pipe/calls.go:70`: negative thread index, ignored mid-record
  read errors; §5) — it blocks the real-timer soak. Audit the other pipe readers for the same pattern; phase 2
  already flagged mis-framing in the suspend/params readers.
- **Global read-path memory budget** for the query service (§7): the per-request scan guard multiplies under
  concurrency; a global budget or admission semaphore is needed before T6 runs at cluster scale.
- Re-run the ceilings on the large cluster; only then replace the placeholders above.
- Decide, from the T3 failure shape, whether an accept-side connection cap is warranted (plan §7.3 note).
- Run `specs/t1-contract.yaml` and `specs/t4-soak.yaml` on the large cluster (with the checker and `k6-query`);
  only then fill §5–§7.
- Revisit the T6 profile shares (UI VUs, incident cadence) when real usage data appears (plan §7.6).
