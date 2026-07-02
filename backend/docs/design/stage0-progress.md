# Stage 0 progress

Stage 0 is the contract-and-diagrams phase. No service code is written until these documents are reviewed and merged.

## Status

**Stage 0 — drafted; awaiting review for merge.**

- [x] **01 — Write contract** (`01-write-contract.md`)
  - [x] Wire-protocol invariants verified against agent code (V1–V6)
  - [x] Dictionary WAL format on local PV + S3 dictionary snapshot lifecycle
  - [x] Per-call trace blob assembly at chunk granularity (the (A) decision)
  - [x] Raw chunks staging files on PV with refcount tracking
  - [x] Parquet schema (Call rows) with retention class + error flag columns
  - [x] Local PV directory layout
  - [x] S3 object key layout (per retention class, time-bucketed)
  - [x] Flush semantics (time / size / memory pressure)
  - [x] Hot retention of local parquet past S3 upload
  - [x] Retention class mapping (5 classes by `(duration, error_flag)`)
- [x] **02 — Read contract** (`02-read-contract.md`)
  - [x] External `/api/v1/*` endpoints (fresh design, Java collector API not preserved)
  - [x] Internal `/internal/v1/*` collector hot-read endpoints
  - [x] `/calls/{pk}/tree` endpoint — MessagePack with int-keyed maps + `v` version envelope
  - [x] `/pods/{pod-restart}/dictionary` endpoint
  - [x] Hot/cold model with `hot_retention + overlap_margin` window
  - [x] S3 LIST-based discovery, no manifest yet
  - [x] PK-based deduplication, always-on
  - [x] Fan-out with partial-result protocol
- [x] **03 — Lifecycle** (`03-lifecycle.md`)
  - [x] Collector startup state machine (`INIT/LOADING/RECOVERY/READY/DRAINING/TERMINATING/FATAL`)
  - [x] Recovery sequence (mount PV → SQLite → WAL replay → chunks index → finalize closed pod-restarts → re-attempt uploads)
  - [x] Readiness / liveness probe split
  - [x] Shutdown drain (~95 s budget)
  - [x] `query` / `maintain` / `all` lifecycle
- [x] **04 — Storage layout** (`04-storage-layout.md`)
  - [x] StatefulSet manifest with `volumeClaimTemplates` (RWO)
  - [x] Headless Service (`clusterIP: None`)
  - [x] `query` Deployment + ClusterIP Service
  - [x] `maintain` with two modes (long-running Deployment OR k8s CronJob, operator-toggled)
  - [x] Helm chart structure and values diff against current `profiler-stack`
  - [x] Resource defaults
  - [x] Single Docker image with subcommand args
- [x] **05 — Diagrams** (`05-diagrams.md`)
  - [x] Data flow
  - [x] Deployment topology
  - [x] Collector state machine
  - [x] Per-call write-side lifecycle (sequence)
  - [x] Hot/cold read flow (sequence)
  - [x] Artifact lifetime table (initially a Mermaid `gantt`, replaced with a table because mixed time scales rendered poorly)

## Other Stage 0 artifacts

- `deferred.md` — design ideas intentionally out of MVP scope, with re-visit triggers documented.
- `profiler-plan.md` — high-level roadmap, decisions, and Idea→code map.
- `profiler-mom.md` — meeting notes that seeded the plan.
- `profiler-plan-summary.md` — early code-vs-plan delta.

## Stage 0 → Stage 1 readiness

Items below are explicitly OPEN at the end of Stage 0 and tracked into Stage 1+:

- **`error_flag` from `isCallRed`** — wire-format presence to be verified during Stage 1b; until then `error_flag` is derived from `callInfo.isCorrupted` only (`01-write-contract.md` §5.6).
- **Filesystem-backed S3 emulator** (`backend/libs/s3/`) — implementation deferred; dev currently uses MinIO in docker-compose.
- **`/internal/v1/pods` targeting** — endpoint shipped in Stage 1b but kept dormant in `query` until cluster size justifies it.
- **`stats` endpoint full schema** — sketch only in MVP; full design at Stage 4.
- **Parser consolidation** `streams/ → pipe/` (`profiler-plan.md` decision 8) — execute opportunistically while touching protocol code.

