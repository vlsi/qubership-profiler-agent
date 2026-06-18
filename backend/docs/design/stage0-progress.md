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
