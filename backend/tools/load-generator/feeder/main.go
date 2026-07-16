// Command feeder drives a synthetic, contract-shaped trickle of agent traffic
// into a collector: N emulated pods connect over the real wire protocol and
// keep sending dictionary, trace, calls, and suspend streams on an interval.
//
// It exists to light up the load-stand observability (load-testing-plan.md
// §9 phase 1) — dashboards, /metrics, pprof — before the phase-2 virtual
// dumper delivers the real parameterized generator. It makes no attempt at
// dumper fidelity: one stream at a time, no backpressure handling, no
// reconnect cadence (§3 G1–G9 stay open).
//
// Usage against the local stand:
//
//	kubectl -n profiler-load port-forward svc/profiler-backend-collector-agent 1715:1715 &
//	go run ./tools/load-generator/feeder -addr localhost:1715 -pods 20 -interval 5s -duration 15m
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Netcracker/qubership-profiler-backend/libs/emulator"
	profio "github.com/Netcracker/qubership-profiler-backend/libs/io"
	"github.com/Netcracker/qubership-profiler-backend/libs/log"
	model "github.com/Netcracker/qubership-profiler-backend/libs/protocol"
	"github.com/Netcracker/qubership-profiler-backend/libs/tests/helpers/wire"
)

const (
	methodHandle = 0 // "com.example.Api.handle"
	methodQuery  = 1 // "com.example.Db.query"
	tagRequestID = 2 // "request.id"
)

var dictWords = []string{"com.example.Api.handle", "com.example.Db.query", "request.id"}

func main() {
	var (
		addr      = flag.String("addr", "localhost:1715", "collector agent address")
		pods      = flag.Int("pods", 20, "emulated pods (one TCP connection each)")
		interval  = flag.Duration("interval", 5*time.Second, "per-pod send cadence")
		duration  = flag.Duration("duration", 15*time.Minute, "total run time; 0 runs until SIGINT")
		namespace = flag.String("namespace", "load", "emulated k8s namespace")
		service   = flag.String("service", "load-svc", "emulated service name")
		logLevel  = flag.String("log-level", "info", "trace|debug|info|warning|error")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}
	ctx, err := log.SetLevelString(ctx, *logLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	fmt.Printf("feeder: %d pods -> %s every %s for %s\n", *pods, *addr, *interval, *duration)
	var wg sync.WaitGroup
	for i := 0; i < *pods; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pod := fmt.Sprintf("%s-%d", *service, i)
			// Stagger the starts so sends spread across the interval.
			jitter := time.Duration(rand.Int63n(int64(*interval)))
			select {
			case <-ctx.Done():
				return
			case <-time.After(jitter):
			}
			if err := feedPod(ctx, *addr, *namespace, *service, pod, *interval); err != nil && ctx.Err() == nil {
				fmt.Printf("feeder: pod %s: %v\n", pod, err)
			}
		}(i)
	}
	wg.Wait()
	fmt.Println("feeder: done")
}

// feedPod holds one agent connection and sends a small burst every interval.
// Any send error ends the pod — phase 1 observes the green path; reconnect
// behavior belongs to the phase-2 generator.
func feedPod(ctx context.Context, addr, namespace, service, pod string, interval time.Duration) error {
	ac := emulator.PrepareAgent(ctx, nil, nil, pod)
	err := ac.Prepare(emulator.ConnectionOpts{
		ProtocolAddress: addr,
		Timeout: profio.TcpTimeout{
			ConnectTimeout: 10 * time.Second,
			SessionTimeout: 24 * time.Hour,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   5 * time.Second,
		},
	}).Connect()
	if err != nil {
		return err
	}
	defer func() { _ = ac.Close() }()
	if err := ac.InitializeConnection(model.PROTOCOL_VERSION_V3, namespace, service, pod); err != nil {
		return err
	}

	seq := map[string]int{}
	send := func(stream string, data []byte) error {
		handle, err := ac.CommandInitStream(stream, seq[stream], false)
		if err != nil {
			return err
		}
		seq[stream]++
		for pos := 0; pos < len(data); pos += emulator.MaxBufSize {
			end := min(pos+emulator.MaxBufSize, len(data))
			if err := ac.CommandRcvData(stream, handle, data[pos:end]); err != nil {
				return err
			}
		}
		if err := ac.Flush(); err != nil {
			return err
		}
		return ac.WaitForAcks()
	}

	if err := send(model.StreamDictionary, wire.DictionaryStream(dictWords)); err != nil {
		return err
	}

	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
		base := time.Now().Add(-2 * time.Second).UnixMilli()
		// Durations spread across the clean-tier thresholds (100ms / 1s / 10s)
		// so every retention class sees rows.
		durShort := int64(20 + rand.Intn(70))
		durNormal := int64(150 + rand.Intn(700))
		durLong := int64(1_100 + rand.Intn(2_000))
		trace, offs := wire.TraceStream(base-1_000, []wire.TraceChunk{
			{ThreadId: 11, StartMs: base + 5, Events: []wire.TraceEvent{
				wire.Enter(0, methodHandle),
				wire.Tag(1, tagRequestID, fmt.Sprintf("req-%s-%d", pod, seq[model.StreamTrace])),
				wire.Enter(2, methodQuery), wire.Exit(int(durShort)),
				wire.Exit(int(durNormal)),
			}},
			{ThreadId: 22, StartMs: base + 40, Events: []wire.TraceEvent{
				wire.Enter(0, methodQuery), wire.Exit(int(durLong)),
			}},
		})
		calls := []wire.CallRecord{
			{DeltaMs: 5, Method: methodHandle, DurationMs: int(durNormal), ChildCalls: 1, ThreadName: "http-1",
				TraceFileIndex: 1, BufferOffset: int(offs[0]), RecordIndex: 0,
				Params: map[int][]string{tagRequestID: {fmt.Sprintf("req-%s-%d", pod, seq[model.StreamTrace])}}},
			{DeltaMs: 40, Method: methodQuery, DurationMs: int(durLong), ThreadName: "http-2",
				TraceFileIndex: 1, BufferOffset: int(offs[1]), RecordIndex: 0},
		}
		if err := send(model.StreamTrace, trace); err != nil {
			return err
		}
		if err := send(model.StreamCalls, wire.CallsStreamRecords(base, calls)); err != nil {
			return err
		}
		if err := send(model.StreamSuspend, wire.SuspendStream(base, []wire.SuspendEvent{
			{DeltaMs: 10, AmountMs: 2},
		})); err != nil {
			return err
		}
	}
}