These are not blockers for Stage 1 to begin.

## Decisions log

Append-only log of decisions taken during Stage 0. Each entry has a date, the question, the choice, and the reason.

### 2026-04-23 — language for the new collector

**Question:** Java/Kotlin or Go for the new write-path code?

**Choice:** Go.

**Reason:** The agent protocol parser (`libs/parser/`), parquet writer (`libs/storage/parquet/`), and S3 abstraction (`libs/s3/`) are already implemented in Go. `dumps-collector` (Go) is a working template for the runtime shape (`oklog/run` + cobra + SQLite + HTTP). The team prefers a single Go binary in the VictoriaMetrics style. Going with Java would mean a long-term bilingual codebase for marginal short-term savings.

**Consequence:** The existing Java collector (`backend/apps/collector/`) becomes legacy. It is not modified during Stage 1; once the new collector is in place (after Stage 4), it is deprecated and removed.

### 2026-04-23 — parquet schema is up for redesign

**Question:** Reuse `CallParquet` from `backend/libs/storage/parquet/calls.go` verbatim?

**Choice:** No. Use it as a reference, but redesign with explicit justification per column.

**Reason:** No design documentation exists for the current schema. There is no migration requirement (no production data to preserve) and no external client locked to it.

**Consequence:** Stage 0 produces a new schema in `01-write-contract.md` §5.2.

### 2026-04-23 — JSON for inter-service traffic

**Question:** JSON or protobuf between query and collector replicas?

**Choice:** JSON for `/internal/v1/*` and most `/api/v1/*`. MessagePack with int-keyed maps for `/api/v1/calls/{pk}/tree` only (size-driven; see 2026-06-18 entry below).

**Reason:** Easy to debug with curl, no schema-compiler in the build, expected payload sizes (calls list responses) are well within JSON's comfortable range.

### 2026-04-23 — S3 discovery via LIST, no manifest yet

**Question:** Maintain a manifest file in S3 for time-range → parquet-file lookups?

**Choice:** Start with LIST by prefix. Add manifest later if LIST becomes slow.

### 2026-04-23 — MinIO in docker-compose for dev

**Question:** Run a real S3 (MinIO) in dev or build a filesystem-backed emulator?

**Choice:** MinIO in docker-compose primary; the filesystem emulator stays as a deferred option for unit tests.

### 2026-06-18 — single Go binary, VictoriaMetrics-style subcommands

**Question:** Three separate Go binaries (collect / query / maintain), or one binary with subcommands?

**Choice:** One binary, four subcommands: `collect`, `query`, `maintain`, `all` (dev-only).

**Reason:** Shared internal libraries (config, S3 client, metrics, lifecycle) live in one tree; integration tests can run `all` in-process; one Docker image to ship.

**Consequence:** k8s manifests use the same image with different `args` for each workload (`04-storage-layout.md` §2-§5).

### 2026-06-18 — V5 corrected: trace stream is chunk-interleaved

**Question:** Are trace bytes for one root call emitted contiguously?

**Choice:** No — corrected. The trace stream is a sequence of `LocalBuffer` chunks (~4096 events each) with `[threadId, startTime]` headers; chunks from different threads interleave.

**Reason:** Verified by reading `dumper/.../Dumper.java:820-1010` and `boot/.../LocalBuffer.java`. The agent does not emit one contiguous byte range per call.

**Consequence:** Major write-contract rework. The collector reassembles per-call blobs at chunk granularity at write time (decision below).

### 2026-06-18 — per-call reassembly at chunk granularity ((A) chunk-level)

**Question:** Given the V5 correction, how does the collector serve per-call trace bytes? Options were: (A) physically reassemble per-call blob at write time, (B) read-side reassembly from raw chunks, (C) write-side index, read-side fetch.

**Choice:** (A) — write-side reassembly at chunk granularity. Per-call blob stored directly in the parquet row's `trace_blob` column.

