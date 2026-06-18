# 01 — Write contract

> Status: **draft**, awaiting review. Wire-protocol invariants (Section 1) verified against agent code; per-call trace bytes are assembled by the collector at chunk granularity (Section 4). No agent or protocol changes are required by this contract.

This document defines what the new Go collector writes to local PV and to S3, and on which events. It is the source of truth for Stages 1, 2, 6.

## 1. Background: what the agent actually sends

The agent opens a long-lived TCP connection to the collector and multiplexes seven named streams over it (`backend/libs/protocol/streams.go`). Each stream is a sequence of binary chunks delivered via `COMMAND_RCV_DATA` (`backend/libs/parser/parser.go`).

| Stream | Contents | Cardinality |
|---|---|---|
| `dictionary` | Append-only `(int → utf-8 string)` map. Method names, class names, tag names. | One per pod-restart |
| `params` | Parameter metadata: `(name, isIndex, isList, order, signature)`. | One per pod-restart |
| `suspend` | GC / JIT pause events `(time, delta, amount)`. | One per pod-restart |
| `calls` | One record per **closed root call** (e.g. one HTTP request handler invocation). Carries per-call summary metrics + a back-reference into the `trace` stream (`TraceFileIndex`, `BufferOffset`, `RecordIndex`). | Many per pod-restart |
| `trace` | Binary log of `methodEnter` / `methodExit` events. Children are written before their parent's close. | Many per pod-restart |
| `sql`, `xml` | Captured payload bodies referenced from calls. | Many per pod-restart |

Important consequence: **the collector does not assemble calls.** A `Call` record arrives only when the root call has closed on the agent side. The collector's job is to demultiplex streams, persist them, and emit a parquet row per `Call`.

### Verified against agent code

Sources: `dumper/src/main/java/com/netcracker/profiler/Dumper.java`, `boot/src/main/java/com/netcracker/profiler/agent/{LocalBuffer.java,DumperConstants.java}`.

**V1.** Verified. A `Call` record is emitted only after the root call has closed; its trace bytes are written to the trace stream during the same `writeBufferToFS` pass (`Dumper.java:940-983`).

**V2.** Partially verified. `(TraceFileIndex, BufferOffset, RecordIndex)` points to the START of the root call's first ENTER record (`Dumper.java:921-923`). The end is not transmitted on the wire. For the collector this is moot — we reassemble blobs at chunk granularity, not by byte range (§4).

**V3.** Verified. Dictionary entries are append-only within one pod-restart.

**V4.** Verified. A pod-restart boundary is a new TCP connection with a new `COMMAND_GET_PROTOCOL_VERSION_V2` handshake. **`restartTime` is stamped by the collector at TCP accept** — the agent does not transmit it. No protocol change required.

**V5.** Corrected — bytes for one root call are NOT contiguous in the stream. The trace stream is a sequence of `LocalBuffer` chunks: each chunk is prefixed with a 16-byte header `[threadId:long, startTime:long]`, holds events of ONE thread, and ends with `EVENT_FINISH_RECORD` (`Dumper.java:881-1004`, `LocalBuffer.SIZE=4096`). Chunks from different threads interleave at chunk boundaries. One business call spans N chunks of the same thread, separated by chunks of other threads. **Implication: the collector reassembles per-call blobs at chunk granularity at write time** (§4). No protocol change required.

**V6.** Verified. One TCP connection corresponds to one `(namespace, service, podName)` triple.

## 2. What the collector persists

There are four distinct artifacts. They live in different storage tiers because they have different access patterns, durability requirements, and lifetimes.

| Artifact | Storage | Why |
|---|---|---|
| Dictionary WAL | Local RWO PV, append-only file | Required for restart recovery. Without it, after a collector restart the trace bytes already received but not yet decoded are unreadable. |
| Raw chunks staging | Local RWO PV, append-only file(s) | Buffers incoming chunks of all threads as received. Source for per-call reassembly (§4). Refcounted; deleted when every parquet row that sourced from it has been uploaded to S3. |
| Per-call blob | Inside parquet (`trace_blob` column) | Reassembled by the collector at chunk granularity at write time. Lifetime = parquet row lifetime. |
| Pending parquet | Local RWO PV, parquet writer state | Current "writing" parquet file for each retention class. Closed and uploaded on flush. |
| Closed parquet | S3 | Read by query (cold path) and by maintenance (retention). |

