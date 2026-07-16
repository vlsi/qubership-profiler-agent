package vdumper

// StatsListener receives the virtual dumper's observable events. The feeder
// exposes them as logs and metrics; the k6 module maps them to samples when
// phase 3 revives it. Callbacks run on the dumper goroutine — keep them cheap.
type StatsListener interface {
	// Connected fires after a successful handshake and stream setup.
	Connected(incarnation int)
	// Disconnected fires when an incarnation dies; err is the cause.
	Disconnected(incarnation int, err error)
	// StreamOpened fires per INIT_STREAM_V2, including rotations.
	StreamOpened(stream string, fileIndex int, reset bool)
	// BytesSent counts one RCV_DATA payload of the stream.
	BytesSent(stream string, n int)
	// AckError fires when the collector refused data with ACK_ERROR_MAGIC.
	AckError()
	// Dropped counts trace chunks lost while the dumper was down (the agent's
	// drop window: producers keep running, their output goes nowhere).
	Dropped(chunks int)
}

// NoopStats discards every event.
type NoopStats struct{}

func (NoopStats) Connected(int)                 {}
func (NoopStats) Disconnected(int, error)       {}
func (NoopStats) StreamOpened(string, int, bool) {}
func (NoopStats) BytesSent(string, int)         {}
func (NoopStats) AckError()                     {}
func (NoopStats) Dropped(int)                   {}
