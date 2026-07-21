package hot

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Netcracker/qubership-profiler-backend/libs/query/budget"
	"github.com/Netcracker/qubership-profiler-backend/libs/query/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tracePK = model.PK{
	PodNamespace: "ns", PodService: "svc", PodName: "pod", RestartTimeMs: 1000,
	TraceFileIndex: 1, BufferOffset: 2, RecordIndex: 0,
}

// TestTraceBudgetedBody pins the §7.5 body reader on the blob endpoint: a
// declared size is charged up front and owned by the returned lease; a body
// past the cap is refused on the headers alone; a chunked body is read in
// pre-reserved steps and still lands under the lease.
func TestTraceBudgetedBody(t *testing.T) {
	blob := strings.Repeat("B", 4096)

	t.Run("content-length charged and owned", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", fmt.Sprint(len(blob)))
			_, _ = w.Write([]byte(blob))
		}))
		defer srv.Close()
		b := budget.New(1<<20, time.Second, budget.Hooks{})
		c := NewClient(time.Second, b)
		got, lease, found, err := c.Trace(context.Background(), srv.URL, tracePK)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, blob, string(got))
		assert.Equal(t, int64(len(blob)), lease.Held(), "the lease owns exactly the blob")
		lease.Release()
		assert.Equal(t, int64(0), b.Used())
	})

	t.Run("declared size past the cap refused on headers", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", fmt.Sprint(int64(maxHotBlobBytes)+1))
			// The handler never gets to stream the body: the client must bail
			// out on the headers.
			_, _ = w.Write([]byte("x"))
		}))
		defer srv.Close()
		b := budget.New(1<<20, time.Second, budget.Hooks{})
		c := NewClient(time.Second, b)
		_, _, _, err := c.Trace(context.Background(), srv.URL, tracePK)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds")
		assert.Equal(t, int64(0), b.Used())
	})

	t.Run("chunked body reserved before read", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// No Content-Length: the server flushes chunks.
			fl := w.(http.Flusher)
			for i := 0; i < 3; i++ {
				_, _ = w.Write([]byte(blob))
				fl.Flush()
			}
		}))
		defer srv.Close()
		b := budget.New(64<<20, time.Second, budget.Hooks{})
		c := NewClient(time.Second, b)
		got, lease, found, err := c.Trace(context.Background(), srv.URL, tracePK)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, strings.Repeat(blob, 3), string(got))
		assert.Equal(t, int64(3*len(blob)), lease.Held())
		lease.Release()
		assert.Equal(t, int64(0), b.Used())
	})

	t.Run("chunked denial under a saturated budget", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fl := w.(http.Flusher)
			_, _ = w.Write([]byte(blob))
			fl.Flush()
		}))
		defer srv.Close()
		b := budget.New(1<<20, 30*time.Millisecond, budget.Hooks{})
		blocker, err := b.Acquire(context.Background(), 1<<20)
		require.NoError(t, err)
		defer blocker.Release()
		c := NewClient(time.Second, b)
		_, _, _, err = c.Trace(context.Background(), srv.URL, tracePK)
		require.Error(t, err)
		assert.ErrorIs(t, err, budget.ErrExhausted)
		blocker.Release()
		assert.Equal(t, int64(0), b.Used())
	})
}

// TestCallsLeaseOwnsDecodedRows pins the hot list path: the lease returned
// with the rows is reconciled to the rows' accounting footprint, not the raw
// body size.
func TestCallsLeaseOwnsDecodedRows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"calls":[{"pk":{"pod_namespace":"ns","pod_service":"svc","pod_name":"pod",` +
			`"restart_time_ms":1000,"trace_file_index":1,"buffer_offset":2,"record_index":0},` +
			`"ts_ms":1234,"duration_ms":10,"method":"com.example.M.handle"}]}`))
	}))
	defer srv.Close()
	b := budget.New(1<<20, time.Second, budget.Hooks{})
	c := NewClient(time.Second, b)
	rows, lease, err := c.Calls(context.Background(), srv.URL, model.CallsQuery{FromMs: 0, ToMs: 2000}, nil, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	var want int64
	for i := range rows {
		want += model.RowFootprint(&rows[i])
	}
	assert.Equal(t, want, lease.Held(), "the lease is reconciled to the decoded rows")
	lease.Release()
	assert.Equal(t, int64(0), b.Used())
}