In-memory state is small: dictionary (full copy for fast lookups), open parquet writers, and per-thread accumulators `accumulator[threadId]` — ordered lists of `(staging_file, offset, length)` triples pointing at chunks of the currently-open root call (§4.3). There is no in-memory call-tree assembly; the collector never decodes individual events at write time.

## 3. Dictionary WAL

### 3.1 Why a WAL

The trace stream is encoded against the dictionary. A method-enter event references method ID 42, and the dictionary tells us "42 = `com.example.Service.handle`". If the collector restarts and the dictionary is lost, all subsequent trace bytes for that pod-restart are unreadable: the agent does not retransmit dictionary entries.

A WAL writes each new dictionary word to disk as it arrives, so on restart we can rebuild the dictionary by replay.

### 3.2 Format

One file per `(namespace, service, podName, restartTime)` tuple. Append-only. Records are length-prefixed:

```
record := varint(record_len) record_body
record_body := varint(word_id) varint(word_len) word_bytes
```

`word_id` is the dictionary position from the agent (sequential int starting at 0). `word_bytes` is UTF-8.

No header, no checksums per record. A single CRC32 at file footer is computed on close and verified on replay; partially written tail past the last valid record is truncated on recovery.

### 3.3 Durability

`fsync` is called every N entries or every T milliseconds, whichever first. Defaults: `N = 256`, `T = 100 ms`. Configurable via env.

Rationale: a power-loss between fsync windows costs at most 100 ms of dictionary entries. Subsequent trace bytes referencing those entries become unreadable for that window — those calls are dropped from the parquet output and logged. This is acceptable for a profiler.

### 3.4 Path

```
/data/pods/<namespace>/<service>/<podName>/<restartTime>/dictionary.wal
```

`<restartTime>` is a Unix-milliseconds string stamped by the collector at the moment of TCP accept (= the boundary marked by `COMMAND_GET_PROTOCOL_VERSION_V2`). The agent does not transmit it. No protocol change required.

### 3.5 Lifetime

The local WAL is deleted when the corresponding pod-restart is fully flushed: all `Call` records for it are written to parquet, parquet uploaded to S3, the dictionary snapshot is uploaded to S3 (§3.6), and a hold-back grace period (default 1 hour) elapses to allow late-arriving Calls.

`params` and `suspend` streams use the same WAL pattern with separate files (`params.wal`, `suspend.wal`).

### 3.6 Dictionary snapshot in S3

The per-call trace blobs (§4) embed `method_id` and `param_id` references that resolve against the per-pod-restart dictionary. Without the dictionary the blobs are not decodable.

Parquet rows have a per-bucket retention of up to 30 days (`long_clean`, `any_error`); the local dictionary WAL lives only until the pod-restart fully flushes (§3.5). Therefore the dictionary MUST be persisted alongside parquet.

**On pod-restart close** (TCP connection terminates AND all in-flight Calls flush), the collector:

1. Reads the local WAL and builds the final dictionary snapshot in memory.
2. Serializes it as a single JSON object: `{ version, methods: [...], params: [...] }`.
3. Uploads it to S3 with a deterministic key:

   ```
   s3://<bucket>/dictionaries/v1/<yyyy>/<mm>/<dd>/<podRestartHash>.json
   ```

   Date hierarchy mirrors the parquet layout (§7) so maintenance can apply a single retention rule per day.

4. Deletes the local WAL after the hold-back grace period.

**Retention:** the S3 dictionary lives at least as long as the longest retention class touched by any parquet row of this pod-restart. The simplest policy — `PROFILER_RETENTION_DICTIONARY_TTL` (default `35d`) — covers the longest retention bucket (`30d`) plus a safety margin.

**Live pod-restart:** while the TCP connection is open, the dictionary lives only on local WAL + RAM. Reads go through the collector replica's `/internal/v1/pods/{pod-restart}/dictionary` (see `02-read-contract.md` §2.6). Query never reaches into the WAL directly.

