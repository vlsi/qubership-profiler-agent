// Package envconfig parses the profiler-backend configuration from the
// environment, following the catalogues of 01-write-contract.md §9,
// 02-read-contract.md §9, and 03-lifecycle.md §10. Only the knobs the
// composed services actually honour are parsed: accepting an env var the
// process would ignore misleads operators, so the ones that belong to
// unshipped features (memory budget, hot retention, seal concurrency, ...)
// stay unparsed until those features land.
package envconfig

import (
	"strconv"
	"strings"
	"time"

	"github.com/Netcracker/qubership-profiler-backend/libs/s3"
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
)

type (
	// Collect is the `collect` subcommand configuration.
	Collect struct {
		LogLevel string `envconfig:"PROFILER_LOG_LEVEL" default:"info"`

		DataDir             string             `envconfig:"PROFILER_DATA_DIR" default:"/data"`
		AgentPort           int                `envconfig:"PROFILER_AGENT_PORT" default:"1715"`
		InternalAPIPort     int                `envconfig:"PROFILER_INTERNAL_API_PORT" default:"8081"`
		TimeBucket          time.Duration      `envconfig:"PROFILER_TIME_BUCKET" default:"5m"`
		TimeBucketGrace     time.Duration      `envconfig:"PROFILER_TIME_BUCKET_GRACE" default:"30s"`
		DictFsyncRecords    int                `envconfig:"PROFILER_DICT_FSYNC_RECORDS" default:"256"`
		DictFsyncInterval   time.Duration      `envconfig:"PROFILER_DICT_FSYNC_INTERVAL" default:"100ms"`
		DurationThresholds  DurationThresholds `envconfig:"PROFILER_DURATION_THRESHOLDS" default:"100ms,1s"`
		SegmentRotationSize ByteSize           `envconfig:"PROFILER_SEGMENT_ROTATION_SIZE" default:"4MB"`

		// Replica names this instance in sealed-file names and S3 keys
		// (01 §7). The StatefulSet passes its pod name (04 §3.2); outside k8s
		// the wiring falls back to HOSTNAME, then to the libs default.
		Replica string `envconfig:"STATEFULSET_ORDINAL"`

		// SealCheckInterval / UploadCheckInterval pace the background loops.
		// The contract defines the seal trigger (01 §6.1) but not the poll
		// cadence, so these two names are an implementation choice recorded
		// in stage1-progress.md.
		SealCheckInterval   time.Duration `envconfig:"PROFILER_SEAL_CHECK_INTERVAL" default:"15s"`
		UploadCheckInterval time.Duration `envconfig:"PROFILER_UPLOAD_CHECK_INTERVAL" default:"30s"`

		ShutdownDrainGrace time.Duration `envconfig:"PROFILER_SHUTDOWN_DRAIN_GRACE" default:"30s"`

		S3 S3
	}

	// Query is the `query` subcommand configuration (02 §9).
	Query struct {
		LogLevel string `envconfig:"PROFILER_LOG_LEVEL" default:"info"`

		ExternalAPIPort  int           `envconfig:"PROFILER_EXTERNAL_API_PORT" default:"8080"`
		CollectorService string        `envconfig:"COLLECTOR_HEADLESS_SVC"`
		CollectorPort    int           `envconfig:"PROFILER_INTERNAL_API_PORT" default:"8081"`
		OverlapMargin    time.Duration `envconfig:"PROFILER_OVERLAP_MARGIN" default:"5m"`
		FanoutTimeout    time.Duration `envconfig:"PROFILER_FANOUT_TIMEOUT" default:"2s"`
		ListConcurrency  int           `envconfig:"PROFILER_S3_LIST_CONCURRENCY" default:"16"`
		CursorTTL        time.Duration `envconfig:"PROFILER_CURSOR_TTL" default:"15m"`
		WideRangeLimit   time.Duration `envconfig:"PROFILER_WIDE_RANGE_LIMIT" default:"6h"`
		MaxScanFiles     int           `envconfig:"PROFILER_MAX_SCAN_FILES" default:"10000"`
		MaxScanBytes     ByteSize      `envconfig:"PROFILER_MAX_SCAN_BYTES" default:"2GB"`

		ShutdownDrainGrace time.Duration `envconfig:"PROFILER_SHUTDOWN_DRAIN_GRACE" default:"30s"`

		S3 S3
	}

	// S3 carries the object-store connection shared by both subcommands
	// (01 §9). The scheme of S3_ENDPOINT selects TLS; the path prefix is not
	// configurable — the seal pass bakes `parquet/v1` into every key (01 §7).
	S3 struct {
		Endpoint  string `envconfig:"S3_ENDPOINT" required:"true"`
		Bucket    string `envconfig:"S3_BUCKET" required:"true"`
		AccessKey string `envconfig:"S3_ACCESS_KEY" required:"true"`
		SecretKey string `envconfig:"S3_SECRET_KEY" required:"true"`
	}
)

