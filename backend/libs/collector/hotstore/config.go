// Package hotstore persists the demultiplexed agent streams on the collector's
// local PV: append-only WALs for dictionary/params/suspend and the raw Call
// records, gzip segments for the offset-addressable bulk streams (trace, sql,
// xml), and the SQLite metadata that indexes them. It implements the write-path
// side of backend/docs/design/01-write-contract.md §3-§4 and the recovery
// sequence of 03-lifecycle.md §3. Sealing, parquet, and S3 are out of scope
// until the seal pass lands.
package hotstore

import "time"

// Config carries the write-path knobs from 01-write-contract.md §9. Zero
// values fall back to the contract defaults via Normalize.
type Config struct {
	// DataDir is the PV root (PROFILER_DATA_DIR, default /data).
	DataDir string
	// TimeBucket is the call-index partition width (PROFILER_TIME_BUCKET).
	TimeBucket time.Duration
	// DictFsyncRecords / DictFsyncInterval bound the WAL fsync lag
	// (PROFILER_DICT_FSYNC_RECORDS / PROFILER_DICT_FSYNC_INTERVAL).
	DictFsyncRecords  int
	DictFsyncInterval time.Duration
	// DurationThresholds split clean calls into retention classes
	// (PROFILER_DURATION_THRESHOLDS, default 100ms,1s; see §6.4).
	DurationThresholds [2]time.Duration
}

// Normalize fills unset fields with the contract defaults.
func (c Config) Normalize() Config {
	if c.TimeBucket <= 0 {
		c.TimeBucket = 5 * time.Minute
	}
	if c.DictFsyncRecords <= 0 {
		c.DictFsyncRecords = 256
	}
	if c.DictFsyncInterval <= 0 {
		c.DictFsyncInterval = 100 * time.Millisecond
	}
	if c.DurationThresholds[0] <= 0 {
		c.DurationThresholds[0] = 100 * time.Millisecond
	}
	if c.DurationThresholds[1] <= 0 {
		c.DurationThresholds[1] = time.Second
	}
	return c
}

// Retention classes derived at write time from (duration, error_flag)
// (01-write-contract.md §6.4). The corrupted class is reserved and never
// populated in the MVP (§5.6).
const (
	RetentionShortClean  = "short_clean"
	RetentionNormalClean = "normal_clean"
	RetentionLongClean   = "long_clean"
	RetentionAnyError    = "any_error"
)

// RetentionClass classifies one call for parquet sharding and TTL.
func (c Config) RetentionClass(duration time.Duration, errorFlag bool) string {
	switch {
	case errorFlag:
		return RetentionAnyError
	case duration < c.DurationThresholds[0]:
		return RetentionShortClean
	case duration < c.DurationThresholds[1]:
		return RetentionNormalClean
	default:
		return RetentionLongClean
	}
}

// Bucket maps a call start time to its partition index,
// floor(ts_ms / TimeBucket) (03-lifecycle.md §3.2).
func (c Config) Bucket(tsMs int64) int64 {
	return tsMs / c.TimeBucket.Milliseconds()
}

// BucketStartMs is the inverse of Bucket for naming partitions.
func (c Config) BucketStartMs(bucket int64) int64 {
	return bucket * c.TimeBucket.Milliseconds()
}
