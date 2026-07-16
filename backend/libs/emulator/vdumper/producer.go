package vdumper

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/Netcracker/qubership-profiler-backend/libs/emulator/wire"
)

// chunk is one producer buffer handoff: the encoded events of one thread plus
// the root calls that completed inside it — the LocalBuffer the dumper loop
// serializes as one logical trace chunk.
type chunk struct {
	threadId   uint64
	threadName string
	startMs    int64  // chunk start epoch (LocalBuffer.startTime)
	events     []byte // encoded trace events, no chunk header, no FINISH
	calls      []completedCall
}

// completedCall carries what the dumper needs to emit one calls record; the
// trace-file linkage (file index, buffer offset) is only known at
// serialization time and filled in there.
type completedCall struct {
	recordIndex int // index of the root ENTER within the chunk's events
	method      int
	startMs     int64
	durationMs  int
	// callCount is the agent's per-call enter counter (ThreadState.calls):
	// every ENTER including the root, so a depth-3 stack counts 3.
	callCount int
}

// producer models one application thread: it generates root calls at a
// jittered rate, encodes their trace events into a private chunk buffer, and
// hands the buffer to the dumper when it fills (ChunkMaxBytes) or when the
// buffer-steal deadline claims a non-empty one — so chunks from different
// threads interleave on the wire the way stolen LocalBuffers do (§2.5).
//
// Producers run for the pod's whole life, across dumper reconnects: when the
// chunk queue is full (the dumper is down or behind), a handoff is dropped and
// counted, never blocked on — the agent's drop window.
type producer struct {
	threadId   uint64
	threadName string
	cfg        Config
	clock      Clock
	rnd        *rand.Rand
	out        chan<- chunk
	stats      StatsListener

	buf        *bytes.Buffer
	events     int
	calls      []completedCall
	chunkStart int64 // epoch ms of the chunk's first event
	lastEventMs int64
	stealAt    time.Time
}

func newProducer(id int, cfg Config, out chan<- chunk) *producer {
	return &producer{
		threadId:   uint64(1000 + id),
		threadName: fmt.Sprintf("exec-%d", id),
		cfg:        cfg,
		clock:      cfg.Clock,
		rnd:        rand.New(rand.NewSource(cfg.Seed + int64(id))), //nolint:gosec // load shape, not crypto
		out:        out,
		stats:      cfg.Stats,
		buf:        &bytes.Buffer{},
	}
}

func (p *producer) run(ctx context.Context) {
	interval := time.Duration(float64(time.Second) / p.cfg.CallsPerSecPerThread)
	nextCall := p.clock.Now().Add(p.jittered(interval))
	for {
		wake := nextCall
		if p.buf.Len() > 0 && p.stealAt.Before(wake) {
			wake = p.stealAt
		}
		select {
		case <-ctx.Done():
			return
		case <-p.clock.After(wake.Sub(p.clock.Now())):
		}
		now := p.clock.Now()
		if !now.Before(nextCall) {
			p.addCall(now)
			nextCall = nextCall.Add(p.jittered(interval))
			if p.buf.Len() >= p.cfg.ChunkMaxBytes {
				p.handoff()
			}
		}
		if p.buf.Len() > 0 && !now.Before(p.stealAt) {
			p.handoff()
		}
	}
}

// jittered spreads the call cadence uniformly over [0.5, 1.5) of the interval
// so threads do not fire in lockstep.
func (p *producer) jittered(interval time.Duration) time.Duration {
	return interval/2 + time.Duration(p.rnd.Int63n(int64(interval)))
}

// addCall appends the trace events of one root call that completes now. The
// shape is fixed at this stage (the workload knobs land with G7–G9): a root
// enter, two nested child calls, and the matching exits.
func (p *producer) addCall(now time.Time) {
	const durationMs = 30
	const depth = 3
	startMs := now.UnixMilli() - durationMs
	if p.buf.Len() == 0 {
		p.chunkStart = startMs
		p.lastEventMs = startMs
		p.stealAt = now.Add(p.cfg.BufferStealInterval)
	}
	if startMs < p.lastEventMs {
		startMs = p.lastEventMs // same thread: calls never overlap
	}
	method := p.rnd.Intn(max(1, p.cfg.DictionaryInitial))

	rootIndex := p.events
	p.putEvent(wire.Enter(int(startMs-p.lastEventMs), method))
	for i := 1; i < depth; i++ {
		p.putEvent(wire.Enter(0, (method+i)%max(1, p.cfg.DictionaryInitial)))
	}
	p.putEvent(wire.Exit(durationMs)) // the leaf carries the call's time
	for i := 1; i < depth; i++ {
		p.putEvent(wire.Exit(0))
	}
	p.lastEventMs = startMs + durationMs
	p.calls = append(p.calls, completedCall{
		recordIndex: rootIndex,
		method:      method,
		startMs:     startMs,
		durationMs:  durationMs,
		callCount:   depth,
	})
}

func (p *producer) putEvent(e wire.TraceEvent) {
	wire.PutTraceEvent(p.buf, e)
	p.events++
}

func (p *producer) handoff() {
	c := chunk{
		threadId:   p.threadId,
		threadName: p.threadName,
		startMs:    p.chunkStart,
		events:     append([]byte(nil), p.buf.Bytes()...),
		calls:      p.calls,
	}
	p.buf.Reset()
	p.events = 0
	p.calls = nil
	select {
	case p.out <- c:
	default:
		// Drop window: the dumper is down or behind; the agent loses these
		// events the same way exhausted LocalBuffers lose theirs.
		p.stats.Dropped(1)
	}
}
