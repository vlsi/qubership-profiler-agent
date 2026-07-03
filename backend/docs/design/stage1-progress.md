# Stage 1 progress

Stage 1 is the collector write path: ingest the agent streams, persist them in
the hot store, seal parquet, and recover after a restart (`profiler-plan.md`
Stage 1, contracts `01-write-contract.md`, `03-lifecycle.md`,
`06-wire-protocol-server.md`). This document tracks status, decisions, and open
issues, per `WORKFLOW.md` §7. A full `stage1-plan.md` task breakdown is still
pending (see open issues).

## Status

- [x] **Ingest → hot store** (first slice; branch `feat/stage1-hot-store-ingest`)
  - [x] `libs/collector/hotstore` — WAL writer/replayer (`01` §3.2–§3.3: varint framing, fsync N/T, CRC32 footer, torn-tail truncate)
  - [x] `libs/collector/hotstore` — gzip segments for `trace`/`sql`/`xml`, named `serverRollingSequenceId + 1` (`01` §4.4)
  - [x] `libs/collector/hotstore` — `metadata.sqlite` + per-bucket `calls-<bucketStart>.sqlite` call index (`03` §3.2; `parquet_local`/`seal_state` created but unused until the seal pass)
  - [x] `libs/collector/hotstore` — recovery: close open pod-restarts, replay WALs, rescan segments into the catalog and `chunk_index[threadId]`, reconcile `calls.wal` against partitions (`03` §3.3–§3.5)
  - [x] `libs/collector/ingest` — `server.Listener` routing demuxed streams: trace tee (segment append + chunk parse), calls decode → `calls.wal` + index row (`ts_ms` delta accumulation, retention class, `call.red` error flag), dictionary/params/suspend → WALs
  - [x] `libs/collector` — `oklog/run` service composing the store and the TCP listener (dumps-collector pattern)
  - [x] `libs/server` — `RestartTimeMs` stamped at TCP accept (`01` §1 V4); `PodDisconnected` callback; listener errors propagate to `ACK_ERROR_MAGIC` / null-UUID teardown (`06` §6); `Stop()` waits for connection teardown
  - [x] Synthetic integration test `libs/tests/integration/hotstore_test.go`: segment naming + pointer resolution (M7), `ts_ms` accumulation across buckets (B1), chunk index / catalog / WALs, recovery from PV alone after wiping every SQLite file
- [ ] Seal pass → parquet (`01` §5–§6)
- [ ] S3 upload + dictionary/pods/suspend snapshots (`01` §3.6, §6.2)
- [ ] Hot-read API `/internal/v1/*` (`02-read-contract.md` §3)
- [ ] Budgets and janitors: segment refcounts/eviction, idle accumulator timeout, memory budget (`01` §4.6)
- [ ] Collector app wiring: `profiler-backend collect` subcommand, readiness states, Prometheus metrics (`03` §2)

## Decisions log

### 2026-07-03 — calls.wal record body is length-prefixed JSON, not raw wire bytes

**Question:** `01` §2 describes `calls.wal` as "full Call records as received".
Should the WAL store the raw wire bytes of each record?

**Choice:** No. Each record is stored in the shared WAL framing
(varint length + body + CRC footer) with a JSON body `{ts_ms, call}` carrying
the decoded record and its absolute start time.

**Reason:** A raw wire record is not self-contained: its start time is a
zig-zag delta from the *previous* record and its thread name is an index into a
per-file table (`01` §5.1). The contract requires reading one record by offset
(hot `/calls/{pk}` fetch, seal-pass column read, recovery reconciliation), so
the stored form must decode standalone. JSON was picked over a bespoke binary
codec for the first slice: one codec, debuggable, and the WAL lives only for
the hot window, so the format can change with a version bump before Stage 2
scale tests. `params.wal` and `suspend.wal` use the same framing with JSON
bodies; `dictionary.wal` keeps the exact binary body pinned by `01` §3.2.

### 2026-07-03 — listener errors surface through the wire error path

**Question:** `06` §6 specifies `ACK_ERROR_MAGIC` + close when the `RCV_DATA`
handler fails, and null-UUID + close when the `INIT_STREAM_V2` handler fails,
but the `server.Listener` interface returned no errors, so a failing hot store
could only log and silently drop data.

**Choice:** `RegisterPod`, `RegisterStream`, and `AppendData` now return
errors; the connection handler maps them to the `06` §6 teardown responses.
Decode errors *inside* a stream (a malformed record on an otherwise healthy
connection) still only log and skip — the decoder drains the rest of the file
so the connection never stalls.

### 2026-07-03 — chunk_index stays RAM-only; recovery always rescans segments

`01` §4.3 stores chunk refs "in `chunk_index[threadId]` and in the SQLite
segment catalog", while the `03` §3.2 `segments` schema has no per-chunk rows
and `03` §3.5 rebuilds the index by re-parsing the segments. Implemented per
`03`: the catalog holds one row per segment (path, logical size, time range);
per-chunk refs live in RAM and are rebuilt by the recovery rescan. The rescan
also repopulates the catalog itself, so deleting a corrupt `metadata.sqlite`
(the `03` §3.2 step-4 repair) loses nothing — the integration test wipes every
SQLite file before recovery to pin that property.

### 2026-07-03 — WAL footer is marked by a zero-length record

`01` §3.2 pins "a single CRC32 at file footer" but not its byte encoding. A
bare 4-byte trailer is ambiguous on replay: a torn tail could parse as a
footer. The footer is therefore encoded as a zero-length record (varint `0`)
followed by the 4-byte CRC32; a zero-length record cannot occur as data, so
replay distinguishes "cleanly closed", "crash without footer", and "torn tail"
deterministically.

## Open issues

- **`stage1-plan.md` does not exist yet.** This slice was specified directly
  by the user; the remaining Stage 1 tasks (seal pass, S3, read API, budgets,
  app wiring) need a plan document with dependencies and acceptance criteria.
- **`error_flag` can race the dictionary on ingest.** The dictionary and calls
  streams decode on independent pipelines, so a Call indexed microseconds
  after its `call.red` dictionary word arrives could read a not-yet-registered
  id and store `error_flag = false`. The window exists only for the first
  errored call of a pod-restart. The seal pass re-derives from `calls.wal`
  params, so the parquet row can still be corrected there; decide in the seal
  task whether to re-resolve at seal or to sequence the pipelines.
- **`server.Service.Stop()` waits for live agent connections** and is bounded
  only by the socket read timeout (~40 s). The `03` §5.2 drain (send
  `COMMAND_CLOSE`, 5 s per-connection timeout) is not implemented yet; it
  belongs to the collector app wiring task.
- **Ingest decode errors only log.** A malformed calls/dictionary record skips
  the record; there is no metric yet. Prometheus counters land with the app
  wiring task (`01` §5.1 expects counters for dropped/truncated calls).
- **`params.wal` phrase-length quirk.** The agent's params/suspend phrase
  length includes bytes (version byte, suspend base time) that the pipe
  decoders do not subtract; single-phrase streams parse fine, which is what
  the agent produces today. Revisit only if multi-phrase params streams appear
  (would belong in the `streams/ → pipe/` consolidation, `profiler-plan.md`
  decision 8).
- **Pre-existing test failures** in `libs/parser/...` (`TestIntegration`,
  `TestParsePodDump`, `streams` suites) come from binary fixtures that are
  deliberately not committed (`WORKFLOW.md` §6); they fail identically with
  and without this slice. Worth a `t.Skip` when the fixture is absent.
