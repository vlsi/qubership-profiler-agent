# 03 — Lifecycle

> Status: **draft**, awaiting review. Process lifecycle for each subcommand of the single Go binary: startup, recovery, readiness, flush, and shutdown. The collector path carries most of the complexity because of crash recovery and chunk-level reassembly (`01-write-contract.md` §4).

## 1. Scope

The single Go binary `profiler-backend` runs in one of four modes (`profiler-plan.md`, single-binary VictoriaMetrics-style decision):

| Subcommand | k8s shape | Persistent state | Lifecycle complexity |
|---|---|---|---|
| `collect` | StatefulSet + RWO PV | dictionary WAL, chunks staging, pending parquet, `metadata.sqlite` | high (recovery, drain) |
| `query` | Deployment, stateless | — | low |
| `maintain` | Deployment or CronJob | — | low |
| `all` | dev only, single process | combination of the above on a local FS | varies |

This document defines the state machine for each. `04-storage-layout.md` covers the k8s manifests that wire these processes up.

## 2. `collect` startup state machine

```
INIT → LOADING → RECOVERY → READY → DRAINING → TERMINATING → (exit)
              ↘ FATAL (on unrecoverable corruption) → (exit)
```

| State | Readiness probe | What is happening |
|---|---|---|
| `INIT` | 503 | Process started; binary initialization. |
| `LOADING` | 503 | Mount PV; acquire `collector.lock`; open `metadata.sqlite`; bind ports but DO NOT accept agent TCP yet. |
| `RECOVERY` | 503 | Replay dictionary/params/suspend WALs; index chunks staging files; index pending parquet; finalize closed pod-restarts; re-trigger pending uploads. |
| `READY` | 200 | TCP listener accepts new agent connections; `/internal/v1/*` serves reads; flush loop runs. |
| `DRAINING` | 503 | Received SIGTERM. Marked Not-Ready so kubelet removes from headless service endpoints. Still serves in-flight requests and TCP connections. |
| `TERMINATING` | 503 | TCP listener closed; finalizing in-flight pod-restarts (emit accumulators as truncated; flush parquet; upload dictionaries; upload remaining parquet). |
| `FATAL` | 503 | An unrecoverable condition (corrupt SQLite that cannot be repaired, repeated S3 PUT failures during recovery, PV mount missing). Process exits non-zero so kubelet restarts it; alert fires. |

The readiness endpoint is `GET /internal/v1/health/ready` (separate from `/internal/v1/health/hot-window` from `02-read-contract.md` §3).

## 3. Recovery sequence (`LOADING` → `READY`)

This is the heaviest section because chunk-level reassembly (`01-write-contract.md` §4) and the dictionary cold-path lifecycle (`01-write-contract.md` §3.6) both depend on it.

### 3.1 Mount PV and acquire exclusive lock

1. Verify `/data` is mounted and writable. If not → `FATAL`.
2. Open `/data/collector.lock` with `flock(LOCK_EX | LOCK_NB)`. If another process holds it → wait up to `STARTUP_LOCK_WAIT` (default 30 s), then `FATAL`. (Misconfigured `volumeClaimTemplates` would lead to two pods racing; the lock guarantees exclusive ownership.)

### 3.2 Open metadata SQLite

3. Open `/data/metadata.sqlite`. Run schema migrations. Tables (initial set):

   - `staging_files (path PRIMARY KEY, refcount, size_bytes, created_at)` — chunks-staging refcount tracking (`01-write-contract.md` §4.4).
   - `parquet_local (path PRIMARY KEY, time_bucket_end_ms, retention_class, pod_restart, uploaded_at NULL, file_size, row_count, time_min_ms, time_max_ms)` — every locally-held parquet file. `uploaded_at NULL` means pending upload.
   - `pod_restarts (pod_restart PRIMARY KEY, namespace, service, pod_name, restart_time_ms, opened_at, closed_at NULL, dict_uploaded_at NULL)`.

4. Self-check: integrity check; on corruption → repair (delete file, rebuild from PV contents — costly but recoverable). If repair fails → `FATAL`.

### 3.3 Determine which pod-restarts are closed

Because the collector crashed (or was killed), every TCP connection has been broken. Agents have reconnected — likely to a different replica — and started fresh pod-restarts there with new `restart_time_ms`. Therefore:

**All pod-restarts in `pod_restarts` table with `closed_at IS NULL` are now closed.** Update each: `closed_at = now()`.

5. The collector's job is to finalize these closed pod-restarts (steps 3.4–3.7) and the pending parquet uploads (step 3.8), then begin serving.