**Growth during live pod-restart:** the dictionary is append-only (§1 V3). If a collector crashes and recovers mid-flight, the WAL is replayed; the on-disk monotonic `version` counter is rebuilt from the highest entry's index. Clients revalidating an old ETag get the fresh snapshot.

## 4. Per-call trace blob assembly

### 4.1 Why per-call reassembly

The trace stream is chunk-interleaved (§1 V5): bytes of one root call are scattered across N chunks of the same thread, with chunks of other threads in between. We cannot serve "the bytes for call X" as a single byte range from the wire stream.

The collector materializes each root call's bytes into a contiguous per-call blob at chunk granularity, in real time as Call records close. The blob is embedded in the parquet row as the `trace_blob` column.

This is a chunk-level memcpy. The collector does NOT parse events inside chunks at write time — only the 16-byte chunk header.

### 4.2 Chunk model

Each chunk begins with `[threadId:long, startTime:long]` (16 bytes), followed by events, terminated by `EVENT_FINISH_RECORD` (`Dumper.java:881-1004`, `LocalBuffer.SIZE=4096` events ≈ tens of KB on the wire). The full body of one chunk belongs to one thread. Within a chunk, multiple short root calls of that thread may open and close — tolerated as "neighbor noise" inside the per-call blob (§4.5).

### 4.3 Reassembly algorithm

Per-thread state held in RAM:

- `accumulator[threadId]` — ordered list of `(staging_file, offset, length)` for chunks of T's currently-open root call.

On each incoming chunk:

1. Append the chunk verbatim to the current `chunks/<seq>.bin` staging file on PV (§4.4).
2. Parse the 16-byte header → extract `threadId`.
3. Append `(staging_file, offset_in_file, chunk_length)` to `accumulator[threadId]`.

On each incoming `Call` record for thread T (delivered on the calls stream):

1. memcpy each chunk referenced by `accumulator[T]` from its staging file into a contiguous in-memory blob.
2. Write the parquet row with `trace_blob` = blob; route by retention class (§6.4).
3. Reset `accumulator[T]`. **Carry the last chunk over** to the new accumulator — it may contain the start of T's next root call. That chunk is read twice and its bytes are duplicated into both blobs.

The carry-over rule is the only source of byte duplication; bounded by one chunk per pair of consecutive same-thread root calls.

### 4.4 Staging files on PV

```
/data/pods/<ns>/<svc>/<pod>/<restartTime>/chunks/<seq>.bin
```

- Append-only. Each chunk written verbatim (including the 16-byte header), so a staging file is itself a valid chunk stream.
- Rotated when size exceeds `PROFILER_CHUNKS_STAGING_FILE_SIZE` (default 256 MB). New file `<seq+1>.bin` opened; accumulators continue uninterrupted.
- Refcounted in the SQLite metadata DB (§8): incremented when an accumulator first references a file, decremented when each referencing parquet row uploads to S3.
- Deletable once refcount = 0.

### 4.5 Reader semantics

The per-call blob is a self-contained chunk stream (concatenation of full chunks); all chunks have the SAME `threadId` in their headers. The reader walks events to build the call tree:

- **Tail noise:** the first chunk may begin with events of the PREVIOUS root call of this thread, ending at its depth-0 EXIT. The reader skips events until it reaches the depth-0 ENTER matching the row's `record_index`.
- **Head noise:** the last chunk may end with the start of the NEXT root call of this thread, beginning after the depth-0 EXIT of the call we want. The reader stops at depth-0 EXIT.

Noise is bounded by `LocalBuffer.SIZE` (4096 events ≈ tens of KB per side).

### 4.6 Recovery and budget

**Disk budget:** `PROFILER_CHUNKS_STAGING_MAX_BYTES` (default 10 GB) bounds total staging-file disk usage per replica. Eviction:

1. First, drop staging files whose refcount is zero.
2. If still over budget, drop the oldest staging files even with pending refs. Affected blobs are emitted with `trace_blob = NULL` and `truncated_reason = disk_budget` (§5.2); metrics counter incremented.

**Memory budget:** accumulator state in RAM is capped by `PROFILER_MEM_BUDGET`. Under pressure, the oldest open accumulator is force-emitted as `truncated_reason = mem_pressure`.