// Params maps the env shape onto the libs/s3 connection parameters.
func (s S3) Params() s3.Params {
	p := s3.Params{
		Endpoint:        s.Endpoint,
		AccessKeyID:     s.AccessKey,
		SecretAccessKey: s.SecretKey,
		UseSSL:          strings.HasPrefix(s.Endpoint, "https://"),
		BucketName:      s.Bucket,
	}
	p.Prepare() // strips the scheme the UseSSL check just consumed
	return p
}

// ParseCollect reads the `collect` configuration from the environment.
func ParseCollect() (Collect, error) {
	var c Collect
	err := envconfig.Process("", &c)
	return c, errors.Wrap(err, "parse collect env")
}

// ParseQuery reads the `query` configuration from the environment.
func ParseQuery() (Query, error) {
	var q Query
	err := envconfig.Process("", &q)
	return q, errors.Wrap(err, "parse query env")
}

// ByteSize decodes the contract's size literals ("64MB", "2GB", plain
// bytes). Suffixes are powers of 1024; the IEC spellings (KiB, MiB, ...) are
// accepted as synonyms.
type ByteSize int64

var byteSuffixes = []struct {
	suffix string
	shift  uint
}{
	{"TIB", 40}, {"TB", 40}, {"T", 40},
	{"GIB", 30}, {"GB", 30}, {"G", 30},
	{"MIB", 20}, {"MB", 20}, {"M", 20},
	{"KIB", 10}, {"KB", 10}, {"K", 10},
	{"B", 0},
}

// Decode implements envconfig.Decoder.
func (b *ByteSize) Decode(value string) error {
	raw := strings.ToUpper(strings.TrimSpace(value))
	shift := uint(0)
	for _, s := range byteSuffixes {
		if strings.HasSuffix(raw, s.suffix) {
			raw, shift = strings.TrimSpace(strings.TrimSuffix(raw, s.suffix)), s.shift
			break
		}
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return errors.Errorf("byte size %q: want <non-negative integer>[KB|MB|GB|TB]", value)
	}
	if shift > 0 && n > (1<<63-1)>>shift {
		return errors.Errorf("byte size %q overflows int64", value)
	}
	*b = ByteSize(n << shift)
	return nil
}

// DurationThresholds decodes PROFILER_DURATION_THRESHOLDS: two ascending
// duration-class boundaries, "100ms,1s" (01 §6.4).
type DurationThresholds [2]time.Duration

// Decode implements envconfig.Decoder.
func (d *DurationThresholds) Decode(value string) error {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return errors.Errorf("duration thresholds %q: want two comma-separated durations", value)
	}
	for i, part := range parts {
		v, err := time.ParseDuration(strings.TrimSpace(part))
		if err != nil {
			return errors.Wrapf(err, "duration thresholds %q", value)
		}
		d[i] = v
	}
	if d[0] <= 0 || d[1] <= d[0] {
		return errors.Errorf("duration thresholds %q: want 0 < first < second", value)
	}
	return nil
}
