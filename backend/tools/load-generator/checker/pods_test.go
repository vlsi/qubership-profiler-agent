package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestartBudget(t *testing.T) {
	p := newPodState(0)

	// Baseline: pre-existing restart counts do not count against the budget.
	p.observe([]podInfo{
		{Name: "collector-0", UID: "a", Restarts: 2},
		{Name: "collector-1", UID: "b", Restarts: 0},
	})
	assert.Empty(t, p.findings())

	// One container restart over the baseline blows the zero budget.
	p.observe([]podInfo{
		{Name: "collector-0", UID: "a", Restarts: 3},
		{Name: "collector-1", UID: "b", Restarts: 0},
	})
	fs := p.findings()
	require.Len(t, fs, 1)
	assert.Contains(t, fs[0].msg, "collector-0 +1")

	// A budget of 1 absorbs exactly that restart.
	p = newPodState(1)
	p.observe([]podInfo{{Name: "collector-0", UID: "a", Restarts: 2}})
	p.observe([]podInfo{{Name: "collector-0", UID: "a", Restarts: 3}})
	assert.Empty(t, p.findings())
	p.observe([]podInfo{{Name: "collector-0", UID: "a", Restarts: 4}})
	assert.NotEmpty(t, p.findings(), "the second restart exceeds the budget of 1")
}

func TestReplacementPodAccounting(t *testing.T) {
	p := newPodState(0)
	p.observe([]podInfo{{Name: "collector-0", UID: "a", Restarts: 0}})

	// The pod is replaced: one replacement event, plus its own restarts.
	p.observe([]podInfo{{Name: "collector-0", UID: "b", Restarts: 1}})
	fs := p.findings()
	require.Len(t, fs, 1, "replacement + its restart = 2 events over a 0 budget")
	assert.Contains(t, fs[0].msg, "2 restart events")
	assert.Contains(t, fs[0].msg, "(replacement)")

	// A budget of 2 absorbs the replacement and its restart; the vanished
	// pod is paired with the replacement, so nothing is reported as gone.
	p = newPodState(2)
	p.observe([]podInfo{{Name: "collector-0", UID: "a", Restarts: 0}})
	p.observe([]podInfo{{Name: "collector-0", UID: "b", Restarts: 1}})
	assert.Empty(t, p.findings())
}

func TestPodGoneWithoutReplacement(t *testing.T) {
	p := newPodState(10) // a generous budget: only the disappearance matters
	p.observe([]podInfo{
		{Name: "collector-0", UID: "a", Restarts: 0},
		{Name: "collector-1", UID: "b", Restarts: 0},
	})
	p.observe([]podInfo{{Name: "collector-0", UID: "a", Restarts: 0}})

	fs := p.findings()
	require.Len(t, fs, 1)
	assert.Contains(t, fs[0].msg, "disappeared without a replacement")
	assert.Contains(t, fs[0].msg, "collector-1")
}

func TestBaselineIsFirstSuccessfulList(t *testing.T) {
	p := newPodState(0)
	assert.Empty(t, p.findings(), "no baseline, nothing judged")

	// The first successful list IS the baseline, whatever counts it carries.
	p.observe([]podInfo{{Name: "collector-0", UID: "a", Restarts: 7}})
	assert.Empty(t, p.findings())
}
