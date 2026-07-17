package main

import (
	"fmt"
	"time"
)

// finding is one invariant failure at one evaluation tick, tied to the
// subject it judged (a target URL, an S3 prefix, a marker PK, a pod name).
type finding struct {
	subject string
	msg     string
}

// violationRecord is one latched violation: a (invariant, subject) pair that
// failed at least once after warm-up. It never clears (doc/checker.md).
type violationRecord struct {
	invariant string
	plan      string
	subject   string
	firstAt   time.Time
	lastAt    time.Time
	count     int
	lastMsg   string
}

// latch accumulates violations for the whole run; the exit code and the
// final report come from here, not from the last evaluation pass.
type latch struct {
	recs  map[string]*violationRecord
	order []string
}

func newLatch() *latch {
	return &latch{recs: map[string]*violationRecord{}}
}

// record latches one finding. It returns the record and whether it is new,
// so the caller can print new violations louder than repeats.
func (l *latch) record(at time.Time, inv invariant, f finding) (*violationRecord, bool) {
	key := inv.name + "|" + f.subject
	if rec, ok := l.recs[key]; ok {
		rec.lastAt = at
		rec.count++
		rec.lastMsg = f.msg
		return rec, false
	}
	rec := &violationRecord{
		invariant: inv.name, plan: inv.plan, subject: f.subject,
		firstAt: at, lastAt: at, count: 1, lastMsg: f.msg,
	}
	l.recs[key] = rec
	l.order = append(l.order, key)
	return rec, true
}

func (l *latch) len() int { return len(l.recs) }

// report prints every latched violation in first-seen order.
func (l *latch) report() {
	for _, key := range l.order {
		rec := l.recs[key]
		fmt.Printf("VIOLATION %s (%s) %s: %s [first %s, last %s, %d times]\n",
			rec.invariant, rec.plan, rec.subject, rec.lastMsg,
			rec.firstAt.Format(time.RFC3339), rec.lastAt.Format(time.RFC3339), rec.count)
	}
}
