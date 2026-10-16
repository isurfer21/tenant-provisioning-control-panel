// Package consumer subscribes to task progress events and applies them
// to the application's DB. Implements GAP 2, T21-T24.
package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	js "github.com/nats-io/nats.go/jetstream"

	lcipkg "tenant-provisioning-control-plane/internal/lifecycle"
	outbxpkg "tenant-provisioning-control-plane/internal/outbox"
	models "tenant-provisioning-control-plane/internal/model"
	storep "tenant-provisioning-control-plane/internal/store"
)

// inboundMsg is the JSON payload workers send on tasks.progress.<taskID>.
type inboundMsg struct {
	TenantID string `json:"tenant_id"`
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	EventID  string `json:"event_id"`
}

// Consumer subscribes to "tasks.progress.>" and drives the inbound half of the
// async provisioning loop (GAP 2 + T21-T24). Call Run(ctx) to start.
type Consumer struct {
	jsCtx        js.JetStream
	streamName   string
	consumerName string
	tenantRepo   *storep.TenantRepository
	taskRepo     *storep.TaskRepository
	db           *sql.DB
	orderGuard   *lcipkg.OrderGuard
	log          *log.Logger
}

// Config is passed to New. All fields are required except OrderGuard and Log.
type Config struct {
	JetStream  js.JetStream
	StreamName string
	Consumer   string
	TenantRepo *storep.TenantRepository
	TaskRepo   *storep.TaskRepository
	DB         *sql.DB
	OrderGuard *lcipkg.OrderGuard
	Log        *log.Logger
}

// New creates a Consumer bound to the given stream + durable consumer name.
func New(cfg Config) (*Consumer, error) {
	if cfg.JetStream == nil || cfg.StreamName == "" {
		return nil, fmt.Errorf("jetstream + stream_name required")
	}
	l := cfg.Log
	if l == nil {
		l = log.New(log.Writer(), "[consumer] ", log.LstdFlags)
	}
	og := cfg.OrderGuard
	if og == nil {
		og = lcipkg.NewOrderGuard()
	}
	return &Consumer{
		jsCtx:        cfg.JetStream,
		streamName:   cfg.StreamName,
		consumerName: cfg.Consumer,
		tenantRepo:   cfg.TenantRepo,
		taskRepo:     cfg.TaskRepo,
		db:           cfg.DB,
		orderGuard:   og,
		log:          l,
	}, nil
}

