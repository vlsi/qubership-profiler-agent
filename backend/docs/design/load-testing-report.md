# Load-testing report: Go profiler backend

Status: **draft — harness validated on the local stand; final numbers await the large cluster**
(`load-testing-plan.md` §5.2). Owner: @vlsi.

Every number in this report must cite its run: the artifact directory (`runs/<ts>-<name>/`, kept outside the
repository), the image digests, and the frozen run spec inside it. A number without a run citation is a placeholder,
not a result. Numbers from the local stand (OrbStack) validate the harness and are marked *(local, not a ceiling)* —
they must never be quoted as collector limits.

## 1. Scope and method

This report covers the ceiling campaign (plan §9.3): the throughput ceiling (T2, plan §7.2) and the connection-count
ceiling (T3, plan §7.3) of a single collector replica, measured with:

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

## 5. Generator headroom (plan §10)

Every step records the k6 pod's CPU share against its limit (`steps.jsonl`, `generator` field); runs stop as
`invalid` past 70%. Large-cluster runs additionally pin the runner to dedicated nodes.

> Placeholder — headroom observed at the T2/T3 ceilings, and the runner sizing that keeps it. On the local smoke
> levels the generator used 1–2% of a 2-core limit — the guard machinery is verified, the sizing question is not.

## 6. Follow-ups

- Re-run the ceilings on the large cluster; only then replace the placeholders above.
- Decide, from the T3 failure shape, whether an accept-side connection cap is warranted (plan §7.3 note).
