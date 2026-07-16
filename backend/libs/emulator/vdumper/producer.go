package vdumper

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/Netcracker/qubership-profiler-backend/libs/emulator/wire"
)

// chunk is one producer buffer handoff: the trace operations of one thread
// plus the root calls that completed inside it — the LocalBuffer the dumper
// loop serializes as one logical trace chunk.
type chunk struct {
	threadId   uint64
	threadName string
	startMs    int64 // chunk start epoch (LocalBuffer.startTime)
	ops        []traceOp
	calls      []completedCall
}

// traceOp is one event to serialize. Plain events are final; a big-param op
// carries the value instead, because its (seq, offset) reference into the
// sql/xml stream exists only once the dumper writes the value out
// (Dumper.writeParam).
type traceOp struct {
	ev  wire.TraceEvent
	big *bigOp
}

type bigOp struct {
	tagId   int
	deltaMs int
	value   string
	dedup   bool // sql deduplicates through the agent cache; xml never does
}

// completedCall carries what the dumper needs to emit one calls record; the
// trace-file linkage (file index, buffer offset) is only known at
// serialization time and filled in there.
type completedCall struct {
	recordIndex int // index of the root ENTER within the chunk's ops
	method      int
	startMs     int64
	durationMs  int
	// callCount is the agent's per-call enter counter (ThreadState.calls):
	// every ENTER including the root, so a depth-3 stack counts 3.
	callCount int
	params    map[int][]string
	cpuMs     int64
	waitMs    int64
	memory    int64
}

// producer models one application thread: it generates root calls at a
// jittered rate, shapes them per the Workload knobs, and hands its buffer to
// the dumper when it fills (ChunkMaxBytes) or when the buffer-steal deadline
// claims a non-empty one — so chunks from different threads interleave on the
// wire the way stolen LocalBuffers do (§2.5).
//
// A call completes now and started duration ago; the calls record carries
// that true (start, duration) — the calls format's zig-zag start deltas may
// run backwards, and retention classes plus time buckets are computed from
// the record, so the load shape is exact. The trace events, whose in-chunk
// deltas are unsigned and monotonic, get time-compressed instead: a call's
// enter lands at the thread's last event time and its exit no later than
// "now". Only a human eyeballing a /tree of a synthetic long call would
// notice; the alternative — one chunk per overlapping call — floods the
// dumper queue as soon as rate × duration exceeds one.
//
// Producers run for the pod's whole life, across dumper reconnects: when the
// chunk queue is full (the dumper is down or behind), a handoff is dropped and
// counted, never blocked on — the agent's drop window.
type producer struct {
	threadId   uint64
	threadName string
	cfg        Config
	w          Workload
	clock      Clock
	rnd        *rand.Rand
	dict       *dictionary
	out        chan<- chunk
	stats      StatsListener

	ops         []traceOp
	calls       []completedCall
	estBytes    int
	chunkStart  int64 // epoch ms of the chunk's first event
	lastEventMs int64
	stealAt     time.Time

	callSeq int
	sqlPool []string
}

func newProducer(id int, cfg Config, dict *dictionary, out chan<- chunk) *producer {
	return &producer{
		threadId:   uint64(1000 + id),
		threadName: fmt.Sprintf("exec-%d", id),
		cfg:        cfg,
		w:          cfg.Workload,
		clock:      cfg.Clock,
		rnd:        rand.New(rand.NewSource(cfg.Seed + int64(id))), //nolint:gosec // load shape, not crypto
		dict:       dict,
		out:        out,
		stats:      cfg.Stats,
	}
}

func (p *producer) run(ctx context.Context) {
	interval := time.Duration(float64(time.Second) / p.cfg.CallsPerSecPerThread)
	nextCall := p.clock.Now().Add(p.jittered(interval))
	for {
		wake := nextCall
		if len(p.ops) > 0 && p.stealAt.Before(wake) {
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
			if p.estBytes >= p.cfg.ChunkMaxBytes {
				p.handoff()
			}
		}
		if len(p.ops) > 0 && !now.Before(p.stealAt) {
			p.handoff()
		}
	}
}

// jittered spreads the call cadence uniformly over [0.5, 1.5) of the interval
// so threads do not fire in lockstep.
func (p *producer) jittered(interval time.Duration) time.Duration {
	return interval/2 + time.Duration(p.rnd.Int63n(int64(interval)))
}

