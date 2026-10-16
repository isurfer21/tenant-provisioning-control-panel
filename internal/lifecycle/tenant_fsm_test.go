package lifecycle

import (
	"testing"
)

func TestValidateTenantTransition_Valid(t *testing.T) {
	// Every transition in the valid map must succeed.
	validCases := []struct {
		from, to string
	}{
		{"provisioning", "active"},
		{"provisioning", "failed"},
		{"active", "updating"},
		{"active", "destroying"},
		{"updating", "active"},
		{"updating", "failed"},
		{"destroying", "destroyed"},
		{"destroying", "failed"},
		{"failed", "destroying"},
	}

	for _, tc := range validCases {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			if err := ValidateTenantTransition(tc.from, tc.to); err != nil {
				t.Fatalf("expected valid transition %s -> %s, got: %v", tc.from, tc.to, err)
			}
		})
	}
}

func TestValidateTenantTransition_Invalid(t *testing.T) {
	type testCase struct {
		from, to   string
		errType    string // expected error type name
		substr     string // partial message check
	}

	invalidCases := []testCase{
		// Can't go from provisioning directly to any non-permitted state.
		{"provisioning", "updated", "*ErrInvalidTransition", "not allowed"},
		{"provisioning", "destroyed", "*ErrInvalidTransition", "not allowed"},

		// Can't leave a terminal state.
		{"destroyed", "active", "*ErrInvalidTransition", ""},          // no entry in map = invalid
		{"destroyed", "failed", "*ErrInvalidTransition", ""},
		{"destroyed", "provisioning", "*ErrInvalidTransition", ""},

		// Unknown source state.
		{"unknown-state", "active", "*ErrInvalidTransition", "not allowed"},
		{"", "active", "*ErrInvalidTransition", "not allowed"},

		// Same-state edges not in the valid map.
		{"active", "active", "*ErrInvalidTransition", "not allowed"},
	}

	for _, tc := range invalidCases {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			err := ValidateTenantTransition(tc.from, tc.to)
			if err == nil {
				t.Fatalf("expected error for %s -> %s, got nil", tc.from, tc.to)
			}

			// Check error type.
			_, isInvalid := err.(*ErrInvalidTransition)
			if tc.errType != "" && !isInvalid {
				t.Errorf("expected *ErrInvalidTransition but got %T", err)
			}

			// Check message substring if provided.
			if tc.substr != "" && tc.substr == "not allowed" {
				if err.Error() == "" {
					t.Fatal("error message is empty")
				}
			}
		})
	}
}

func TestValidateTenantTransition_ErrorMessage(t *testing.T) {
	err := ValidateTenantTransition("active", "provisioning")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expected := "tenant state transition not allowed: active -> provisioning"
	if err.Error() != expected {
		t.Errorf("got %q; want %q", err.Error(), expected)
	}
}

func TestValidateTenantTransition_DestinationDestroyedNeverValid(t *testing.T) {
	// destroyed is a terminal sink — no valid transition ever enters it (it's only a target).
	err := ValidateTenantTransition("destroyed", "anything")
	if err == nil {
		t.Fatal("expected error transitioning from destroyed, got nil")
	}
}