**Reason:** Per-call retention sharding is trivial (each blob lives in one retention-class bucket); reassembly cost is bounded chunk-level memcpy (no event-level parsing); read API is dumb (single blob fetch).

**Trade-off accepted:** Carry-over chunk on root-call boundary is duplicated into both blobs (~16 KB per pair of consecutive same-thread root calls). Negligible.

**Consequence:** `01-write-contract.md` §4 fully rewritten; in-memory accumulator state holds `(staging_file, offset, length)` per thread.

### 2026-06-18 — hot/cold model with overlap window

**Question:** How does query reconcile data that's in flight on collector with data that's already flushed to S3?

**Choice:** Hot tier (collector PV) retains parquet for `PROFILER_HOT_RETENTION` (default 15 min) past S3 upload. Query reads both tiers for an `overlap_margin` (default 5 min) window, dedupes by PK. Standard Prometheus / VictoriaMetrics / Loki pattern.

**Consequence:** `01-write-contract.md` §6.3, `02-read-contract.md` §4.

### 2026-06-18 — retention classes (5 classes by duration × error)

**Question:** How is per-call retention sharded?

**Choice:** 5 retention classes: `short_clean` (<100ms, no error, 1d), `normal_clean` (<1s, 7d), `long_clean` (≥1s, 30d), `any_error` (30d), `corrupted` (forensic, 7d). Each class is its own S3 path segment so retention is enforced by S3 LIST + DELETE without opening parquet.

**Consequence:** `01-write-contract.md` §6.4; per-class TTL env vars in §9.

### 2026-06-18 — dictionary snapshot uploaded to S3 on pod-restart close

**Question:** How is the dictionary kept available for blob decoding once a pod-restart's WAL is purged from the collector?

**Choice:** On pod-restart close (TCP termination + finalization), the collector serializes the final dictionary as JSON and uploads it to S3 at `dictionaries/v1/<yyyy>/<mm>/<dd>/<podRestartHash>.json`. Default TTL 35 days (exceeds longest parquet retention).

**Reason:** Without this, `trace_blob` columns in long-retention parquet become unreadable after the WAL is purged. Surfaced during read-contract design (`02-read-contract.md` §2.6 discussion).

**Consequence:** `01-write-contract.md` §3.5 + §3.6; `02-read-contract.md` §2.6.

### 2026-06-18 — `trace_id` as three INT32 columns

**Question:** Carry the trace pointer as a single string `"a_b_c"` or split into three integers?

**Choice:** Split: `trace_file_index`, `buffer_offset`, `record_index` as separate INT32 columns.

**Reason:** Compact storage, dictionary encoding works per-column, dedup on query side is cheaper.

### 2026-06-18 — `/tree` endpoint format: MessagePack with int-keyed maps + version envelope

**Question:** What format does the server-decoded tree endpoint return — JSON, MessagePack, Protobuf, FlatBuffers?

**Choice:** MessagePack with int-keyed maps (semantically equivalent to protobuf field tags) and a `v: 1` envelope.

**Reason:** Compact binary (~2-3× smaller than JSON), zero codegen / schema-file overhead, hand-written ~50-80 LOC decoders per language. Scales to MCP/CLI within the same team. Future migration to protobuf is mechanical because int-keyed maps map 1:1 to field tags.

**Trade-off accepted:** Manual code review enforces "append-only / reserved-numbers" conventions instead of compiler-checked schema evolution.

**Consequence:** `02-read-contract.md` §2.5 (full field-tag tables and versioning rules).

### 2026-06-18 — always dedup, do not trust sticky TCP

**Question:** Does query rely on sticky TCP to avoid duplicate Call records, or always dedup by PK?

**Choice:** Always dedup. Sticky TCP routing breaks down during replica failover, scale events, and the hot/cold overlap window.

**Consequence:** `02-read-contract.md` §6.

### 2026-06-18 — Java collector API not preserved

**Question:** Does the new `query` service preserve the legacy Java collector's `/cdt/v2/calls/...` endpoint shape?