**Idle timeout:** if an accumulator receives no chunks for `PROFILER_IDLE_ACCUMULATOR_TIMEOUT` (default 10 min) and no Call record arrives, the blob is emitted as `truncated_reason = idle_timeout` — these are calls whose Call record was lost or whose thread died.

**Crash recovery:** on collector restart, walk staging files in `chunks/`, replay chunk headers, and rebuild `accumulator[*]`. The SQLite metadata DB carries refcounts and the "last uploaded parquet" checkpoint; pending parquet writers are reconstructed from staging files + their `Call` records held in `parquet-pending/`.

## 5. Calls stream → parquet

### 5.1 Pipeline

For each `Call` record arriving on the calls stream:

1. Resolve dictionary words for `Method` and any tag IDs in `Params`.
2. Take the pre-assembled blob from `accumulator[Call.thread_id]` (§4.3); reset that accumulator and carry the last chunk forward.
3. Derive `retention_class` from `(duration_ms, error_flag)` (§6.4) and pick the corresponding open parquet writer for the current time bucket.
4. Append a row to that writer.

Calls that fail step 1 (missing dictionary entry) are written with `trace_blob` NULL and `truncated_reason = dict_miss`. Calls that hit the cache budgets in §4.6 are written with `trace_blob` NULL and the corresponding truncation reason. Counters exposed as Prometheus metrics.

### 5.2 Parquet schema

Starting from the existing `CallParquet` (`backend/libs/storage/parquet/calls.go`) and refining. Each column carries a one-line rationale.

```
schema CallV2 {
  -- identity
  ts_ms             INT64                      -- call start, Unix ms UTC; primary time axis
  pod_id            BYTE_ARRAY (UTF8) DICT     -- "<ns>/<service>/<pod>"; dictionary-encoded for compact storage
  restart_time_ms   INT64                      -- pod-restart boundary; dedupe key component
  trace_file_index  INT32                      -- PK component; chunk-staging file ordinal at the start of the call's bytes
  buffer_offset     INT32                      -- PK component; offset within trace_file_index where the call's first chunk begins
  record_index      INT32                      -- PK component; event index of the root ENTER within that chunk
  thread_name       BYTE_ARRAY (UTF8) DICT     -- thread name; high cardinality bounded by app threadpool size

  -- dimensions
  namespace         BYTE_ARRAY (UTF8) DICT
  service_name      BYTE_ARRAY (UTF8) DICT
  pod_name          BYTE_ARRAY (UTF8) DICT
  method            BYTE_ARRAY (UTF8) DICT     -- root method, resolved from dictionary at write time

  -- metrics (raw, not aggregated)
  duration_ms       INT32                      -- main filter axis
  cpu_time_ms       INT64
  wait_time_ms      INT64
  memory_used       INT64
  non_blocking_ms   INT64                      -- agent-specific
  queue_wait_ms     INT32
  suspend_ms        INT32                      -- GC/JIT pause time within this call
  child_calls       INT32                      -- count of child method invocations in the trace tree
  transactions      INT32
  logs_generated    INT64
  logs_written      INT64
  file_read         INT64
  file_written      INT64
  net_read          INT64
  net_written       INT64

  -- classification (collector-derived at write time)
  error_flag        BOOLEAN                    -- agent-indicated error; see §5.6
  retention_class   BYTE_ARRAY (UTF8) DICT     -- one of short_clean / normal_clean / long_clean / any_error / corrupted; see §6.4

  -- semi-structured
  params            MAP<UTF8, LIST<UTF8>>      -- per-param key→values, e.g. "request.id" → ["abc123"]
  trace_blob        BYTE_ARRAY                 -- per-call blob assembled at chunk granularity (§4); reader parses against the dictionary; NULL when truncated
  truncated_reason  BYTE_ARRAY (UTF8) DICT     -- NULL on success; one of mem_pressure / disk_budget / idle_timeout / dict_miss (§5.1, §4.6)
}
```

### 5.3 Differences from old `CallParquet`