// Run subscribes to "tasks.progress.>" using JetStream Consume callback API and
// blocks until ctx is cancelled. Uses go-nats 1.34's Consumer.Consume(handler).
func (c *Consumer) Run(ctx context.Context) error {
	const subject = "tasks.progress.>"

	// Ensure stream exists first.
	_, createErr := c.jsCtx.CreateStream(ctx, js.StreamConfig{
		Name:     c.streamName,
		Subjects: []string{"tasks.create.>", "tasks.progress.>"},
	})
	if createErr != nil && !alreadyExists(createErr) {
		c.log.Printf("[consumer] warn: creating stream: %v", createErr)
	}

	// go-nats 1.34 uses *stream.CreateConsumer (NOT BindConsumer — removed).
	consumer, err := c.jsCtx.CreateConsumer(ctx, c.streamName, js.ConsumerConfig{
		Durable:       c.consumerName,
		DeliverPolicy: js.DeliverAllPolicy,
		AckPolicy:     js.AckExplicitPolicy,
		AckWait:       30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("create consumer %s on stream %s: %w", c.consumerName, c.streamName, err)
	}

	c.log.Printf("[consumer] subscribed to %s  stream=%s durable=%s", subject, c.streamName, c.consumerName)

	// Consume with callback — go-nats 1.x JetStream API for push-based delivery.
	consumer.Consume(func(msg js.Msg) {
		if pErr := c.processMessage(ctx, msg); pErr != nil {
			c.log.Printf("[consumer] processing error (nack): %v", pErr)
			msg.Nak() // let NATS redeliver.
		} else {
			msg.Ack()
		}
	})

	<-ctx.Done()
	return ctx.Err()
}

// alreadyExists checks if an error indicates "already exists" (HTTP 409 or text).
func alreadyExists(err error) bool {
	e := strings.ToLower(fmt.Sprintf("%v", err))
	return strings.Contains(e, "already") || strings.Contains(e, "409")
}

// processMessage is the full pipeline for inbound progress events:
// idempotent → order-guard → task-FSM → DB-update → tenant-terminal.
func (c *Consumer) processMessage(ctx context.Context, msg js.Msg) error {
	var m inboundMsg
	if err := json.Unmarshal(msg.Data(), &m); err != nil {
		return fmt.Errorf("unmarshal inbound: %w", err) // nack; transient → retry.
	}

	// 1 — idempotent guard (T22).
	evID := m.EventID
	if evID == "" {
		evID = m.TaskID
	}
	ok, dbErr := outbxpkg.IsProcessed(c.db, evID)
	if dbErr != nil || ok {
		c.log.Printf("[consumer] dup-or-fail ev=%s task=%s", evID, m.TaskID)
		return nil // skip.
	}

	// 2 — order-tolerant guard (T23).
	ts := time.Now().UTC()
	if err := c.orderGuard.Validate(m.TaskID, m.Status, ts, 0); err != nil {
		c.log.Printf("[consumer] out-of-order task=%s: %v", m.TaskID, err)
		return nil // best-effort; drop out-of-order.
	}

	// 3 — verify task exists and validate FSM transition.
	if c.taskRepo != nil {
		t, gErr := c.taskRepo.GetByID(ctx, m.TaskID)
		from := ""
		if t != nil {
			from = t.Status
		}
		if gErr != nil {
			return fmt.Errorf("get-task(%s): %w", m.TaskID, gErr) // retry.
		}
		if err := lcipkg.ValidateTaskTransition(from, m.Status); err != nil {
			c.log.Printf("[consumer] fsm-block task %s from=%s to=%s: %v", m.TaskID, from, m.Status, err)
			return nil // invalid → drop (transient won't help).
		}

		// 4 — update task status.
		if upErr := c.taskRepo.UpdateStatus(ctx, m.TaskID, m.Status); upErr != nil {
			c.log.Printf("[consumer] warn: task(%s) update: %v", m.TaskID, upErr)
			return nil // drop; optimistic lock means stale data for sure.
		}
	}

	c.log.Printf("[consumer] task %s -> %s tenant=%s ev=%s", m.TaskID, m.Status, m.TenantID, evID)

	// 5 — idempotent marker.
	if dbErr := outbxpkg.MarkProcessed(c.db, evID, "tasks.progress."+m.TaskID); dbErr != nil {
		c.log.Printf("[consumer] warn: mark-processed(ev=%s): %v", evID, dbErr)
	}

	// 6 — terminal outcome -> tenant FSM transition (GAP 2 / T14).
	if lcipkg.IsTerminal(m.Status) && m.TenantID != "" {
		c.doTenantFsm(ctx, m)
	}
	return nil
}

// doTenantFsm updates tenant state on terminal task outcomes.
func (c *Consumer) doTenantFsm(ctx context.Context, m inboundMsg) {
	t, err := c.tenantRepo.GetByID(ctx, m.TenantID)
	if err != nil {
		c.log.Printf("[consumer] warn: tenant %s not found", m.TenantID)
		return
	}

	var newS string
	switch m.Status {
	case models.Completed:
		newS = "active" // provisioning -> active.
	case models.Failed:
		newS = "failed" // provisioning -> failed (retry).
	default:
		return
	}

	if err := lcipkg.ValidateTenantTransition(t.Status, newS); err != nil {
		c.log.Printf("[consumer] warn: tenant FSM block %s->%s", t.Status, newS)
		return
	}
	if upErr := c.tenantRepo.Update(ctx, t.ID, t.Name, newS, t.Version); upErr != nil {
		c.log.Printf("[consumer] warn: tenant(%s) update: %v", m.TenantID, upErr)
		return
	}
	c.log.Printf("[consumer] terminal tenant %s -> %s via task[%s]=%s done", m.TenantID, newS, m.TaskID, m.Status)
}

const (
	TaskDone   = models.Completed
	TaskFailed = "failed"
	Destroyed  = "destroyed"
)
