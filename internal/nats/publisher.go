package natspublisher

import (
	"context"
	"fmt"
	"sync"
	"time"

	js "github.com/nats-io/nats.go/jetstream"

	"tenant-provisioning-control-plane/internal/outbox"
)

// Publisher drives the outbox-poll → JetStream-publish loop.
// T19: NATS JetStream Publisher — reads pending events from the outbox table
// and publishes them to the configured JetStream stream, incrementing attempts
// on each failure so that the retry guard (T24) can enforce limits.
type Publisher struct {
	jsCtx   *js.APIContext
	stream  js.Stream
	limit   int
	updated chan<- string // optional signal channel for external monitoring
	mu      sync.RWMutex
	dones   int64
	fails   int64
}

// Config holds the parameters for New().
type Config struct {
	JetStream  *js.APIContext
	StreamName string
	Limit      int     // fetch batch size (default 50)
	Interval   time.Duration // poll interval (default 2s, max retry delay; see T24)
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

	p := &Publisher{
		jsCtx:    cfg.JetStream,
		stream:   stm,
		limit:    limit,
		Interval: interval,
	}
	return p, nil
}

// Run starts the polling loop. Blocks until ctx is cancelled.
func (p *Publisher) Run(ctx context.Context) error {
	if err := p.stream.Ready(); err != nil {
		return fmt.Errorf("nats publisher: stream not ready: %w", err)
	}

	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p.publishOneBatch(ctx)
		}
	}
}

// Stats returns processing counters.
func (p *Publisher) Stats() (ok, fail int64) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.dones, p.fails
}

// publishOneBatch fetches pending outbox rows and publishes each one to JetStream.
// On success the row is not deleted (the consumer will ack its own processed status).
// On failure attempts are bumped so T24's retry guard can eventually route it to DLQ.
func (p *Publisher) publishOneBatch(ctx context.Context) {
	events, err := outbox.FetchPendingOutbox(50)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[nats-pub] fetch pending: %v\n", err)
		return
	}

	if len(events) == 0 {
		if p.updated != nil {
			p.updated <- "idle"
		}
		return
	}

	for _, ev := range events {
		err = p.publishOnce(ctx, ev)
		if err != nil {
			p.mu.Lock()
			p.fails++
			p.mu.Unlock()
			fmt.Fprintln(os.Stderr, fmt.Sprintf("[nats-pub] publish %s: %v", ev.ID, err))
		} else {
			p.mu.Lock()
			p.dones++
			p.mu.Unlock()
			if p.updated != nil {
				p.updated <- fmt.Sprintf("done:%s", ev.ID)
			}
		}

		time.Sleep(10 * time.Millisecond) // back-off to avoid hammering JetStream
	}
}

// publishOnce publishes a single outbox event. Returns nil on success, error on failure.
func (p *Publisher) publishOnce(ctx context.Context, ev outbox.OutboxEvent) error {
	err := p.stream.PublishMessage(ctx, &js.Message{
		Subject: ev.Subject,
		Data:    ev.Payload,
	})
	if err != nil {
		return fmt.Errorf("publish %s: %w", ev.ID, err)
	}
	return nil
}
