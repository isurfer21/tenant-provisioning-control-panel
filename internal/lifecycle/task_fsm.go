package lifecycle

import "fmt"

// taskTransitions maps a source status to allowed target statuses for tasks.
var taskTransitions map[string][]string

func init() {
	taskTransitions = map[string][]string{
		"accepted":    {"in_progress"},
		"in_progress": {"done", "failed"},
	}
}

// ErrTerminalTask is the error type when a state transition targets a terminal status.
type ErrTerminalTask struct {
	Status string
}

func (e *ErrTerminalTask) Error() string {
	return fmt.Sprintf("task already in terminal state: %s", e.Status)
}

// ErrInvalidTaskTransition is the error type for disallowed task transitions.
type ErrInvalidTaskTransition struct {
	From, To string
}

func (e *ErrInvalidTaskTransition) Error() string {
	return fmt.Sprintf("task state transition not allowed: %s -> %s", e.From, e.To)
}

// IsTerminal returns if the status is a terminal state.
func IsTerminal(s string) bool {
	return s == "done" || s == "failed"
}

// ValidateTaskTransition checks that from→to is valid for task FSM.
func ValidateTaskTransition(from, to string) error {
	if IsTerminal(from) && from != to {
		return &ErrTerminalTask{Status: from}
	}
	for _, t := range taskTransitions[from] {
		if t == to {
			return nil
		}
	}
	return &ErrInvalidTaskTransition{From: from, To: to}
}
