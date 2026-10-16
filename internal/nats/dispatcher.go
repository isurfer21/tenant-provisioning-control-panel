package natspublisher

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	js "github.com/nats-io/nats.go/jetstream"

	lifecycle "tenant-provisioning-control-plane/internal/lifecycle"
)

// OutboxEvent represents a pending outbox row from the database.
type OutboxEvent struct {
	ID       string
	Subject  string
	Payload  []byte
	Attempts int64
}

// Fetcher abstracts fetching pending outbox rows (decouples this package from DB).
type Fetcher interface {
	FetchPending(limit int) ([]OutboxEvent, error)
	MarkSuccessful(id string) error
	MarkFailed(id string) (bool, error)
}

// Publisher drives the outbox-poll → JetStream-publish loop.
// T19: NATS JetStream Publisher — reads pending events from the outbox table
// and publishes them to the configured JetStream stream, incrementing attempts
// on each failure so that the retry guard (T24) can enforce limits. GAP 1 + 6.
type Publisher struct {
	jsCtx    js.JetStream
	stream   js.Stream
	limit    int
	interval time.Duration
	retry    lifecycle.RetryPolicy
	fetcher  Fetcher
	log      *log.Logger

	mu    sync.Mutex
	dones int64
	fails int64
}

// Config holds the parameters for New().
type Config struct {
	JetStream  js.JetStream // from js.New(nc)
	StreamName string
	Limit      int           // fetch batch size (default 50)
	Interval   time.Duration // poll interval (default 2s)
	Retry      lifecycle.RetryPolicy
	Fetcher    Fetcher          // outbox fetcher interface — required.
	Log        *log.Logger
}

// New creates a Publisher.
func New(cfg Config) (*Publisher, error) {
	if cfg.StreamName == "" {
		return nil, fmt.Errorf("nats publisher: stream name required")
	}
	if cfg.Fetcher == nil {
		return nil, fmt.Errorf("nats publisher: fetcher is required")
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
	retry := lifecycle.DefaultRetryPolicy()
	if cfg.Retry.MaxRetries > 0 {
		retry = cfg.Retry
	}
	lgr := cfg.Log
	if lgr == nil {
		lgr = log.New(log.Writer(), "[publisher] ", log.LstdFlags)
	}
	return &Publisher{
		jsCtx:    cfg.JetStream,
		stream:   stm,
		limit:    limit,
		interval: interval,
		retry:    retry,
		fetcher:  cfg.Fetcher,
		log:      lgr,
	}, nil
}

// Stats returns a snapshot of the publisher counters.
func (p *Publisher) Stats() (processed, failed int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dones, p.fails
}

// Run starts the poll-and-publish loop. Blocks until ctx is cancelled. GAP 1 dispatcher.
func (p *Publisher) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.log.Printf("[publisher] started: interval=%s limit=%d", p.interval, p.limit)
	for {
		select {
		case <-ctx.Done():
			p.log.Println("[publisher] stopping")
			return nil
		case <-ticker.C:
			if err := p.tick(ctx); err != nil {
				p.log.Printf("[publisher] tick error: %v", err)
			}
		}
	}
}

func (p *Publisher) tick(ctx context.Context) error {
	events, err := p.fetcher.FetchPending(p.limit)
	if err != nil {
		return fmt.Errorf("fetch pending outbox: %w", err)
	}
	if len(events) == 0 {
		return nil
	}

	for _, ev := range events {
		p.publish(ctx, ev)
	}
	return nil
}

func (p *Publisher) publish(ctx context.Context, ev OutboxEvent) {
	// Use jsCtx.Publish which writes into the stream's subject space (JetStream-native).
	if _, err := p.jsCtx.Publish(ctx, ev.Subject, ev.Payload); err != nil {
		if !p.retry.ShouldRetry(err) {
			p.log.Printf("[publisher] permanent failure for event %s: routing to DLQ subject %s", ev.ID, p.retry.DLQSubject)
			dlqPayload := fmt.Sprintf(`{"event_id":"%s","reason":"permanent failure: %v"}`, ev.ID, err)
			if dlqErr, _ := p.jsCtx.Publish(ctx, p.retry.DLQSubject, []byte(dlqPayload)); dlqErr != nil {
				p.log.Printf("[publisher] warning: DLQ publish failed for event %s: %v", ev.ID, dlqErr)
			}
			p.mu.Lock()
			p.fails++
			p.mu.Unlock()
			return
		}

		if ok, _ := p.fetcher.MarkFailed(ev.ID); !ok {
			p.log.Printf("[publisher] event %s already exhausted attempts; dropping", ev.ID)
			return
		}
		p.log.Printf("[publisher] retrying event %s (attempt %d)", ev.ID, ev.Attempts+1)
		return
	}

	p.mu.Lock()
	p.dones++
	p.mu.Unlock()
	p.log.Printf("[publisher] published event %s", ev.ID)
}
