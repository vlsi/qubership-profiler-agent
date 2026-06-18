# 02 — Read contract

> Status: **draft**, awaiting review. Read API for both external clients (UI, automation, MCP) and internal query → collector fan-out. Fresh design — only the agent's TCP wire protocol (Section 1 of `01-write-contract.md`) is preserved; the legacy Java collector's external API is **not** a baseline.

## 1. Scope

Two API surfaces:

- **External:** clients (UI, scripts, MCP) talk to the `query` service. JSON over HTTP.
- **Internal:** `query` fans out to `collector` replicas for the hot tier and reads parquet from **S3** for the cold tier. JSON over HTTP for collector; S3 SDK for cold.

The agent's TCP wire protocol is preserved — existing agents continue to ship to the new collector with no changes (see `01-write-contract.md` §1).

The Java collector's external API is **not** preserved. The new query service designs endpoints fresh; the UI is rewired in Stage 5 (`profiler-plan.md`).

No authentication for MVP. Add later (Keycloak/Bearer is the likely choice).

## 2. External query API

Base path: `/api/v1`. JSON request and response. Error bodies follow RFC 7807 (Problem Details).

### 2.1 Endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | `/pods` | List `(namespace, service, pod, restart_time)` tuples with any data in a time range. |
| GET | `/pods/{pod-restart}/dictionary` | Per pod-restart dictionary (`method_id → string`, `param_id → string`). For advanced consumers using raw `/trace` (see §2.5). |
| GET | `/calls` | Page through Call records matching filter. |
| GET | `/calls/{pk}` | Fetch a single Call by PK with full metadata. |
| GET | `/calls/{pk}/trace` | Fetch the per-call trace blob (raw wire bytes; advanced consumers — see §2.5). |
| GET | `/calls/{pk}/tree` | Server-decoded call tree, MessagePack-encoded (canonical for UI / MCP / CLI — see §2.5). |
| GET | `/stats` | Aggregates: top methods by duration, p50/p95/p99, counts per bucket. (Sketch only; details deferred — §10.) |

### 2.2 Primary key

Every Call has a 7-component PK:

```
PK = {
  pod_namespace:     string,
  pod_service:       string,
  pod_name:          string,
  restart_time_ms:   int64,
  trace_file_index:  int32,
  buffer_offset:     int32,
  record_index:      int32,
}
```

`trace_file_index`, `buffer_offset`, `record_index` are stored as three INT32 columns in parquet (decision in `01-write-contract.md` §5.3). `pod_*` and `restart_time_ms` are stored as separate columns.

URL serialization (path segments are colon-separated, percent-encoded):

```
<ns>:<svc>:<pod>:<restartMs>:<file>:<off>:<rec>
```

JSON bodies use the nested-object form.

### 2.3 GET /calls — filter and pagination

Query parameters:

| Param | Type | Notes |
|---|---|---|
| `from` | int64 (Unix ms) | Required. Inclusive lower bound on `ts_ms`. |
| `to` | int64 (Unix ms) | Required. Exclusive upper bound on `ts_ms`. |
| `pod` | string (repeatable) | Optional. `namespace/service/pod`. |
| `method` | string | Optional substring/prefix match on the `method` column. |
| `duration_min_ms` | int32 | Optional. |
| `duration_max_ms` | int32 | Optional. |
| `error_only` | bool | Optional. Maps to `error_flag = true`. |
| `retention_class` | string (repeatable) | Optional. `short_clean` / `normal_clean` / `long_clean` / `any_error` / `corrupted`. |
| `cursor` | string | Opaque cursor from previous page's `next_cursor`. |
| `limit` | int | Page size. Default 100, max 1000. |

Response:

```json
{
  "calls": [
    {
      "pk": {
        "pod_namespace": "default", "pod_service": "billing", "pod_name": "billing-7f8c",
        "restart_time_ms": 1714060800000,
        "trace_file_index": 5, "buffer_offset": 12340, "record_index": 0
      },
      "ts_ms": 1714060812345,
      "duration_ms": 1247,
      "method": "com.example.Service.handle",
      "thread_name": "http-nio-8080-exec-3",
      "cpu_time_ms": 873,
      "wait_time_ms": 12,
      "memory_used": 1048576,
      "child_calls": 42,
      "error_flag": false,
      "retention_class": "long_clean",
      "params": { "request.id": ["abc123"] },
      "trace_blob_size": 18432,
      "truncated_reason": null
    }
  ],
  "next_cursor": "opaque-string-or-null",
  "partial": false,
  "partial_reasons": []
}
```

