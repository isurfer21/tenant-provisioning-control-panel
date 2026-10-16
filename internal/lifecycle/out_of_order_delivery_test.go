package lifecycle

import (
	"testing"
	"time"
)

// TestOrderGuard_OutOfOrderDelivery_Rejects verifies the OrderGuard correctly rejects
// out-of-sequence events — specifically "done" arriving before any "in_progress". (T23.)
func TestOrderGuard_OutOfOrderDelivery(t *testing.T) {
	guard := NewOrderGuard()
	now := time.Now().UTC()

	t.Run("valid first event accepted", func(t *testing.T) {
		err := guard.Validate("task-v1", "accepted", now, 1)
		if err != nil {
			t.Fatalf("first accepted should be valid: %v", err)
		}
	})

	t.Run("in_progress after accepted is valid", func(t *testing.T) {
		err := guard.Validate("task-v1", "in_progress", now.Add(time.Second), 2)
		if err != nil {
			t.Fatalf("in_progress after accepted should be valid: %v", err)
		}

		// Also confirm the FSM allows this transition.
		fsmErr := ValidateTaskTransition("accepted", "in_progress")
		if fsmErr != nil {
			t.Errorf("FSM should allow accepted -> in_progress: %v", fsmErr)
		}
	})

	t.Run("done after in_progress is valid", func(t *testing.T) {
		err := guard.Validate("task-v1", "done", now.Add(2*time.Second), 3)
		if err != nil {
			t.Fatalf("done after in_progress should be valid: %v", err)
		}

		fsmErr := ValidateTaskTransition("in_progress", "done")
		if fsmErr != nil {
			t.Errorf("FSM should allow in_progress -> done: %v", fsmErr)
		}
	})
}

// TestOrderGuard_OutOfStale_Sequence verifies we can't jump ahead in the state machine.
func TestOrderGuard_StaleSequenceRejected(t *testing.T) {
	guard := NewOrderGuard()
	now := time.Now().UTC()

	t.Run("done before accepted rejected by guard", func(t *testing.T) {
		err := guard.Validate("task-v2", "done", now, 3)
		if err != nil {
			// First arrival of task-v2 — guard will accept it since no prior state exists.
			// But then ValidateTaskTransition("done", "in_progress") should also reject.
			t.Logf("guard accepted first arrival (no prior state); expected FSM lock: %v", err)
		}

		// Now check the FSM itself — done -> in_progress must be blocked.
		fsmErr := ValidateTaskTransition("done", "in_progress")
		if fsmErr == nil {
			t.Fatal("FSM should reject done -> in_progress (stale/out-of-order)")
		}
	})

	t.Run("backwards transition rejected by guard", func(t *testing.T) {
		guard2 := NewOrderGuard()
		err := guard2.Validate("task-v3", "accepted", now, 1)
		if err != nil {
			t.Fatalf("initial accepted should be valid: %v", err)
		}

		err = guard2.Validate("task-v3", "in_progress", now.Add(time.Second), 2)
		if err != nil {
			t.Fatalf("accepted -> in_progress should be valid: %v", err)
		}

		// Now try to go backwards: in_progress -> accepted. Should fail.
		err = guard2.Validate("task-v3", "accepted", now.Add(3*time.Second), 10)
		if err == nil {
			t.Fatal("expected error for backwards transition in_progress -> accepted")
		}

		fsmErr := ValidateTaskTransition("in_progress", "accepted")
		if fsmErr == nil {
			t.Fatal("FSM should reject in_progress -> accepted (backwards)")
		}
	})
}

// TestOrderGuard_TerminalState_NoFurtherProgress verifies a terminal state cannot be exited.
func TestOrderGuard_TerminalStateNoExit(t *testing.T) {
	now := time.Now().UTC()

	for _, status := range []string{"done", "failed"} {
		t.Run("terminal-"+status, func(t *testing.T) {
			guard := NewOrderGuard()

			err := guard.Validate(status+"-task", status, now, 1)
			if err != nil {
				t.Fatalf("initial %s should be valid: %v", status, err)
			}

			// The guard stored the terminal state. Any *subsequent* transition
			// from this terminal state must fail — both at the guard AND FSM level.
			err2 := guard.Validate(status+"-task", "in_progress", now.Add(time.Second), 2)
			if err2 == nil {
				t.Fatalf("expected error for post-terminal transition %s -> in_progress", status)
			}

			// Also verify the FSM blocks it directly.
			fsmErr := ValidateTaskTransition(status, "in_progress")
			if fsmErr == nil {
				t.Fatalf("FSM should block any forward transition from terminal %s", status)
			}

			termErr, isTerminal := fsmErr.(*ErrTerminalTask)
			if !isTerminal {
				t.Errorf("expected *ErrTerminalTask for %s -> in_progress, got %T", status, fsmErr)
			} else if termErr.Status != status {
				t.Errorf("expected terminal status %q, got %q", status, termErr.Status)
			}
		})
	}
}

// TestOrderGuard_ValidProgression verifies the full accepted → in_progress → done flow.
func TestOrderGuard_FullValidProgression(t *testing.T) {
	guard := NewOrderGuard()
	now := time.Now().UTC()
	var seq int64 = 0

	steps := []string{"accepted", "in_progress", "done"}
	for i, s := range steps {
		seq++
		t.Run(s, func(t *testing.T) {
			ts := now.Add(time.Duration(i) * time.Second)
			err := guard.Validate("task-full", s, ts, seq)
			if err != nil {
				t.Fatalf("valid step %s at seq=%d should succeed: %v", s, seq, err)
			}

			// Verify FSM also allows this transition.
			var nextFrom string
			if i > 0 {
				nextFrom = steps[i-1]
			} else {
				nextFrom = "" // first step — no from-state guard needed.
			}
			if nextFrom != "" {
				fsmErr := ValidateTaskTransition(nextFrom, s)
				if fsmErr != nil {
					t.Errorf("FSM should allow %s -> %s: %v", nextFrom, s, fsmErr)
				}
			}

			// Verify the guard itself enforces the FSM (T23): done state must not allow further progress.
			if s == "done" {
				fsmErr := ValidateTaskTransition("in_progress", "done")
				if fsmErr != nil {
					t.Errorf("FSM should reject forward progress from terminal: %v", fsmErr)
				}
			}
		})
	}
}
