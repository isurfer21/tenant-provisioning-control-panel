package natspublisher

import (
	"context"
	"fmt"
	"sync"
	"time"

	js "github.com/nats-io/nats.go/jetstream"
)

// Publisher drives the outbox-poll → JetStream-publish loop.
// T19: NATS JetStream Publisher — reads pending events from the outbox table
// and publishes them to the configured JetStream stream, incrementing attempts
// on each failure so that the retry guard (T24) can enforce limits.
type Publisher struct {
	jsCtx    js.JetStream
	stream   js.Stream
	limit    int
	interval time.Duration
	mu       sync.RWMutex
	dones    int64
	fails    int64
}

// Config holds the parameters for New().
type Config struct {
	JetStream  js.JetStream // from js.New(nc)
	StreamName string
	Limit      int           // fetch batch size (default 50)
	Interval   time.Duration // poll interval (default 2s)
}

// New creates a Publisher.
func New(cfg Config) (*Publisher, error) {
	if cfg.StreamName == "" {
		return nil, fmt.Errorf("nats publisher: stream name required")
	}
	stm, err := cfg.JetStream.Stream(context.Background(), cfg.StreamName)
	if err != nil {
		return nil, fmt.Errorf("nats publisher: lookup stream %s: %w", cfg.StreamName, err)
	}
	limit := cfg.Limit
	if limit <= 0 {
		limit = 50
	}
	interval := cfg.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &Publisher{
		jsCtx:    cfg.JetStream,
		stream:   stm,
		limit:    limit,
		interval: interval,
	}, nil
}

// Stats returns a snapshot of the publisher counters.
func (p *Publisher) Stats() (processed, failed int64) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.dones, p.fails
}