`trace_blob_size = 0` when `truncated_reason != null` (blob was dropped under pressure; see §4.6 of `01-write-contract.md`).

### 2.4 Trace blob — lazy endpoint

`GET /api/v1/calls/{pk}/trace` returns the per-call blob as raw bytes.

- `Content-Type: application/octet-stream`
- `ETag`: a stable hash of the PK (blob is immutable per PK)
- `Cache-Control: public, max-age=31536000, immutable`
- `Accept-Ranges: bytes` — supports `Range:` for partial reads (useful when the UI streams the start of a long trace)
- `404` if `trace_blob = NULL` (see `truncated_reason` in the Call row)
- `503 + partial` markers do not apply here: blob is either present or absent.

Reader semantics (chunk stream with tail/head noise; parsed against the per-pod-restart dictionary) — see `01-write-contract.md` §4.5.

### 2.5 Rendering the call tree

Two consumption paths:

- **`/calls/{pk}/tree` — canonical path** for UI, MCP, CLI. Server pre-aggregates the per-call blob into a tree, encodes as MessagePack with stable int-keyed maps and a version envelope. Self-contained (the response carries its own per-tree dictionary inline). Hand-written decoders in any language are ~50–80 LOC.
- **`/calls/{pk}/trace` + `/pods/{pod-restart}/dictionary` — advanced path** for consumers that want the raw wire format (third-party tooling re-using our Go decoder, full-fidelity offline analysis). Smaller payload, but more client code to maintain.

The decision in MVP: ship `/tree` as the canonical contract. `/trace` + `/dictionary` remain as the secondary, lower-traffic interface — useful, but not the default.

#### 2.5.1 Why MessagePack with int-keyed maps

Schema-evolution requirement: backend + UI ship together today, but MCP and CLI are planned for Stage 4+. Future field additions must not break older clients of either kind.

To avoid a `.proto` schema toolchain and codegen step while still getting forward/backward compatibility, the response uses the **int-keyed map** convention — semantically equivalent to protobuf field tags, encoded in plain MessagePack:

- Every record (`Tree`, `Node`, `Param`) is a `Map<int, value>`, NOT a positional array.
- Field numbers are documented in this contract (§2.5.3) and are append-only.
- Removed fields keep their numbers reserved (never re-used) — same as protobuf.
- Top-level `v` (version) supports hard breaking changes if they are ever required.
- Unknown int keys MUST be silently ignored by decoders (= forward compat).

Trade-off vs protobuf: hand-written ~50–80 LOC decoders per consumer language; no codegen; convention enforced by code review of this contract document. Acceptable for an in-team API. The shape maps 1:1 to protobuf field tags, so migration later (e.g. if external partners appear) is mechanical.

#### 2.5.2 Response envelope

```
GET /api/v1/calls/{pk}/tree
Accept: application/x-msgpack

200 OK
Content-Type: application/x-msgpack
ETag: <hash-of-PK>
Cache-Control: public, max-age=31536000, immutable
Content-Encoding: gzip          # transparent if client supports it

body — MessagePack-encoded Map<int, value>:
  {
    0: 1,                                      # v: version = 1
    1: ["com.example.Service.handle", "..."],  # methods: per-tree method dictionary
    2: ["request.id", "..."],                  # params:  per-tree param-key dictionary
    3: <Node>                                  # root: the root node
  }
```

The `methods` and `params` arrays carry only strings that this specific tree references — not the entire pod-restart dictionary. The response is self-contained; no separate dictionary fetch is required for this path.

#### 2.5.3 Field tag tables

`Tree` envelope (top level):

| # | Field | Type | Notes |
|---|---|---|---|
| 0 | `v` | int | Format version. Currently `1`. |
| 1 | `methods` | `[str]` | Per-tree method dictionary. |
| 2 | `params` | `[str]` | Per-tree param-key dictionary. |
| 3 | `root` | `Node` | Root node of the call tree. |

`Node`:

| # | Field | Type | Required | Notes |
|---|---|---|---|---|
| 0 | `methodIdx` | int | yes | Index into top-level `methods`. |
| 1 | `enterMsRel` | int | yes | Millis from root's enter. Root's value is `0`. |
| 2 | `durationMs` | int | yes | Millis. |
| 3 | `params` | `[Param]` | no | Omitted if the node has no params. |
| 4 | `children` | `[Node]` | no | Omitted for leaf nodes. |
| 5+ | reserved | — | — | Future additions (e.g. `cpuMs`, `memBytes`) use the next free numbers. |

`Param`:

| # | Field | Type | Required | Notes |
|---|---|---|---|---|
| 0 | `paramIdx` | int | yes | Index into top-level `params`. |
| 1 | `values` | `[str]` | yes | Multi-value list (`01-write-contract.md` §5.2 carries `params` as `MAP<UTF8, LIST<UTF8>>`). |

**Reserved-number registry.** When a field is removed in a future version, its number is added below and never re-used. (Empty in v1.)

#### 2.5.4 Versioning rules

- **Additive changes (new fields).** New int key appended at the next free number. Old clients skip unknown keys; new clients see new data. **No `v` bump.**
- **Field removal.** Server stops emitting; number moves to the reserved registry above. **No `v` bump.**
- **Breaking changes** (rename, type change, restructure of the tree shape). Bump `v` to `2`. Server emits `v: 1` when client sends `Accept-Version: 1` (request header), `v: 2` otherwise. Old `v: 1` support kept for 6+ months past the `v: 2` launch, then retired with a release note.

#### 2.5.5 Compression

The wire payload goes through standard HTTP `Content-Encoding: gzip` if the client advertises `Accept-Encoding: gzip`. UI does by default; MCP/CLI are expected to as well. Saves another 30–50% on large trees. Server-side gzip is cheap (`compress/gzip`); no separate dictionary needed since the per-tree dictionary already shares strings inside this response.

### 2.6 Dictionary endpoint

`GET /api/v1/pods/{pod-restart}/dictionary` returns the per-pod-restart dictionary that the blob references.

Path serialization for the pod-restart key: `<ns>:<svc>:<pod>:<restartMs>` (same convention as the Call PK, minus the call coordinates).

Response:

```json
{
  "version": 4231,
  "methods": ["com.example.Service.handle", "com.example.Service.tx", "..."],
  "params":  ["request.id", "user.tenant", "..."]
}
```

- `methods[i]` and `params[i]` resolve `method_id = i` and `param_id = i` references inside the blob.
- `version` is a monotonic counter, incremented each time the dictionary grows during a live pod-restart. ETag is `(pod-restart, version)`.
- For a **live** pod-restart (TCP connection still open), the dictionary may grow. `query` forwards the request to the collector replica hosting that pod-restart (via internal endpoint, §3) where it lives on local PV + RAM. Clients revalidate with `If-None-Match`; on a no-change response 304 is returned. On growth, a new full snapshot is returned (small enough that delta encoding is not worth the complexity).
- For a **closed** pod-restart (TCP connection terminated, `restart_time_ms` in the past), `query` reads a final snapshot from S3 (`s3://<bucket>/dictionaries/v1/...`, see `01-write-contract.md` §3.6). Response includes `Cache-Control: public, max-age=31536000, immutable`.

### 2.7 Pods and stats

`GET /api/v1/pods?from=...&to=...` returns the set of `(namespace, service, pod, restart_time)` tuples that have any Call rows in the time range.

`GET /api/v1/stats` is a sketch — full schema deferred to Stage 4. Initial shape: `top_methods_by_duration`, latency percentiles per `(method, retention_class)`, counts per `(retention_class, hour_bucket)`. Implemented over the same hot/cold model as `/calls`.

## 3. Internal hot-read API on collector

Base path: `/internal/v1`. Same JSON shapes as `/api/v1`. Aggregation is done in `query`, not in collector.

| Method | Path | Purpose |
|---|---|---|
| GET | `/internal/v1/pods` | Pods/restarts this replica holds data for. Used by `query` for targeted fan-out. |
| GET | `/internal/v1/pods/{pod-restart}/dictionary` | Same shape as `/api/v1/pods/{pod-restart}/dictionary` (§2.6). For live pod-restarts hosted by this replica. |
| GET | `/internal/v1/calls` | Same params as `/api/v1/calls`; returns only rows this replica holds. |
| GET | `/internal/v1/calls/{pk}` | Single-row fetch from this replica. |
| GET | `/internal/v1/calls/{pk}/trace` | Blob from this replica. |
| GET | `/internal/v1/health/hot-window` | `{ "hot_window_oldest_ms": ..., "hot_window_now_ms": ... }`. Reports the earliest `ts_ms` this replica still serves. Lets `query` compute the cold cutoff dynamically (§4.3). |

Sources this replica reads from when serving `/internal/v1/calls`:

1. Open parquet writers (one per retention class for the current time bucket).
2. Local parquet files in `parquet-pending/` (post-flush, pre-deletion; retained until `hot_retention` past flush — see §4.2).
3. The SQLite metadata DB (`metadata.sqlite`) for indexing the above; not a data source itself.