| Change | Reason |
|---|---|
| `time` → `ts_ms`, with `_ms` suffix | The unit is the most common confusion source. Always carry it in the name. |
| `restartTime` → `restart_time_ms` | Same. |
| Added `pod_id` derived column | Trivially computable but speeds up filtering and dictionary-encodes well. |
| `Method` stays as resolved string, not `int TagId` | Otherwise every reader needs the dictionary on hand. The cost is dictionary-encoding overhead in parquet — minor. |
| Renamed `Calls` → `child_calls` | "calls" is overloaded with "list of calls"; this is the per-tree counter. |
| Removed `convertedtype=UINT_*` annotations | Parquet's UINT_64 is poorly supported in some readers. INT64 with documented "always non-negative" suffices. |
| `TraceId string "seqId_bufOffset_recordIndex"` → three `INT32` columns (`trace_file_index`, `buffer_offset`, `record_index`) | Better column compression, cheaper integer comparison at dedup time, no string parsing on the read path. Decision recorded; no open question remains. |

### 5.4 Sharding: time bucket × retention class

Each parquet file covers one (time bucket × retention class × pod-restart):

- **Time bucket:** 5 minutes by default, aligned to wall clock (00:00, 00:05, …). Configurable.
- **Retention class:** computed from `(duration_ms, error_flag)` at write time. Five classes by default — see §6.4.

So for each pod-restart, at any given moment there are up to five open parquet writers (one per retention class) for the current time bucket. `duration_ms` remains a row-level column for query-time filtering.

### 5.5 Path

Local pending:
```
/data/pods/<namespace>/<service>/<podName>/<restartTime>/parquet-pending/<timeBucket>/<retentionClass>.parquet
```

After flush, uploaded to S3 (Section 7).

### 5.6 error_flag derivation

The natural source of an error indicator is the Java agent's `CallInfo.isCallRed` (set whenever `ExceptionLogger.callRed()` fires, typically from a caught exception — `boot/src/main/java/com/netcracker/profiler/agent/ExceptionLogger.java:29-35`; the field itself: `boot/.../CallInfo.java:33`). However, **`isCallRed` is not currently surfaced on the Go-side `Call` struct** (`backend/libs/protocol/data/calls.go` — no field for it). Exposing it requires either confirming the wire `Call` record already carries the boolean and adding it to the Go decoder, or — if the wire format does not include it — a small agent change. Both are tracked as out-of-scope follow-ups, not blockers for Stage 1.

Until `isCallRed` flows through, the MVP derives `error_flag` from one signal only:

- `callInfo.isCorrupted` — the agent could not finish the call cleanly (already on the Go side via the regular Call decoding path).

`callInfo.isPersist` is **not** an error flag — it is the persistence gate the agent uses to decide whether to emit a Call record at all (`Dumper.java:945-947`). It is therefore not folded into `error_flag`.

Practical consequence for the retention table (§6.4): until `isCallRed` is exposed, the `any_error` retention class effectively shares its content with `corrupted`. Both classes are kept as distinct buckets in the schema so that wiring `isCallRed` later does not require re-partitioning historical data.

## 6. Flush semantics

### 6.1 Triggers

A pending parquet file is closed and uploaded when **any** of:

1. Its time bucket has ended (current wall-clock time ≥ bucket end + `time_bucket_grace`). Default `time_bucket_grace = 30 s` to absorb late Call arrivals.
2. Its file size exceeds `parquet_max_size` (default 64 MB). New writer for the same bucket is opened and continues with a sequence suffix.
3. Memory pressure: the collector exceeds `mem_budget` and selects largest parquet writers to evict early.

Trigger 3 is rare in practice — parquet writers buffer little (row groups are small).

### 6.2 Atomic upload

Upload sequence:

1. Close the parquet writer locally → file is fully on disk.
2. PUT to S3 with `Content-MD5`.
3. On 200 OK, decrement chunks-staging refcounts (§4.4) and record `(file_path, retention_class, time_bucket_end, uploaded_at)` in `metadata.sqlite`. **The local file is NOT deleted here** — it serves the hot tier until `hot_retention` past flush.
4. On any S3 error, retry with exponential backoff. The local file remains until upload succeeds.

If the collector crashes between local close and S3 upload, on restart we re-read pending parquet files in `parquet-pending/` and re-attempt upload. Idempotent at the S3 layer because the object key is deterministic (Section 7) — re-uploading the same file produces the same object.

