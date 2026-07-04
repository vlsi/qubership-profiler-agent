package envconfig

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestByteSizeDecode(t *testing.T) {
	for raw, want := range map[string]ByteSize{
		"0":     0,
		"1234":  1234,
		"4MB":   4 << 20,
		"2GB":   2 << 30,
		"10GiB": 10 << 30,
		"64mb":  64 << 20,
		" 1 KB": 1 << 10,
	} {
		var b ByteSize
		require.NoError(t, b.Decode(raw), raw)
		assert.Equal(t, want, b, raw)
	}
	for _, raw := range []string{"", "MB", "-1KB", "1.5GB", "10EB"} {
		var b ByteSize
		assert.Error(t, b.Decode(raw), raw)
	}
}

func TestDurationThresholdsDecode(t *testing.T) {
	var d DurationThresholds
	require.NoError(t, d.Decode("100ms, 1s"))
	assert.Equal(t, DurationThresholds{100 * time.Millisecond, time.Second}, d)

	for _, raw := range []string{"", "1s", "1s,100ms", "0s,1s", "1s,1s,2s", "abc,1s"} {
		assert.Error(t, d.Decode(raw), raw)
	}
}

func TestCollectDefaults(t *testing.T) {
	t.Setenv("S3_ENDPOINT", "http://minio:9000")
	t.Setenv("S3_BUCKET", "profiler-data")
	t.Setenv("S3_ACCESS_KEY", "ak")
	t.Setenv("S3_SECRET_KEY", "sk")

	c, err := ParseCollect()
	require.NoError(t, err)
	assert.Equal(t, "/data", c.DataDir)
	assert.Equal(t, 1715, c.AgentPort)
	assert.Equal(t, 8081, c.InternalAPIPort)
	assert.Equal(t, 5*time.Minute, c.TimeBucket)
	assert.Equal(t, DurationThresholds{100 * time.Millisecond, time.Second}, c.DurationThresholds)
	assert.Equal(t, ByteSize(4<<20), c.SegmentRotationSize)
	// The loops must default ON: a collector that never seals or uploads is
	// not a collector (01 §6.1-§6.2).
	assert.Positive(t, c.SealCheckInterval)
	assert.Positive(t, c.UploadCheckInterval)

	p := c.S3.Params()
	assert.Equal(t, "minio:9000", p.Endpoint)
	assert.False(t, p.UseSSL)
	assert.Equal(t, "profiler-data", p.BucketName)
}

func TestQueryDefaults(t *testing.T) {
	t.Setenv("S3_ENDPOINT", "https://s3.example.com")
	t.Setenv("S3_BUCKET", "profiler-data")
	t.Setenv("S3_ACCESS_KEY", "ak")
	t.Setenv("S3_SECRET_KEY", "sk")
	t.Setenv("COLLECTOR_HEADLESS_SVC", "profiler-collector-headless")

	q, err := ParseQuery()
	require.NoError(t, err)
	assert.Equal(t, 8080, q.ExternalAPIPort)
	assert.Equal(t, "profiler-collector-headless", q.CollectorService)
	assert.Equal(t, 8081, q.CollectorPort)
	assert.Equal(t, ByteSize(2<<30), q.MaxScanBytes)
	assert.True(t, q.S3.Params().UseSSL)
}

func TestS3Required(t *testing.T) {
	// t.Setenv registers the restore; Unsetenv then truly clears the var —
	// envconfig's `required` accepts a set-but-empty value.
	for _, key := range []string{"S3_ENDPOINT", "S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY"} {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
	_, err := ParseCollect()
	assert.Error(t, err)
}