The replica never reads from S3 to serve `/internal/v1/*`. S3 is `query`'s job via the cold path.

## 4. Hot / cold model

### 4.1 Tiers

- **Hot tier** = collector replicas. Each holds data for its assigned pod-restarts (sticky TCP) in: in-memory accumulators, open parquet writers, and already-flushed parquet files retained on the local PV for `hot_retention - flush_interval` past flush.
- **Cold tier** = S3. Authoritative copy of everything flushed.

### 4.2 Hot retention on PV

`PROFILER_HOT_RETENTION` (default `15m`) is how long each collector keeps flushed parquet files locally past their flush. Must satisfy `hot_retention ≥ flush_interval + overlap_margin`.

This is the standard pattern (Prometheus head block + WAL, VictoriaMetrics in-memory + on-disk, Loki ingester + object storage): the hot tier intentionally overlaps with cold for a grace window. Queries cover both tiers, dedup by PK, and obtain a consistent view with no "gap risk" between flush completion and cold visibility.

Local parquet lifecycle:

1. Parquet writer closes a file when its time bucket ends (`01-write-contract.md` §6.1).
2. File is uploaded to S3 (idempotent key, §7 of write contract).
3. On `200 OK`, chunks staging refcounts are decremented (`01-write-contract.md` §6.2). **The local file is NOT deleted yet.**
4. A janitor goroutine on the replica deletes the local file when `now > flush_ts + hot_retention`.

### 4.3 Overlap and cutoff

Given a query for `[from, to]`:

- **Hot fan-out:** every collector replica is queried for `[max(from, replica.hot_window_oldest_ms), to]`.
- **Cold LIST:** S3 read for `[from, min(to, now - hot_retention + overlap_margin)]`.
- **Overlap window** = `[now - hot_retention, now - hot_retention + overlap_margin]`.
- After merge, dedup by PK (§6) collapses any overlap.

`overlap_margin` defaults to one `flush_interval` (5 min). Tunable via `PROFILER_OVERLAP_MARGIN`.

Result: every flushed Call is visible from at least one tier from the moment it leaves in-memory state, and from BOTH tiers for `overlap_margin` after that. Zero-gap guarantee, bounded duplication cost.

## 5. S3 LIST-based discovery

### 5.1 Time range → S3 prefixes

Path layout (`01-write-contract.md` §7):

```
s3://<bucket>/parquet/v1/<retentionClass>/<yyyy>/<mm>/<dd>/<hh>/<replica>-<podRestartHash>-<timeBucketStart>-<seq>.parquet
```

For range `[t1, t2]`:

1. For each retention class in the filter (default: all 5).
2. Walk the hour list between `floor(t1, 1h)` and `ceil(t2, 1h)`.
3. LIST each `<retentionClass>/<yyyy>/<mm>/<dd>/<hh>/` prefix in parallel.
4. Filter filenames by `<timeBucketStart>` ∈ `[t1, t2]` (encoded in the filename — no footer read required).

### 5.2 Parallelism

Up to `PROFILER_S3_LIST_CONCURRENCY` (default `16`) parallel LIST calls per query. Tens to hundreds of LISTs for multi-day ranges.

### 5.3 Manifest deferred

Per Stage 0 decision (`stage0-progress.md`, 2026-04-23): start with LIST. Add an S3 manifest file only if LIST profiles slow at scale.

## 6. Deduplication

### 6.1 PK as dedup key

PK (§2.2) is unique per Call across all time and all replicas.

### 6.2 When duplicates appear

- **Overlap window (expected).** Same Call visible from both hot (collector local parquet) and cold (S3) during `hot_retention - flush_interval ≤ age < hot_retention`. This is the normal case.
- **Replica transition (rare).** During scale-out or pod restart, sticky TCP may temporarily land an agent on two replicas in quick succession. Each writes its own copy of the affected Calls. Both copies have the same PK; dedup collapses to one.
- **Upload retry.** The S3 PUT object key is deterministic (`01-write-contract.md` §7), so retries overwrite the same key — no S3-side duplicates.

### 6.3 Tiebreaker

When merge sees the same PK from multiple sources, prefer **cold (S3)** over hot. Both copies are byte-identical (same parquet file uploaded once), so the choice is for determinism, not correctness. If only hot has it, use hot.

### 6.4 Why `query` always dedups

Even when sticky TCP routes one agent to one replica, `query` never relies on that for correctness. Replica failover, network partitions, or transient routing flaps can violate the "one source" assumption. Dedup by PK costs ~O(rows) hashing on merge and protects against all these cases.