### 6.3 Hot retention of local parquet

After successful upload, local parquet files are retained for `PROFILER_HOT_RETENTION` (default `15m`) to back the collector's hot-read API (`02-read-contract.md` §4.2). A janitor goroutine deletes files where `now > uploaded_at + hot_retention` and removes the corresponding row from `metadata.sqlite`.

`hot_retention ≥ flush_interval + overlap_margin` must hold — otherwise queries are not guaranteed to see every Call from at least one tier during the overlap window.

### 6.4 Retention class

Each parquet file holds rows of exactly one retention class. Maintenance applies per-class TTL.

Default mapping (configurable per-deployment):

| Class | Condition | Default TTL |
|---|---|---|
| `short_clean` | `duration_ms < 100` AND `!error_flag` | 1 day |
| `normal_clean` | `100 ≤ duration_ms < 1000` AND `!error_flag` | 7 days |
| `long_clean` | `duration_ms ≥ 1000` AND `!error_flag` | 30 days |
| `any_error` | `error_flag = true` (any duration) | 30 days |
| `corrupted` | `callInfo.isCorrupted` (subclass of `any_error`, segregated for forensics) | 7 days |

The classifier runs per Call record at write time and routes the row to one of up to 5 open parquet writers for the current time bucket. Maintenance reads `<retentionClass>` from the S3 object key (§7) — it does not open parquet files to apply TTL.

## 7. S3 object layout

Path pattern:
```
s3://<bucket>/parquet/v1/<retentionClass>/<yyyy>/<mm>/<dd>/<hh>/<replica>-<podRestartHash>-<timeBucketStart>-<seq>.parquet
```

- `v1` — schema version. Bump on incompatible changes.
- `<retentionClass>` — one of `short_clean` / `normal_clean` / `long_clean` / `any_error` / `corrupted`. Maintenance applies per-class TTL by listing this segment (§6.4). Filenames sort chronologically within a class.
- Date hierarchy `<yyyy>/<mm>/<dd>/<hh>` — primary access pattern is "give me parquet for time range [t1, t2]"; this hierarchy makes that a small LIST.
- `<replica>` — the StatefulSet ordinal (`collector-0`, `collector-1`, …). Ensures distinct replicas don't collide on object keys.
- `<podRestartHash>` — short hash of `(namespace, service, podName, restartTime)`. Distinguishes pod-restarts.
- `<timeBucketStart>` — the bucket's start as `yyyymmddTHHMMSSZ`.
- `<seq>` — sequence number when one bucket spawned multiple files via size trigger.

Example:
```
s3://profiler-data/parquet/v1/normal_clean/2026/04/23/14/collector-2-a7f3-20260423T140000Z-0.parquet
```

Why date in the path even though `ts_ms` is in the file: query needs to LIST efficiently. Filtering by reading every parquet file's footer to check time range is too expensive at scale.

## 8. Local PV layout (full)

```
/data/
  pods/
    <namespace>/<service>/<podName>/<restartTime>/
      dictionary.wal
      params.wal
      suspend.wal
      chunks/
        000.bin              # raw chunks staging, refcounted, deleted post-upload
        001.bin
        ...
      parquet-pending/
        20260423T140000Z/
          short_clean.parquet
          normal_clean.parquet
          long_clean.parquet
          any_error.parquet
          corrupted.parquet
  upload-failed/             # parquet that S3 rejected and needs human attention
    ...
  collector.lock             # exclusive PV ownership (one collector replica)
  metadata.sqlite            # staging-file refcounts, upload checkpoints, dictionary index
```

`collector.lock` is a flock'd file written at startup. Prevents two collector processes from sharing a PV — critical when `volumeClaimTemplates` is misconfigured.

## 9. Configuration

