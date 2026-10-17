package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	nats "github.com/nats-io/nats.go"
	js "github.com/nats-io/nats.go/jetstream"
)

var (
	nc      *nats.Conn
	jsWpub  js.JetStream // JetStream context for writing progress.
)

func main() {
	var failRate float64
	maxDelay := 5 * time.Second

	// Read defaults from env vars (set by docker-compose.yml) if present.
	hasEnv := false
	if v := os.Getenv("FAIL_RATE"); v != "" {
		fmt.Sscanf(v, "%f", &failRate); hasEnv = true
	}
	if v := os.Getenv("MAX_DELAY_MS"); v != "" {
		d, err := time.ParseDuration(v + "ms")
		if err == nil { maxDelay = d }; hasEnv = true
	}

	for _, arg := range os.Args[1:] {
		switch {
		case strings.HasPrefix(arg, "--fail-rate="):
			fmt.Sscanf(arg, "--fail-rate=%f", &failRate)
		case strings.HasPrefix(arg, "--max-delay="):
			d, err := time.ParseDuration(strings.TrimPrefix(arg, "--max-delay="))
			if err == nil {
				maxDelay = d
			}
		}
	}

	// Require either env vars or CLI args; show usage if neither.
	if !hasEnv && len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: worker-sim --fail-rate=<0-1> --max-delay=<duration>\n")
		os.Exit(1)
	}

	var err error
	natsURL := ""
	if v := os.Getenv("NATS_URL"); v != "" { natsURL = v }
	nc, err = nats.Connect(natsURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nats connect: %v\n", err)
		os.Exit(1)
	}
	defer nc.Drain()

	jsWpub, err = js.New(nc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jetstream new: %v\n", err)
		os.Exit(1)
	}

	streamName := "TASKS_CREATE"
	_, _ = jsWpub.CreateStream(context.Background(), js.StreamConfig{
		Name:     streamName,
		Subjects: []string{"tasks.create.>", "tasks.progress.>"},
	})

	consumerName := "worker-sim"
	consumerCfg := js.ConsumerConfig{
		Durable:       consumerName,
		DeliverPolicy: js.DeliverAllPolicy,
		AckPolicy:     js.AckExplicitPolicy,
		AckWait:       30 * time.Second,
	}
	consumer, err := jsWpub.CreateConsumer(context.Background(), streamName, consumerCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "consumer: %v\n", err)
		os.Exit(1)
	}

	cfg := cfg{failRate: failRate, maxDelay: maxDelay}
	handler := func(msg js.Msg) { handleInBound(context.Background(), cfg, msg) }
	conCtx, err := consumer.Consume(handler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "consume: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "[worker-sim] subscribed to tasks.create.>")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Fprintln(os.Stderr, "[worker-sim] shutting down...")
	conCtx.Stop()
}

type cfg struct {
	failRate float64
	maxDelay time.Duration
}

func handleInBound(ctx context.Context, c cfg, msg js.Msg) {
	var p struct {
		TenantID string `json:"tenant_id"`
		TaskID   string `json:"task_id"`
		Type     string `json:"type"`
		EventID  string `json:"event_id"`
	}
	if err := json.Unmarshal(msg.Data(), &p); err != nil {
		fmt.Fprintf(os.Stderr, "[worker-sim] parse fail: %v\n", err)
		msg.Nak()
		return
	}

	time.Sleep(time.Duration(rand.Float64() * float64(c.maxDelay)))

	// Each delivery gets a unique event_id so the consumer doesn't reject it as
	// "already processed" on subsequent progress updates (dedup is per-event).
	deliveryEvt := uuid.New().String()

	if rand.Float64() < c.failRate {
		msg.Nak()
		fmt.Fprintf(os.Stderr, "[worker-sim] FAIL task=%s\n", p.TaskID)
		publishProgress(ctx, "failed", p.TenantID, p.TaskID, deliveryEvt)
		return
	}

	status := "done" // default: simulate successful provisioning complete
	if rand.IntN(4) != 0 {
		status = "in_progress" // sometimes stall in intermediate state
	}

	fmt.Fprintf(os.Stderr, "[worker-sim] %s task=%s\n", status, p.TaskID)

	// GAP 3 FIX: publish progress updates back to NATS.
	publishProgress(ctx, status, p.TenantID, p.TaskID, deliveryEvt)

	msg.Ack()
}

func publishProgress(ctx context.Context, status, tenantID, taskID, eventID string) {
	// Use nc.Publish (no stream sequence like js.Publish).
	progress := struct {
		TenantID string `json:"tenant_id"`
		TaskID   string `json:"task_id"`
		Status   string `json:"status"`
		EventID  string `json:"event_id"`
	}{
		TenantID: tenantID,
		TaskID:   taskID,
		Status:   status,
		EventID:  eventID,
	}
	payload, _ := json.Marshal(progress)
	if err := nc.Publish("tasks.progress."+taskID, payload); err != nil {
		fmt.Fprintf(os.Stderr, "[worker-sim] publish err: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[worker-sim] → published tasks.progress.%s [%s]\n", taskID, status)
}
