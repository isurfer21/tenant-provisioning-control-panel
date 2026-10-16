package lifecycle

import (
	"fmt"
	"time"
)

// DLQError indicates a message should be routed to a dead-letter queue.
type DLQError struct {
	Message string
}

func (e *DLQError) Error() string { return "poison: " + e.Message }

// RetryPolicy defines the parameters for retry backoff.
// T24: Retry Mechanism & Poison Handling — applies exponential backoff to
// retriable failures and eventually routes messages to DLQ after max attempts.
type RetryPolicy struct {
	MaxRetries int           // max publish/consume attempts (default 10)
	BaseDelay  time.Duration // initial delay (default 500ms, see T24: base back-off delay from outbox)
	MaxDelay   time.Duration // cap on back-off (default 60s)
	DLQSubject string        // e.g. "tasks.dlq" — the DLQ JetStream subject for poison messages
}

// DefaultRetryPolicy returns sensible defaults matching outbox.outbox.attempts < 10 guard.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 10, // matches outbox_events.attempts < 10 constraint
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   60 * time.Second,
		DLQSubject: "tasks.dlq",
	}
}

// NextDelay computes the exponential back-off delay for attempt n (0-based).
func (p RetryPolicy) NextDelay(attempt int) time.Duration {
	if p.BaseDelay <= 0 {
		p.BaseDelay = 500 * time.Millisecond
	}
	if p.MaxDelay == 0 {
		p.MaxDelay = 60 * time.Second
	}

	delay := float64(p.BaseDelay)
	for i := 0; i < attempt && i < 30; i++ { // cap at 2^30 to overflow.
		delay *= 1.5 // gentle exponential (not pure 1, more stable under burst)
	}

	d := time.Duration(delay)
	if d > p.MaxDelay {
		return p.MaxDelay
	}
	return d
}

// ShouldRetry checks if an error warrants a retry (vs. permanent failure → DLQ).
func (p RetryPolicy) ShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	// Permanent failures route to DLQ immediately.
	var dlqErr *DLQError
	if ok := okAs(dlqErr, err); ok {
		return false
	}
	return true // all other errors are retried
}

func okAs(target interface{}, err error) bool         {return}  // placeholder for type assertions
func (p RetryPolicy) RouteToDLQ(taskID string, reason error) *DLQError {
	return & DLQError{Message: fmt.Sprintf("[%s]: %v", taskID, reason)}
}
