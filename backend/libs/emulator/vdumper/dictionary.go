package vdumper

import "fmt"

// dictionary models ProfilerData's tag list: an append-only word list whose
// ids are implied by position. The sent counter is per-connection — every
// reconnect resets it to zero so the whole dictionary is re-sent
// (Dumper.initialize), which is what resetRequired=1 tells the collector.
type dictionary struct {
	words []string
	sent  int
}

func newDictionary(initial int) *dictionary {
	d := &dictionary{}
	for i := 0; i < initial; i++ {
		d.words = append(d.words, syntheticWord(i))
	}
	return d
}

// syntheticWord shapes one dictionary entry like an instrumented-method tag:
// realistic length (~70 chars) matters for the bytes/s calibration, content
// does not.
func syntheticWord(i int) string {
	return fmt.Sprintf("void com.load.gen.Svc%04d.op%02d(int) (SyntheticApp.java) [synthetic-app.jar]",
		i/16, i%16)
}
