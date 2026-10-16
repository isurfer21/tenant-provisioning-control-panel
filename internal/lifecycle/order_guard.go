package lifecycle

import (
	"fmt"
	"sync"
	"time"
)

// OrderGuard prevents out-of-order event processing for task provisioning.
// T23: Order-Tolerant Processing Guard — validates that events arrive in a
// reasonable sequence and rejects stale updates (e.g. "done" before any
// "in_progress" has been observed, or newer timestamps arriving after older ones).
type OrderGuard struct {
	mu       sync.RWMutex
	tasks    map[string]taskState // taskID -> latest known state
	maxStale time.Duration        // tolerance window (default 30s)
}

// taskState tracks the last seen event per task.
type taskState struct {
	status   string // "accepted" | "in_progress" | "done" | "failed"
	ts       time.Time
	seq      int64 // monotonically increasing sequence number
}

// NewOrderGuard creates a guard with max stale tolerance of 30 seconds.
func NewOrderGuard() *OrderGuard {
	return &OrderGuard{
		tasks:    make(map[string]taskState),
		maxStale: 30 * time.Second,
	}
}

// Validate checks whether a new event for the given task is acceptable.
// Returns nil if the event should be processed; returns an explanation if it
// should be rejected (stale or out of sequence).
func (g *OrderGuard) Validate(taskID, status string, ts time.Time, seq int64) error {
	if ts.IsZero() {
		return fmt.Errorf("order guard: zero timestamp")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	prev, exists := g.tasks[taskID]
	if !exists {
		// First event for this task — accept it as baseline.
		g.tasks[taskID] = taskState{status: status, ts: ts, seq: seq}
		return nil
	}

	stale := ts.Sub(prev.ts)
	if stale < -g.maxStale {
		return fmt.Errorf("order guard: event for %s is %.0fs behind latest (seq %d → %d)",
			taskID, -stale.Seconds(), prev.seq, seq)
	}

	// Reject if: same or later timestamp but sequence went backwards.
	if ts.After(prev.ts) && seq <= prev.seq {
		return fmt.Errorf("order guard: %s stale sequence (got %d, latest %d)",
			taskID, seq, prev.seq)
	}

	// Validate state transitions are sensible.
	if !validTransition(prev.status, status) {
		return fmt.Errorf("order guard: invalid transition %s → %s for task %s",
			prev.status, status, taskID)
	}

	// Accept and update state.
	g.tasks[taskID] = taskState{status: status, ts: ts, seq: seq}
	return nil
}

// validTransition checks if a status transition is sensible.
func validTransition(from, to string) bool {
	switch from {
	case "accepted":
		return to == "in_progress" || to == "failed" // can start or abort immediately
	case "in_progress":
		return to == "done" || to == "in_progress" // continue or re-enter (idempotent)
	case "done", "failed":
		return false // terminal states — no further transitions allowed
	default:
		return true // unknown state from prev event — accept for forward compatibility
	}
}
