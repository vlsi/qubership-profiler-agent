# Soak invariant checker

Contract for `tools/load-generator/checker`: the watchdog that runs alongside a contract or soak run
(`load-testing-plan.md` §7.1, §7.4) and fails the run when a §8 invariant breaks. This document defines the data
sources, the violation model, and the pass/fail rule of every invariant; the implementation must not be trusted over
this contract.

The checker is a standalone binary run next to the runner, against port-forwarded endpoints (`doc/soak-runs.md` lists
the per-stand commands). It takes ready URLs and credentials through flags and environment variables and never
manages port-forwards or deploys anything.

## Violation model: latched, not last-seen

A soak verdict must reflect the whole run, not the final sample. Every invariant is a pure predicate over the current
state; the harness around it latches failures:

- Each poll tick evaluates every enabled invariant. A failed evaluation after warm-up creates (or updates) a latched
  violation record: `{invariant, subject, firstAt, lastAt, count, message}`. `subject` is the target URL, S3 prefix,
  marker PK, or pod name the invariant judged.
- A latched violation never clears. A collector whose RSS spiked over the limit for one tick fails the run even when
  the final tick looks healthy.
- Live output prints a violation line when a record is created and when it repeats; the final report prints every
  latched record with its first/last time and repeat count.
- Exit code is 0 only when the latch registry is empty at the end of the run; the final evaluation pass no longer
  decides anything on its own.

Samples taken during warm-up (`-warmup`, default 15m) are stored but never judged, as before.

## Sources

Each source is optional and enabled by its flags; with none of the new flags set the checker behaves as the §8.1–§8.4
metrics watcher.

| Source | Enables | Flags / env | Cadence |
| --- | --- | --- | --- |
| `/metrics` scrape | §8.1–§8.4, §8.6 | `-targets` (required) | `-interval` (30s) |
| S3 listing | §8.5 | `S3_ENDPOINT`, `S3_BUCKET`, `S3_PATH_PREFIX`, `S3_ACCESS_KEY[_FILE]`, `S3_SECRET_KEY[_FILE]` (same names the collector reads); enabled by `-s3` | `-s3-interval` (2m) |
| Query API probe | §8.7 | `-query-url` | `-interval` |
| Kubernetes pod list | §8.8 | `-kube-namespace` (enables), `-kube-selector` (default `app.kubernetes.io/name=profiler-backend`), kubeconfig or in-cluster config | `-interval` |

The S3 client is read-only: it must not create buckets or write objects (`s3.NewClient` calls `MakeBucket` and is
therefore not usable here — the checker builds a bare client from the same `s3.Params`).

### Scrape gaps

A target (metrics URL, S3 endpoint, query URL, or the k8s API) that fails for more than `-max-scrape-gap` consecutive
polls (default 3) after warm-up latches a `target-unavailable` violation. Silent absence must not hide a dead
component: §8.1–§8.7 skip a tick they have no data for, and this rule is what makes that skip safe.

## Invariants

### §8.1–§8.4 (implemented in phase 1, unchanged)

- **§8.1 hot-store-not-growing**: the per-target sum of the hot-store disk gauges must not grow monotonically by more
  than 5% over the `-window` (2h default).
- **§8.2 ingest-paused-not-sticky**: `profiler_backpressure_ingest_paused` nonzero in ≥ 1% of post-warm-up samples.
- **§8.3 no-refused-bytes**: `profiler_ingest_refused_bytes_total` must stay 0 at contract load.
- **§8.4 hot-window-lag-bounded**: `profiler_hotstore_hot_window_lag_seconds` must stay under `-max-hot-lag`.
  The gauge is the age of the **oldest** row still in the hot index, so its healthy level is
  `hot retention + eviction cadence`, not the seal latency; the budget is
  `hot retention + seal/upload chain + margin` (default 25m against the 15m default retention). Sustained growth
  past the budget means the hot→cold handoff is stuck.

These now latch like everything else; their predicates are unchanged.

### §8.5 S3 objects per hour prefix

Judges the listing of `parquet/v1/<class>/<yyyy>/<mm>/<dd>/<hh>/…` under the configured bucket and path prefix. Keys
are parsed with the same right-to-left rule as `libs/maintain` (`parseParquetKey`); keys that do not parse are
ignored, mirroring maintain's own tolerance for foreign objects.

The checker must not demand compaction before maintain could have run. Deadlines derive from the stand's timers,
passed as flags with the same names and defaults as the backend env
(`-time-bucket` 5m, `-time-bucket-grace` 30s, `-seal-check-interval` 15s, `-upload-check-interval` 30s,
`-maintain-check-interval` 5m, `-compaction-min-age` 30m, `-compaction-min-files` 4, `-compaction-delete-grace` 5m,
`-s3-settle-slack` 1m):