**Choice:** No. Fresh design at `/api/v1/*`. The legacy UI is rewired in Stage 5 anyway, and there are no external production consumers.

**Reason:** This supersedes the 2026-04-23 "preserve external query API shape" entry, which was a conservative initial assumption.

**Consequence:** `02-read-contract.md` §1 explicitly notes the override.

### 2026-06-18 — `cutoff=strict` query parameter deferred

**Question:** Ship a `?cutoff=strict` mode that disables the hot/cold overlap and dedup?

**Choice:** Defer. No identified MVP consumer that warrants the API complexity.

**Consequence:** Recorded in `deferred.md`. Trivial to add later when a concrete consumer profiles dedup as a bottleneck.

### 2026-06-18 — server-decoded `/tree` is the canonical path, raw blob is secondary

**Question:** Should UI fetch the raw blob + dictionary and decode client-side, or fetch a server-decoded tree?

**Choice:** Server-decoded `/tree` endpoint is canonical (`02-read-contract.md` §2.5). Raw `/trace` + `/dictionary` remain as a secondary advanced path for tooling that wants the wire format.

**Reason:** Naive nested-JSON inflates blob 5-10×; the savings of raw bytes do not justify forcing every client to embed a wire-format decoder. MessagePack + int-keyed maps reaches the size sweet spot.

**Consequence:** `02-read-contract.md` §2.5.

### 2026-06-18 — internal `/internal/v1/pods` targeting endpoint shipped but dormant

**Question:** Implement query-side replica targeting via `/internal/v1/pods` in MVP?

**Choice:** Ship the collector endpoint in Stage 1b (~30 LOC). Keep the query-side optimization off by default; turn on when cluster size warrants.

**Consequence:** `02-read-contract.md` §7.3.

### 2026-06-18 — agent wire protocol unchanged

**Question:** Does this contract require any change to the agent's TCP wire protocol?

**Choice:** No. Every decision above is collector-side. Existing agents ship to the new backend with no modifications.

**Reason:** The `restartTime` is stamped by the collector at TCP accept; the V5 correction is handled by collector-side reassembly; the dictionary snapshot is built by the collector from its WAL.

**Consequence:** Stage 1 work plan does not include the `agent/` subtree.

### 2026-07-01 — code re-review corrections (batch)

A re-review against the agent code and the existing Go parser surfaced several errors in the Stage 0 contracts. Each contract carries a dated amendment block at its top; the decisions are logged here.

**Framing levels.** `COMMAND_RCV_DATA` payload (1 KB, `ProtocolConst.DATA_BUFFER_SIZE`) ≠ logical `LocalBuffer` trace chunk (tens of KB) ≠ Go `Chunk` rolling-stream handle. One logical chunk spans many payloads; chunk boundaries have no length prefix and are found only by parsing events to `EVENT_FINISH_RECORD`. Recorded in `backend/CLAUDE.md`.

**Blob assembly.** Event-level and Call-driven, keyed by the Call pointer `(trace_file_index, buffer_offset, record_index)`, with depth tracking to the depth-0 exit. Not a chunk-memcpy and not accumulator-by-arrival. The trace-vs-calls stream ordering worry is dissolved by this model.

**Canonical stored form: raw trace.** In scope for decodability: the dictionary, the external value streams the trace references (`bigParams` / `bigParamsDedup`, and any `sql` / `xml`), and the trace stream's `timerStartTime` epoch.