## 7. Fan-out

### 7.1 Discovery

`query` reads `COLLECTOR_HEADLESS_SVC` env (e.g. `collector-headless.profiler.svc.cluster.local`). `net.LookupHost` returns one A record per Ready collector pod. Re-resolved on every external request (OS DNS cache typically capped at the CoreDNS TTL, ~30s).

### 7.2 Parallel requests

For each hot fan-out, `query` issues N parallel GETs. Per-request timeout `PROFILER_FANOUT_TIMEOUT` (default `2s`). Replicas exceeding timeout produce a `partial_reasons` entry but do not fail the whole query.

### 7.3 Replica targeting (optimization)

For queries that filter by `pod=...`, `query` calls `/internal/v1/pods` on each replica first to learn which pods that replica holds, then skips replicas with no relevant data. Optional optimization for large clusters; off by default.

### 7.4 Partial results

If at least one replica or S3 LIST fails:

- `partial: true` in the response.
- `partial_reasons: [...]` lists what failed (replica IP, S3 LIST prefix, exception summary).
- The client shows results with an explicit "data may be incomplete" indicator.

A profiler is most useful when at least partial data is shown — failing the whole query because one replica is slow defeats the purpose.

## 8. Error responses

RFC 7807 Problem Details for actual errors (parameter validation, internal, downstream failures that produce zero data).

| HTTP | Condition |
|---|---|
| 400 | Query parameter validation failed. |
| 404 | PK not found, or `trace_blob = NULL` (blob endpoint). |
| 503 | `query` itself is not Ready (e.g., DNS discovery uninitialized). |
| 504 | All replicas AND S3 LIST timed out — no data available at all. |

Partial results (some sources failed but some succeeded) are NOT errors — `partial: true` in the body. See §7.4.

## 9. Configuration

### Collector

| Env | Default | Description |
|---|---|---|
| `PROFILER_HOT_RETENTION` | `15m` | Local parquet retention past flush (§4.2). |
| `PROFILER_INTERNAL_API_PORT` | `8081` | Bind for `/internal/v1/*`. |

### Query

| Env | Default | Description |
|---|---|---|
| `COLLECTOR_HEADLESS_SVC` | — | DNS name for collector replica discovery (§7.1). |
| `PROFILER_OVERLAP_MARGIN` | `5m` | Hot/cold overlap window size (§4.3). |
| `PROFILER_FANOUT_TIMEOUT` | `2s` | Per-replica hot read timeout (§7.2). |
| `PROFILER_S3_LIST_CONCURRENCY` | `16` | Parallel S3 LIST cap (§5.2). |
| `PROFILER_EXTERNAL_API_PORT` | `8080` | Bind for `/api/v1/*`. |
| `S3_ENDPOINT` / `S3_BUCKET` / `S3_ACCESS_KEY` / `S3_SECRET_KEY` | — | Same as in `01-write-contract.md` §9. |

## 10. What this contract does NOT cover

- **Streaming / tail endpoint** for real-time observation — out of scope for MVP, defer to a future iteration once the batch model is stable.
- **Authentication and authorization** — out of scope for MVP. The likely choice (Keycloak/Bearer) will be a thin layer in front of `/api/v1/*`.
- **`/stats` full schema** — sketched in §2.6, full specification deferred to Stage 4.
- **Diagnostic dumps endpoint** — already served by `dumps-collector`, not re-implemented here.
- **UI specifics** — UI consumes `/api/v1/*` as documented; UI-internal data flow is Stage 5.

## 11. Review checklist

- [x] Endpoint paths and parameter naming conventions — accepted.
- [x] `partial: true` response shape — accepted. Client handles partial response and shows a warning indicator instead of failing the whole query.
- [x] Default `hot_retention` (`15m`) and `overlap_margin` (`5m`) — accepted.
- [x] Trace blob caching policy (immutable + 1y) — accepted.
- [x] `cutoff=strict` escape hatch — dropped from MVP; can be added later if a concrete consumer appears.
- [x] PK URL serialization with `:` separator — accepted; k8s pod/service names cannot contain `:`.
- [x] Server-decoded `/tree` endpoint — included as canonical; MessagePack with int-keyed maps and a `v` version envelope (§2.5).
- [x] Dictionary cold-path lifecycle — final snapshot uploaded to S3 on pod-restart close; see `01-write-contract.md` §3.6.
- [x] Optional `/internal/v1/pods` targeting (§7.3) — implemented in collector in Stage 1b; left dormant in `query` until cluster size makes it worthwhile.