```text
objectsVisibleAt(bucket) = bucketEnd + timeBucketGrace + sealCheckInterval + uploadCheckInterval + settleSlack
compactionDueAt(bucket)  = max(objectsVisibleAt(bucket), bucketEnd + compactionMinAge)
                           + 2 × maintainCheckInterval + compactionDeleteGrace + settleSlack
```

Two maintain intervals, not one: a group settling right after a pass starts waits for the next pass to compact it,
and the write → grace → delete protocol removes its inputs one pass later still; back-to-back passes can also run
longer than the interval, which the slack absorbs.

Two sub-invariants:

1. **Compaction keeps up.** For every `(bucket, class)` group with `now ≥ compactionDueAt(bucket)`, the object count
   in the group must be ≤ `compactionMinFiles` (compaction residue below the trigger is legal by design). Groups
   before their deadline are never judged.
2. **Small-file share trends down.** Per hour prefix, the share of objects smaller than `-s3-small-file-bytes`
   (default 1 MB) is sampled on every listing. Once every bucket of the hour is past `compactionDueAt`, a share that
   grows monotonically across the sliding `-window` latches a violation. The window slides: one early drop cannot
   mask later unbounded growth.

Object-count and small-file series are also written to the checker log per listing, so a soak report can cite them.

### §8.6 collector RSS and goroutines

RSS, per metrics target: `process_resident_memory_bytes` must stay under `-rss-limit-bytes` (the pod memory limit;
required to enable the check — the limit is not exposed on `/metrics`), and must not grow monotonically over the
`-window` (same 5% tolerance as §8.1).

Goroutine flatness, per metrics target that exposes `profiler_ingest_active_connections` (the collector; maintain and
query are exempt from this half): only samples where the same scrape returned both `go_goroutines` and the connection
gauge are used, so both series share time points. Over the `-window`:

```text
connections constant  ⇔  range(conns) ≤ max(0.01 × mean(conns), 2)
goroutines flat       ⇔  range(goroutines) ≤ max(tolerance × mean(goroutines), 10)
                          (tolerance: -goroutine-tolerance, default 0.10; the absolute floor of 10
                           keeps near-idle processes out of the noise)
```

The invariant fires when connections are constant and goroutines are not flat. When connections move, the tick is not
judged — §8.6 pins the leak signal, not the connection churn.

### §8.7 sampled UI queries

All time windows are computed by the checker and sent as integer Unix milliseconds — `/api/v1` accepts nothing else
(`ParseWindow`, `libs/query/model/wire.go`).

- **Freshness**: every tick, `GET /api/v1/calls?from=<ms(now−probe)>&to=<ms(now)>&limit=…`; the newest `ts_ms` must
  be younger than `-freshness-budget` (default: `-max-hot-lag`). While the generator is feeding, the hot window must
  keep serving fresh rows.
- **Markers**: right after warm-up the checker samples `-marker-count` rows (default 20) from the earliest
  post-start window and records `(pk, ts_ms, retention_class)`. The `corrupted` class is excluded: it is reserved
  and no writer produces it today (`libs/query/model/tiers.go`). Every tick each marker is fetched with
  `GET /api/v1/calls/<pk>/trace?ts_ms=<ts>&retention_class=<class>`:
  - a 404 or 5xx for a marker younger than `classTTL − ttlMargin` latches a violation (old data must remain
    retrievable from cold until its TTL);
  - a marker older than its class TTL leaves the set.
  Class TTLs come from the same `PROFILER_RETENTION_*` environment variables the stand sets
  (`envconfig.Maintain.ClassTTLs()` semantics: unset keeps the tier-table default); `-ttl-margin` defaults to the
  §8.5 settle slack.
- **TTL deletion (optional)**: with `-expect-ttl-deletion`, a marker older than
  `classTTL + compactionDueAt-style settle` that still answers 200 latches a violation. Off by default; the
  accelerated-timer soak turns it on to validate the whole eviction chain.

### §8.8 no unexpected pod restarts

Kubernetes pods matching `-kube-namespace` + `-kube-selector` are listed every tick (client-go, kubeconfig or
in-cluster config — the `tools/migration/pkg/cleaner` pattern).

- **Baseline**: the first successful list. Restarts are not judged before it exists; if no list succeeds within
  `-max-scrape-gap` ticks, `kube-unavailable` latches.
- **Budget**: `-allowed-restarts` (default 0) is a total budget for the whole run:
  `Σ over containers of (restartCount − baselineRestartCount) + 1 per replacement pod`, where a replacement pod is a
  new UID matching the selector after the baseline; its own restart count adds on top. Exceeding the budget latches
  a violation naming the pods.
- A pod that disappears without a replacement latches its own violation.
- A failed list is treated like a scrape gap: the tick is not judged, the gap counter grows.

Phase 4 runs with the default budget of 0; the flag exists so phase 5 (T5 injected restarts) can raise it without
contract changes.

## Exit and report

`checker: PASS` and exit 0 only with an empty latch registry. Otherwise the final report lists every latched
violation (`invariant`, `subject`, first/last time, count, message) and the exit code is 1. Flag errors exit 2.
