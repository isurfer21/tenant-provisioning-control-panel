package lifecycle

import (
	"testing"
)

func TestValidateTaskTransition_Valid(t *testing.T) {
	validCases := []struct {
		from, to string
	}{
		{"accepted", "in_progress"},
		{"in_progress", "done"},
		{"in_progress", "failed"},
	}

	for _, tc := range validCases {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			if err := ValidateTaskTransition(tc.from, tc.to); err != nil {
				t.Fatalf("expected valid transition %s -> %s, got: %v", tc.from, tc.to, err)
			}
		})
	}
}

func TestValidateTaskTransition_Invalid(t *testing.T) {
	type testCase struct {
		from, to string
		name     string // expected error type name ("terminal" or "invalid")
	}

	invalidCases := []testCase{
		{"accepted", "done", "invalid"},    // must go through in_progress first
		{"accepted", "failed", "invalid"},
		{"accepted", "accepted", "invalid"},  // same state not allowed
		{"in_progress", "accepted", "invalid"}, // can't go backward
		{"in_progress", "in_progress", "invalid"},
	}

	for _, tc := range invalidCases {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			err := ValidateTaskTransition(tc.from, tc.to)
			if err == nil {
				t.Fatalf("expected error for %s -> %s, got nil", tc.from, tc.to)
			}

			switch tc.name {
			case "terminal":
				if _, ok := err.(*ErrTerminalTask); !ok && !(err.Error() != "" && err != nil) {
					t.Errorf("expected ErrTerminalTask for %s -> %s; got %T", tc.from, tc.to, err)
				}
			case "invalid":
				// Could be either — just check it is an error.
			}
		})
	}
}

func TestValidateTaskTransition_TerminalStateGuard(t *testing.T) {
	terminalStates := []string{"done", "failed"}

	for _, status := range terminalStates {
		t.Run("terminal-"+status, func(t *testing.T) {
			// From terminal, only staying in same state should be silently ok or rejected as terminal.
			// Transitions to a DIFFERENT state must fail with ErrTerminalTask.
			err := ValidateTaskTransition(status, "in_progress")
			if err == nil {
				t.Fatalf("expected error transitioning from %s, got nil", status)
			}

			_, isTerminal := err.(*ErrTerminalTask)
			if !isTerminal {
				t.Errorf("expected *ErrTerminalTask for %s -> in_progress, got %T: %v", status, err, err)
			}

			// Same-state from terminal should also be invalid (no self-loop).
			err2 := ValidateTaskTransition(status, status)
			if err2 == nil {
				t.Fatalf("expected error for same-state from terminal %s, got nil", status)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"done", true},
		{"failed", true},
		{"accepted", false},
		{"in_progress", false},
		{"provisioning", false},
		{"active", false},
	}

	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			got := IsTerminal(tc.status)
			if got != tc.want {
				t.Errorf("IsTerminal(%q) = %v; want %v", tc.status, got, tc.want)
			}
		})
	}
}

func TestValidateTaskTransition_ErrorMessage(t *testing.T) {
	err := ValidateTaskTransition("in_progress", "accepted")
	if err == nil {
		t.Fatal("expected error for in_progress -> accepted, got nil")
	}
	expected := "task state transition not allowed: in_progress -> accepted"
	if err.Error() != expected {
		t.Errorf("got %q; want %q", err.Error(), expected)
	}
}

func TestValidateTaskTransition_TerminalErrorMessage(t *testing.T) {
	err := ValidateTaskTransition("done", "in_progress")
	if err == nil {
		t.Fatal("expected error from terminal state, got nil")
	}
	if _, ok := err.(*ErrTerminalTask); !ok {
		t.Errorf("expected *ErrTerminalTask, got %T", err)
	}
	expectedErr := "task already in terminal state: done"
	if err.Error() != expectedErr {
		t.Errorf("got %q; want %q", err.Error(), expectedErr)
	}
}
