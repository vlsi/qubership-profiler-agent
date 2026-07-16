package vdumper

import (
	"time"

	"github.com/Netcracker/qubership-profiler-backend/libs/emulator"
)

// ParamDef is one record of the params stream: the agent's parameter metadata
// (Dumper.prepareParamInfoStream; decoded by libs/parser/pipe/params.go).
type ParamDef struct {
	Name      string
	Index     bool
	List      bool
	Order     int
	Signature string
}

// Config parameterizes one emulated pod. Every load run records its full
// parameter set so runs stay comparable (load-testing-plan.md §4). The zero
// value is not runnable; New applies the defaults documented per field.
type Config struct {
	// Identity sent in the GET_PROTOCOL_VERSION_V2 handshake.
	Namespace, Service, PodName string

	// Connection carries the collector address and socket timeouts. The
	// read timeout defaults to the agent's PLAIN_SOCKET_READ_TIMEOUT (30 s).
	Connection emulator.ConnectionOpts

	// FlushInterval is the wall-clock cadence of the dumper flush cycle
	// (STREAM_FLUSH_INTERVAL, default 5 s).
	FlushInterval time.Duration
	// BufferStealInterval claims non-empty producer buffers that did not fill
	// (BUFFER_STEAL_INTERVAL, default 5 s).
	BufferStealInterval time.Duration
	// RestartInterval is the sleep between dumper incarnations after a failure
	// (DUMPER_RESTART_INTERVAL, default 10 s).
	RestartInterval time.Duration

	// DictionaryInitial is how many synthetic dictionary words the pod knows
	// at startup; the whole set is re-sent after every reconnect (default
	// 2000).
	DictionaryInitial int

	// Params is the params-stream payload, one-shot per connection; nil takes
	// a minimal default set.
	Params []ParamDef

	// Clock defaults to the wall clock; Stats defaults to a no-op listener.
	Clock Clock
	Stats StatsListener
}

func (c Config) withDefaults() Config {
	if c.FlushInterval == 0 {
		c.FlushInterval = 5 * time.Second
	}
	if c.BufferStealInterval == 0 {
		c.BufferStealInterval = 5 * time.Second
	}
	if c.RestartInterval == 0 {
		c.RestartInterval = 10 * time.Second
	}
	if c.DictionaryInitial == 0 {
		c.DictionaryInitial = 2000
	}
	if c.Params == nil {
		c.Params = []ParamDef{
			{Name: "request.id", Index: true},
			{Name: "call.red", Index: true},
		}
	}
	if c.Connection.Timeout.ConnectTimeout == 0 {
		c.Connection.Timeout.ConnectTimeout = 10 * time.Second
	}
	if c.Connection.Timeout.SessionTimeout == 0 {
		c.Connection.Timeout.SessionTimeout = 24 * time.Hour
	}
	if c.Connection.Timeout.ReadTimeout == 0 {
		c.Connection.Timeout.ReadTimeout = 30 * time.Second
	}
	if c.Connection.Timeout.WriteTimeout == 0 {
		c.Connection.Timeout.WriteTimeout = 5 * time.Second
	}
	if c.Clock == nil {
		c.Clock = RealClock()
	}
	if c.Stats == nil {
		c.Stats = NoopStats{}
	}
	return c
}