### 3.4 Replay dictionary, params, suspend WALs

For each closed pod-restart with WAL files on PV:

6. Open `dictionary.wal`. Read length-prefixed records (`01-write-contract.md` §3.2). Reconstruct the in-memory dictionary. Stop at first structurally invalid record; truncate file at that point (this is the standard WAL tail-corruption recovery).
7. Same for `params.wal`, `suspend.wal`.

If a WAL is missing (e.g. crash between TCP accept and first dictionary entry), the pod-restart is recorded with an empty dictionary — any blobs from its chunks will be uninterpretable, but the Call rows themselves still have resolved method names (resolution happens at write time; `01-write-contract.md` §5.1).

### 3.5 Index chunks staging files

For each closed pod-restart:

8. Walk `chunks/*.bin`. For each file, walk chunks by parsing the 16-byte header `[threadId, startTime]`, recording `(staging_file, offset_in_file, chunk_length, threadId)` in memory.
9. Build per-thread inventories: `inventory[threadId] = ordered list of (staging_file, offset, length)`.

### 3.6 Index pending parquet files

10. For each parquet file in `parquet-pending/` not present in `parquet_local` (e.g. SQLite was repaired or the file was added since): open footer, read `(time_min, time_max, row_count, retention_class, pod_restart)` from the schema's columns, insert into `parquet_local`.
11. For each pending parquet (`uploaded_at IS NULL`), enqueue for upload retry (step 3.8).

### 3.7 Finalize closed pod-restarts: emit truncated blobs