// addCall appends the trace events of one root call completing now, shaped by
// the Workload knobs. The record keeps the true retroactive start; the trace
// events are time-compressed onto the thread's monotonic event clock (see the
// type comment).
func (p *producer) addCall(now time.Time) {
	durMs := p.w.Duration.sampleMs(p.rnd)
	nowMs := now.UnixMilli()
	startMs := nowMs - int64(durMs)
	if len(p.ops) == 0 {
		if p.lastEventMs == 0 || p.lastEventMs > nowMs {
			p.lastEventMs = nowMs
		}
		p.chunkStart = p.lastEventMs
		p.stealAt = now.Add(p.cfg.BufferStealInterval)
	}
	enterMs := max(startMs, p.lastEventMs)
	exitMs := max(enterMs, min(nowMs, enterMs+int64(durMs)))
	depth := sampleDepth(p.rnd, p.w.StackDepthMean)
	method := p.dict.methodId(p.rnd)
	p.callSeq++

	rootIndex := len(p.ops)
	p.putEvent(wire.Enter(int(enterMs-p.lastEventMs), method))

	params := map[int][]string{}
	if p.rnd.Float64() < p.w.RequestIdShare {
		value := fmt.Sprintf("req-%s-%s-%d", p.cfg.PodName, p.threadName, p.callSeq)
		p.putEvent(wire.Tag(0, dictRequestId, value))
		params[dictRequestId] = []string{value}
	}
	if p.rnd.Float64() < p.w.ErrorShare {
		p.putEvent(wire.Tag(0, dictCallRed, "1"))
		params[dictCallRed] = []string{"1"}
	}
	if p.rnd.Float64() < p.w.Sql.Share {
		p.putBig(&bigOp{tagId: dictSql, value: p.sqlValue(), dedup: true})
	}
	if p.rnd.Float64() < p.w.Xml.Share {
		p.putBig(&bigOp{tagId: dictXml,
			value: syntheticValue(p.rnd, "xml:", sampleSize(p.rnd, p.w.Xml.MeanBytes))})
	}

	for i := 1; i < depth; i++ {
		p.putEvent(wire.Enter(0, p.dict.methodId(p.rnd)))
	}
	p.putEvent(wire.Exit(int(exitMs - enterMs))) // the leaf carries the call's time
	for i := 1; i < depth; i++ {
		p.putEvent(wire.Exit(0))
	}
	p.lastEventMs = exitMs
	p.calls = append(p.calls, completedCall{
		recordIndex: rootIndex,
		method:      method,
		startMs:     startMs,
		durationMs:  durMs,
		callCount:   depth,
		params:      params,
		cpuMs:       int64(durMs) * 6 / 10,
		waitMs:      int64(durMs) / 5,
		memory:      int64(sampleSize(p.rnd, 256*1024)),
	})
}

// sqlValue draws a statement from the reuse pool with DedupHitRate
// probability; reuse is what turns into (seq, offset) dedup references at the
// dumper's cache.
func (p *producer) sqlValue() string {
	if len(p.sqlPool) > 0 && p.rnd.Float64() < p.w.Sql.DedupHitRate {
		return p.sqlPool[p.rnd.Intn(len(p.sqlPool))]
	}
	v := syntheticValue(p.rnd, "sql:", sampleSize(p.rnd, p.w.Sql.MeanBytes))
	poolSize := p.w.Sql.PoolSize
	if poolSize <= 0 {
		poolSize = 100
	}
	if len(p.sqlPool) < poolSize {
		p.sqlPool = append(p.sqlPool, v)
	}
	return v
}

func (p *producer) putEvent(e wire.TraceEvent) {
	p.ops = append(p.ops, traceOp{ev: e})
	p.estBytes += 4 + len(e.Value)
}

func (p *producer) putBig(op *bigOp) {
	p.ops = append(p.ops, traceOp{big: op})
	p.estBytes += 12 + len(op.value)
}

func (p *producer) handoff() {
	c := chunk{
		threadId:   p.threadId,
		threadName: p.threadName,
		startMs:    p.chunkStart,
		ops:        p.ops,
		calls:      p.calls,
	}
	p.ops = nil
	p.calls = nil
	p.estBytes = 0
	select {
	case p.out <- c:
	default:
		// Drop window: the dumper is down or behind; the agent loses these
		// events the same way exhausted LocalBuffers lose theirs.
		p.stats.Dropped(1)
	}
}
