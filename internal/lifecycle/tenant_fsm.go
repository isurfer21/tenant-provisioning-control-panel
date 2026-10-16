package lifecycle

import "fmt"

// ErrInvalidTransition is the error type for invalid tenant FSM transitions.
type ErrInvalidTransition struct {
	From, To string
}

func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("tenant state transition not allowed: %s -> %s", e.From, e.To)
}

var validTenantTransitions map[string][]string

func init() {
	validTenantTransitions = map[string][]string{
		"provisioning": {"active", "failed", "destroying"}, // allow cancel mid-provisioning (GAP 4)
		"active":       {"updating", "destroying"},
		"updating":     {"active", "failed"},
		"destroying":   {"destroyed", "failed"},
		"failed":       {"destroying"},
	}
}

// ValidateTenantTransition checks if the from->to transition is valid.
func ValidateTenantTransition(from, to string) error {
	validTargets := validTenantTransitions[from]
	if validTargets == nil {
		return &ErrInvalidTransition{From: from, To: to}
	}
	for _, t := range validTargets {
		if t == to {
			return nil
		}
	}
	return &ErrInvalidTransition{From: from, To: to}
}