The trace bytes captured in chunks staging belong to root calls that the agent **may or may not have closed** before the TCP connection broke. The collector cannot tell the difference (the agent's `Call` record for those calls never arrived). Conservative finalization:

For each closed pod-restart, for each thread T in `inventory[T]`:

12. Emit one truncated blob per thread, concatenating ALL chunks in `inventory[T]` (the same chunk-level memcpy as the normal write path).
13. Write a parquet row with:
    - PK fields derived from the chunks (`pod_restart_*`, `thread_id`, and `trace_file_index = 0`, `buffer_offset = 0`, `record_index = 0` as a placeholder).
    - `truncated_reason = "shutdown"`.
    - Other columns NULL or zero (no `Call` metadata available).
    - `retention_class = "corrupted"` (truncated calls are forensic-only).
14. Decrement chunks-staging refcounts; record blob in parquet writer state.

Cost trade-off: this loses metric fidelity (we have only the chunk bytes, not the agent's per-call summaries), but preserves the trace bytes that did make it across the wire. Dedup by PK (`02-read-contract.md` §6) collapses any duplicate rows produced by recovery vs. normal write.

### 3.8 Re-trigger pending parquet uploads

15. For each row in `parquet_local` with `uploaded_at IS NULL`, schedule an upload job in the upload worker pool. Idempotent because the S3 key is deterministic (`01-write-contract.md` §7).
16. After each upload completes: set `uploaded_at = now()`; decrement chunks-staging refcounts for chunks referenced by that file.

This step runs asynchronously; `READY` does not wait for it.

### 3.9 Upload dictionaries for closed pod-restarts

17. For each closed pod-restart whose `dict_uploaded_at IS NULL`, upload the dictionary snapshot to S3 per `01-write-contract.md` §3.6. Set `dict_uploaded_at = now()`.
18. Once dictionary is uploaded AND all that pod-restart's parquet rows are uploaded (steps 3.7+3.8 complete) AND the upload-hold-back grace has elapsed (default 1 h), delete the local WAL files for that pod-restart.

Steps 3.8 and 3.9 are background tasks; `READY` is reached after step 3.7 completes.

### 3.10 Become READY

After 3.1–3.7 finish:

19. Bind the TCP listener for agent connections (`PROFILER_AGENT_PORT`, default `1715`).
20. Bind the internal HTTP listener for `/internal/v1/*` (`PROFILER_INTERNAL_API_PORT`, default `8081`).
21. Start the flush loop, the chunks-staging janitor, the parquet upload worker pool, and the hot-retention janitor.
22. Flip `/internal/v1/health/ready` to 200.

Expected duration of 3.1–3.7 on a healthy PV: seconds to tens of seconds. Dominated by step 3.5 (chunks file walk) when the PV holds gigabytes of staging.

## 4. Readiness probe semantics

`GET /internal/v1/health/ready`:

- `200 OK { "state": "READY" }` once step 3.10 completes.
- `503 Service Unavailable { "state": "INIT"|"LOADING"|"RECOVERY"|"DRAINING"|"TERMINATING"|"FATAL", "details": "..." }` otherwise.

The state name is for kubelet logs and human debugging; kubelet only cares about the HTTP code.

`/internal/v1/health/live` (liveness):

- `200 OK` while the process is healthy enough to keep running.
- `503` only if a deadlock, OOM-imminent, or repeated fatal errors are detected. Most failures should fail readiness, not liveness — liveness failure causes a kubelet kill, which is more disruptive.

Headless service relies on readiness — non-ready pods do not appear in DNS A-records (`02-read-contract.md` §7.1), so `query` won't fan out to them.

## 5. Shutdown sequence (`READY` → exit)

Triggered by SIGTERM (kubelet drain) or SIGINT (operator).

### 5.1 Drain phase (`DRAINING`)

1. Flip readiness to 503 with `state: "DRAINING"`.
2. **Do not close TCP listener yet.** Wait `PROFILER_SHUTDOWN_DRAIN_GRACE` (default 30 s) so kubelet removes the pod from the headless service's endpoints. `query`'s next DNS resolution skips this replica.
3. During this grace period, the collector continues to:
   - Accept new agent TCP connections (sticky-TCP routing isn't aware of readiness; new agents may still land here).
   - Serve `/internal/v1/*` reads for any in-flight `query` requests already routing to this replica.
   - Flush parquet on the normal schedule.

### 5.2 Stop new connections (`DRAINING` → `TERMINATING`)

4. After the drain grace, close the TCP listener (no new agent connections accepted).
5. For each active agent TCP connection, send `COMMAND_CLOSE` (`backend/libs/protocol/commands.go`); wait for the agent's acknowledgement up to `PROFILER_AGENT_CLOSE_TIMEOUT` (default 5 s). If timeout → close from collector side.
6. The affected agents will reconnect — to a different collector replica (this one is not in DNS anymore) — and start a fresh pod-restart there. The current pod-restart on this replica is now closed.

### 5.3 Finalize closed pod-restarts

7. For each pod-restart closed in 5.2:
   - Force-emit per-thread accumulators as truncated blobs with `truncated_reason = "shutdown"`. (Same as recovery step 3.7.)
   - Close all open parquet writers; trigger upload.
   - Upload dictionary snapshot to S3 (`01-write-contract.md` §3.6).
8. Wait for all pending uploads to complete, bounded by `PROFILER_SHUTDOWN_UPLOAD_TIMEOUT` (default 60 s). Uploads that don't complete: leave parquet on PV (next collector start will retry; `metadata.sqlite` carries the state).

### 5.4 Final cleanup

9. Close `metadata.sqlite`.
10. Release `collector.lock`.
11. Exit 0.

Total shutdown budget = `SHUTDOWN_DRAIN_GRACE + AGENT_CLOSE_TIMEOUT + SHUTDOWN_UPLOAD_TIMEOUT` = `30 + 5 + 60` = ~95 s. k8s `terminationGracePeriodSeconds` should be at least this (see `04-storage-layout.md`).

## 6. Flush triggers (reference)

The collector's flush loop is defined in `01-write-contract.md` §6.1. Summary:

- **Time bucket end + grace** (default 5 min + 30 s).
- **Parquet size exceeded** (default 64 MB).
- **Memory pressure** (collector exceeds `mem_budget`).

The flush loop runs only in `READY`. During `DRAINING`, the normal flush continues; during `TERMINATING`, the flush is forced for ALL open writers regardless of triggers.

## 7. `query` subcommand lifecycle

Stateless. Simple state machine:

```
INIT → LOADING → READY → TERMINATING → exit
```

### 7.1 Startup

1. Parse config; resolve `COLLECTOR_HEADLESS_SVC` once at boot to verify it resolves at all (warn but don't fail if it doesn't — collectors may come up later).
2. Connect to S3 endpoint; verify bucket access. If unrecoverable → `FATAL`.
3. Bind external API listener (`PROFILER_EXTERNAL_API_PORT`, default `8080`).
4. Flip readiness to 200.

### 7.2 Per-request

DNS is re-resolved on every external request (`02-read-contract.md` §7.1). No caching at the process level.

### 7.3 Shutdown

1. SIGTERM: flip readiness to 503, wait `PROFILER_SHUTDOWN_DRAIN_GRACE` (same default 30 s) for kubelet to remove from service endpoints.
2. Stop accepting new HTTP requests.
3. Allow in-flight requests to complete, bounded by 15 s.
4. Exit 0.

## 8. `maintain` subcommand lifecycle

Stateless. Two run modes:

### 8.1 Cron mode (`profiler-backend maintain --cron`)

Process runs continuously; uses `backend/libs/cron/` to schedule jobs. Same simple startup as `query`. Shutdown closes the cron scheduler and exits.

### 8.2 One-shot mode (`profiler-backend maintain --run-now`)

Process runs the scheduled jobs once and exits. Used by k8s CronJob if we prefer one-pod-per-run over a long-running scheduler. Decision deferred to `04-storage-layout.md`.

## 9. `all` subcommand lifecycle (dev only)

In-process composition of `collect` + `query` + `maintain`. Used for dev (`profiler-plan.md` decision). Lifecycle is the union:

1. Filesystem-emulated S3 (`backend/libs/s3/` filesystem emulator, deferred — currently MinIO in docker-compose; both supported as dev variants).
2. Local `/data` directory, no PV semantics.
3. Combined startup runs all three subcommands in goroutines under one `oklog/run` group.
4. Shutdown sends one signal; each subcommand drains in its own grace period (collector's longest), then process exits.

The `all` mode is documented as **dev-only**; production uses three separate k8s workloads. Detection: refuse to start in `all` mode if `KUBERNETES_SERVICE_HOST` is set (`PROFILER_ALLOW_K8S_ALL_MODE=true` override exists for k8s-based dev clusters).

## 10. Configuration

### `collect`

| Env | Default | Description |
|---|---|---|
| `PROFILER_AGENT_PORT` | `1715` | TCP listener for agent connections. |
| `PROFILER_INTERNAL_API_PORT` | `8081` | HTTP listener for `/internal/v1/*`. |
| `PROFILER_STARTUP_LOCK_WAIT` | `30s` | How long to wait for `collector.lock` before `FATAL`. |
| `PROFILER_SHUTDOWN_DRAIN_GRACE` | `30s` | DRAINING phase: wait for kubelet endpoint removal (§5.1). |
| `PROFILER_AGENT_CLOSE_TIMEOUT` | `5s` | Per-connection drain timeout (§5.2). |
| `PROFILER_SHUTDOWN_UPLOAD_TIMEOUT` | `60s` | Bound on flushing pending uploads at shutdown (§5.3). |

### `query`

| Env | Default | Description |
|---|---|---|
| `PROFILER_EXTERNAL_API_PORT` | `8080` | HTTP listener for `/api/v1/*`. |
| `COLLECTOR_HEADLESS_SVC` | — | Headless service DNS for collector discovery (`02-read-contract.md` §7.1). |
| `PROFILER_SHUTDOWN_DRAIN_GRACE` | `30s` | Same semantics as `collect`. |

### `maintain`

| Env | Default | Description |
|---|---|---|
| `PROFILER_MAINTAIN_INTERVAL` | `1h` | Cron-mode scheduling interval. |
| (retention TTLs) | (see `01-write-contract.md` §9) | Per retention class. |

### `all`

| Env | Default | Description |
|---|---|---|
| `PROFILER_ALLOW_K8S_ALL_MODE` | `false` | Override to allow `all` mode under k8s (dev cluster); refuses by default. |
| (all of the above) | as above | Subcommands inherit their respective env vars. |

## 11. What this contract does NOT cover

- **k8s manifests** (StatefulSet, Headless Service, PVC templates, probe wiring) → `04-storage-layout.md`.
- **Agent reconnection behaviour** (timing, jitter, backoff) — set by the agent, not the collector. Out of scope here.
- **maintenance job specifics** (compaction algorithms, retention enforcement loop) — covered briefly in `profiler-plan.md`, detailed when Stage 4 begins.
- **Backpressure to the agent** when collector is overloaded — currently the protocol has no backpressure signal; covered as a future C-track item in `profiler-plan.md` (C2 runtime config).

## 12. Review checklist

- [x] State machine names (`INIT`/`LOADING`/`RECOVERY`/`READY`/`DRAINING`/`TERMINATING`/`FATAL`) — accepted.
- [x] Recovery decision to mark ALL on-disk pod-restarts as closed (§3.3) — accepted.
- [x] Truncated-blob emission strategy for recovery (§3.7) — accepted.
- [x] Shutdown budget total (~95 s) — accepted; `terminationGracePeriodSeconds` will be set to at least 100 s in `04-storage-layout.md`.
- [x] `liveness` vs `readiness` split (§4) — accepted.
- [x] Cron vs one-shot for `maintain` (§8) — accepted; ship BOTH modes (`--cron` for long-running deployment, `--run-now` for k8s CronJob). Operator picks per environment.
- [x] `all`-mode design (§9) — accepted; current refusal-under-k8s + env override stays as-is.