**Hot store: gzip segments + SQLite index.** Raw trace stream → rotating multi-MB gzip segment files on the PV (one member per segment, the agent's `<seq>.gz` model); 1× write, `unlink` eviction. SQLite holds the call index, segment catalog, refcounts, upload checkpoints — no bulk bytes. Parquet is the derived immutable cold form; the hot store is a cache over parquet + S3. Supersedes "read from open parquet writers" and the SQLite-blob variant (rejected because WAL doubles bulk writes).

**Bucketing by `floor(ts_ms)`.** Not by processing time. Late calls append files to older buckets; discovery is by range overlap; the `maintain` job compacts small files. Compaction moves from "deferred" to a required MVP task; the S3 catalog/manifest stays deferred as long as compaction keeps the file count low.

**Sealed parquet is immutable.** Sealed once, never rewritten → the deterministic S3 key is genuinely idempotent; no content-hash key needed.

**Recovery drops unclosed calls.** No truncated-blob emission; no `(0,0,0)` placeholder PK; `thread_id` is not in the PK.

**Retention classes.** `corrupted` and `any_error` are not mutually exclusive; a row may be both. The storage bucket per row stays single (route to `corrupted`).

**Overload policy.** Evict oldest segments first and drop fast calls first (class-aware); degrade by dropping the trace body while keeping Call metadata; never reach PV `ENOSPC`.

**No secondary index / no auth in the MVP.** `method` / `params` / `/stats` accept a full scan over the range. `Cache-Control: public` on blob and dictionary is fine until auth lands.

**Target scale.** 100 pods, 100 K calls/s aggregate, `/tree` cold-read SLA 10 s.

**Open spikes carried into Stage 1.**

- Capacity: segment size, seal interval, and staging budget for the target scale; the raw-`trace`-bytes-per-call figure needed to size them.

(The cursor / stable-pagination spike is resolved — see the 2026-07-01 cursor entry below. The "which external value streams does the trace reference" spike is resolved — see the metadata-schema entry below.)

### 2026-07-01 — dictionary continuity on cross-replica reconnect (resolved)

**Question:** On reconnect to a different collector replica, does the agent re-send the full dictionary, or must replicas share dictionary state?

**Finding:** The agent re-sends the full dictionary; replicas need not share state. A dropped connection cannot heal in place: `DefaultCollectorClient` sets `needsReconnect` and throws on any failed write, flush, or ack, and never reconnects itself (`DefaultCollectorClient.java:305,319,452`). The exception unwinds `dumpLoop()` into the `DumperThread` incarnation loop (`DumperThread.java:59-108`), which closes the dumper and calls `initialize()` again. `initialize()` resets `lastWrittenDictionaryTag = 0` and builds a fresh client and socket (`Dumper.java:454,348`), so the next `dumpDictionary()` walks the dictionary from index 0 (`Dumper.java:654,1258`), and the first dictionary chunk carries `resetRequired = 1` because `resetExistingContents()` is true while the tag is 0 (`Dumper.java:271-285`).

**Why the new replica is never dictionary-less:** `restartTime` is stamped by the collector at TCP accept, not sent by the agent (§1 V4). Every reconnect is a new TCP accept, so the new replica sees a fresh `(namespace, service, pod, restartTime)` pod-restart it has not indexed, and the agent opens that connection by sending the whole dictionary with the reset flag. The old replica finalises its own pod-restart independently (`03-lifecycle.md` §3.7).

**Consequence:** No cross-replica dictionary sharing in the MVP. Continuity keys off the dictionary stream's rolling-sequence id and reset flag, as already stated in the contract. This rests on a static code trace; an optional `mock-collector` check — break the socket, then assert `resetRequired = 1` and a from-zero dictionary on the second connection — can confirm it empirically.

### 2026-07-01 — agent-side improvements deferred (out of git)

Two protocol improvements would simplify the collector but are not required for the MVP. Captured in an out-of-git working file (`backend/docs/design/future-improvements.local.md`), not the tracked contracts:

- Agent emits both start and end trace offsets in the Call record, so the collector need not parse events to find a call's end.
- Remote protocol compresses the trace payload while leaving framing uncompressed, so the collector can demux without a channel-wide gunzip and store trace compressed as received.

### 2026-07-01 — parquet materialized by a seal pass, not on the write path

**Question:** With the hot store now gzip segments + SQLite index (2026-07-01 batch above), should the collector keep writing parquet incrementally per Call record, or materialize it from the hot store by a batch seal pass?

**Choice:** Seal pass. The write path only appends trace segments, builds the per-thread `chunk_index`, appends Call records to `calls.wal`, and inserts a lightweight SQLite index row. A per-`(pod-restart, bucket)` seal pass assembles the per-call blobs in one segment-ordered walk (each segment decompressed once) and materializes the parquet.

**Reason:** Hot reads already bypass open parquet writers (`02-read-contract.md` §3), so incremental parquet bought nothing on the hot path while costing a large open-writer fan-out (pod-restart × bucket × retention class) and a second copy of the trace bytes (once in the gzip segments, once in the pending-parquet blob). The seal pass cuts open-writer memory, removes the double store, folds late-data patch files and compaction into one "re-materialize the bucket" operation, and simplifies recovery — no half-written parquet, just re-run the idempotent seal.

**Trade-off accepted:** Call metadata for the un-sealed window lives in `calls.wal` + SQLite rather than in compact pending parquet, and blob assembly buffers the long-call tail across the segment cursor. Both are bounded (overload policy) and sized in Stage 1.

### 2026-07-01 — metadata.sqlite schema and hot-store layout

**Question:** What tables does `metadata.sqlite` hold, how is the decoded call index stored and rotated, and which streams form the hot store?

**Choices.**

- **Central `metadata.sqlite` (one per replica):** `pod_restarts`, `segments` (catalog + refcount), `parquet_local` (sealed files + upload checkpoints), `seal_state` (watermarks), `call_partitions` (partition catalog). No bulk stream bytes. Rewritten into `03-lifecycle.md` §3.2 (supersedes the old `staging_files` / `parquet_local` / `pod_restarts` set).
- **Call index partitioned by time bucket.** The decoded `calls` stream lands in `call_index` rows held in one SQLite file per bucket (`calls-<bucket>.sqlite`), ATTACHed for reads and dropped wholesale past `hot_retention`. A partition-drop avoids the `DELETE`-churn + `VACUUM` of a single ~9M-row table at target scale. The calls stream is consumed sequentially (not offset-addressed), so decoding to rows fits; `trace` / `sql` / `xml` stay raw because they are offset-addressed.
- **Hot-store segment catalog covers `trace`, `sql`, `xml` only.** `dictionary` / `params` / `suspend` stay as append-only WAL files (§3), because the dictionary needs per-entry `fsync` durability — one lost entry makes every trace byte that references it undecodable.
- **Segment = agent stream file, 1:1.** A `segments` row is keyed by `(pod_restart, stream, rolling_seq)`, where `rolling_seq` is the agent's file index. A Call pointer `(trace_file_index, buffer_offset)` and a trace tag's `(rolling_seq, offset)` then resolve by opening `<stream>/<rolling_seq>.gz` and seeking — no offset-translation table. The collector governs segment size via `requiredRotationSize` in the `INIT_STREAM_V2` response, so it does not split agent files. Supersedes the earlier "collector rotates at its own size, catalog stores a logical byte range" wording in `01-write-contract.md` §4.4, which conflicted with §5.2 ("offset within `trace_file_index`").

**Spike resolved — external value streams.** Exactly two: `xml` = `PARAM_BIG` (1), `sql` = `PARAM_BIG_DEDUP` (3). A trace tag of either type carries `(rolling_seq, offset)` into the matching stream (`backend/libs/parser/pipe/traces.go`; `ParamTypes`). The `bigParams` / `bigParamsDedup` names in earlier drafts are these two streams, not additional ones. The `sql` dedup cache clears on rotation (`Dumper.java`), so `sql` offsets are valid only within their `rolling_seq` — consistent with the 1:1 segment model.

**Consequence:** `03-lifecycle.md` §3.2, §3.4, §3.5 rewritten; `01-write-contract.md` §4.4, §8, §9, §10 aligned.

**Consequence:** propagated into the contract bodies — `01-write-contract.md` §2, §4.3, §5.1, §6 (esp. §6.5 seal pass, §6.6 late data), §8, §9; `03-lifecycle.md` §2, §3.2, §3.5–§3.8, §5.3, §6; `05-diagrams.md` §5, §7; `04-storage-layout.md` §3.5 (sizing). The top-of-file 2026-07-01 amendment blocks were collapsed to a dated one-line pointer once the bodies carried them.

**Demux mechanism (recorded to prevent re-litigation).** The raw trace stream stays interleaved on disk (one sequential append per pod-restart); demultiplexing is virtual, via `chunk_index[threadId]` — pointers `(segment_file, logical_offset, length)`, not buffered bytes. Rejected alternatives: (a) demux on receipt into per-thread output streams — turns one sequential append into thousands of tiny per-thread appends (M ≈ live threads on the replica); (b) per-call temp files — one file per in-flight call, rejected as too many handles and writes. The per-thread index is what makes "read only this call's fragment" cheap without either.

### 2026-07-01 — cursor / stable pagination across hot→cold (resolved)

**Question:** How does `/calls` paginate consistently while calls migrate from the hot tier (collector SQLite index) to the cold tier (S3 parquet) between page fetches?

**Choice:** Keyset (seek) pagination on the total order `(ts_ms DESC, pk ASC)`, binary collation. The cursor is an opaque base64 token carrying a format version, the frozen query (`from`, `to`, filters, ordering), the last position `(ts_ms, pk)`, and an issue timestamp; TTL `PROFILER_CURSOR_TTL` (default 15m). One global position, no per-source scroll state: each page re-fans-out, every source seeks past the position, `query` k-way merges, then dedups by PK before truncating to `limit` and before computing `next_cursor`.

**Reason:** `ts_ms` and the PK are immutable and identical in both tiers, so a migrated call keeps its position — migration stops being a special case. The design rests on the existing zero-gap guarantee (`02-read-contract.md` §4.3) and PK dedup (§6). Termination: `next_cursor = null` only when the position passes `from`; an empty mid-range page (rows aged out of hot and TTL-deleted from cold) is valid, not end-of-stream.

**Consistency envelope (explicit):** a pagination session sees a snapshot as of each reported position, not a whole-window snapshot; late data re-sealed below an already-passed position (`01-write-contract.md` §6.6) is not surfaced in that session. Same envelope as §4.3.

**Deferred within this decision:** alternative sort orders (e.g. by duration — `/stats` territory); a stateful scroll cursor (until deep-pagination profiling warrants it); HMAC-signing the cursor.

**Consequence:** `02-read-contract.md` §2.3.1 (new), §4.3 (pointer updated), §9 (config row), §11 (checklist); this spike removed from "Open spikes carried into Stage 1".

### 2026-07-02 — calls-stream time is a running delta, not an absolute offset

**Finding:** A design re-review flagged that the calls stream encodes each record's start time as a zig-zag varint delta from the *previous* record, seeded by the file header, not as an absolute offset from that header. The agent writes the running delta and advances its timer per record (`Dumper.java:1062-1063`), resetting only on file rotation, which also writes a fresh 8-byte `base_ms` header (`Dumper.java:1394-1401`; `CompressedLocalAndRemoteOutputStream.java:156`). The reused Go decoder read each delta as an absolute offset (`base_ms + delta_i`) without accumulating (`backend/libs/parser/pipe/calls.go:52,146`; `streams/calls.go:100,158`). The two formulas coincide for the first record of a file, so short fixtures stayed green; every later record's `ts_ms` was wrong.

**Why it matters:** `ts_ms` is the primary time axis. Bucketing (§5.4), retention (§6.4), the PK, and the read cursor all key off it, so a silent per-record drift corrupts all four.

**Resolution:** `01-write-contract.md` §5.1 specifies the reconstruction (`ts_ms_i = ts_ms_{i-1} + delta_i`, reseeded at each file header) and §5.2 annotates the `ts_ms` column. Both Go decoders now accumulate the deltas (`backend/libs/parser/pipe/calls.go`, `backend/libs/parser/streams/calls.go`), preserving the raw `Call.Time` field so existing CSV fixtures stay valid. `TestCallsTimeAccumulation` in each package guards the reconstruction with a synthetic three-record stream (5 ms, then one and two minutes apart) from the versioned generator `backend/libs/tests/helpers/wire`; the pre-fix formula matches only the first record, so the test fails against it.