| Env | Default | Description |
|---|---|---|
| `PROFILER_DATA_DIR` | `/data` | Root of the local PV. |
| `PROFILER_TIME_BUCKET` | `5m` | Parquet time bucket length. |
| `PROFILER_TIME_BUCKET_GRACE` | `30s` | Wait after bucket end before flush. |
| `PROFILER_PARQUET_MAX_SIZE` | `64MB` | Size trigger for early flush. |
| `PROFILER_MEM_BUDGET` | `2GB` | Soft memory budget for in-flight buffers. |
| `PROFILER_CHUNKS_STAGING_MAX_BYTES` | `10GB` | Total disk budget for raw chunks staging files (§4.6). |
| `PROFILER_CHUNKS_STAGING_FILE_SIZE` | `256MB` | Rotate the current staging file when it exceeds this. |
| `PROFILER_IDLE_ACCUMULATOR_TIMEOUT` | `10m` | Force-emit a per-thread accumulator when no chunk arrives for that thread within the window. |
| `PROFILER_DICT_FSYNC_RECORDS` | `256` | Dictionary WAL fsync trigger by record count. |
| `PROFILER_DICT_FSYNC_INTERVAL` | `100ms` | Dictionary WAL fsync trigger by time. |
| `PROFILER_DURATION_THRESHOLDS` | `100ms,1s` | Boundaries for retention class derivation (§6.4). |
| `PROFILER_RETENTION_SHORT_CLEAN_TTL` | `1d` | TTL for `short_clean` class. |
| `PROFILER_RETENTION_NORMAL_CLEAN_TTL` | `7d` | TTL for `normal_clean` class. |
| `PROFILER_RETENTION_LONG_CLEAN_TTL` | `30d` | TTL for `long_clean` class. |
| `PROFILER_RETENTION_ANY_ERROR_TTL` | `30d` | TTL for `any_error` class. |
| `PROFILER_RETENTION_CORRUPTED_TTL` | `7d` | TTL for `corrupted` class. |
| `PROFILER_RETENTION_DICTIONARY_TTL` | `35d` | TTL for S3 dictionary snapshots (§3.6). Must exceed the longest parquet retention class. |
| `PROFILER_HOT_RETENTION` | `15m` | Local parquet retention past flush (§6.3 and `02-read-contract.md` §4.2). |
| `S3_ENDPOINT` | — | MinIO/S3 endpoint URL. |
| `S3_BUCKET` | — | Target bucket. |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | — | Credentials. |
| `S3_PATH_PREFIX` | `parquet/v1` | Object key prefix below the bucket. |
| `STATEFULSET_ORDINAL` | (from `HOSTNAME`) | Used in S3 object key. |

## 10. What this contract does not cover

These are intentional gaps to be addressed by other documents:

- Recovery sequence on startup (read WAL, re-attempt pending uploads, etc.) → `03-lifecycle.md`.
- Read API for hot data → `02-read-contract.md`.
- Heap/thread dump streams (`sql`, `xml`, dumps) → out of scope for now; remains served by `dumps-collector` until Stage C5.
- Maintenance retention rules (per-bucket TTL, cleanup of S3) → covered briefly in main plan, detailed in maintenance design when Stage 2 begins.

## 11. Review checklist

Before this document is merged and Stage 1 starts, please confirm or correct:

- [x] V1–V6 verified against agent code (§1).
- [x] `restartTime` source — collector-stamped on TCP accept (§3.4).
- [x] Trace-bytes extraction strategy — per-call chunk-level reassembly (§4).
- [x] `trace_id` column shape — three `INT32` columns (§5.2, §5.3).
- [x] `error_flag` source — `isCorrupted` in MVP; `isCallRed` deferred as a follow-up (§5.6).
- [x] Default retention class TTLs and duration thresholds (§6.4, §9) — defaults accepted.
- [ ] S3 path structure (§7) — operational fit.
- [x] Dictionary cold-path lifecycle — final snapshot uploaded to S3 on pod-restart close (§3.6); local WAL purged after upload + grace.
- [ ] Configuration defaults (§9).

Follow-ups out of scope for this contract:

- Surface `CallInfo.isCallRed` on the Go-side `Call` struct (`backend/libs/protocol/data/calls.go`); confirm whether it is already on the wire or whether the Java agent needs a small change. Once exposed, `error_flag` derivation in §5.6 picks it up automatically.
- Consolidate `backend/libs/parser/streams/` into `backend/libs/parser/pipe/` (decision 8 in `profiler-plan.md`).
