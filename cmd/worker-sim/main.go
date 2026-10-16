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

	nats "github.com/nats-io/nats.go"
	js "github.com/nats-io/nats.go/jetstream"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: worker-sim --fail-rate=<0-1> --max-delay=<duration>\n")
		os.Exit(1)
	}

	var failRate float64
	maxDelay := 5 * time.Second
	for _, arg := range os.Args[1:] {
		switch {
		case strings.HasPrefix(arg, "--fail-rate="):
			fmt.Sscanf(arg, "--fail-rate=%f", &failRate)
		case strings.HasPrefix(arg, "--max-delay="):
			d, err := time.ParseDuration(strings.TrimPrefix(arg, "--max-delay="))
			if err == nil { maxDelay = d }
		}
	}

	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil { fmt.Fprintf(os.Stderr, "nats connect: %v\n", err); os.Exit(1) }
	defer nc.Drain()

	jsCtx, err := js.New(nc)
	if err != nil { fmt.Fprintf(os.Stderr, "jetstream new: %v\n", err); os.Exit(1) }

	// Create stream (Idempotent — safe to call repeatedly).
	streamName := "TASKS_CREATE"
	_, _ = jsCtx.CreateStream(context.Background(), js.StreamConfig{
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
	consumer, err := jsCtx.CreateConsumer(context.Background(), streamName, consumerCfg)
	if err != nil { fmt.Fprintf(os.Stderr, "consumer: %v\n", err); os.Exit(1) }

	handler := func(msg js.Msg) { handleInBound(context.Background(), cfg{failRate, maxDelay}, msg) }
	conCtx, err := consumer.Consume(handler)
	if err != nil  { fmt.Fprintf(os.Stderr, "consume: %v\n", err); os.Exit(1) }
	defer conCtx.Stop()

	fmt.Fprintln(os.Stderr, "[worker-sim] subscribed to tasks.create.>")

	// Graceful shutdown on SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Fprintln(os.Stderr, "[worker-sim] shutting down...")
	conCtx.Stop()
}

type cfg struct{ failRate float64; maxDelay time.Duration }

func handleInBound(ctx context.Context, c cfg, msg js.Msg) {
	var p struct {
		TenantID string `json:"tenant_id"`
		TaskID   string `json:"task_id"`
		Type     string `json:"type"`
	}
	if err := json.Unmarshal(msg.Data(), &p); err != nil {
		fmt.Fprintf(os.Stderr, "[worker-sim] parse fail: %v\n", err)
		msg.Nak()
		return
	}

	time.Sleep(time.Duration(rand.Float64()*float64(c.maxDelay)))

	if rand.Float64() < c.failRate {
		msg.Nak()
		fmt.Fprintf(os.Stderr, "[worker-sim] FAIL task=%s\n", p.TaskID)
		return
	}

	status := "in_progress"
	if rand.IntN(2) == 0 { status = "done" /* randomly return done progress event for testing */ }

	_ = status // emitted as 'tasks.progress.<task_id>' in production
	msg.Ack()
}
