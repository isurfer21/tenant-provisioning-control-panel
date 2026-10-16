package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	nats "github.com/nats-io/nats.go"
	js "github.com/nats-io/nats.go/jetstream"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: worker-sim --fail-rate=<0-1> --max-delay=<duration>\nExamples:\n  worker-sim --fail-rate=0.2 --max-delay=2s\n")
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
			if err == nil {
				maxDelay = d
			}
		}
	}

	fmt.Fprintf(os.Stderr, "[worker-sim] connecting to nats://localhost:4222\n")
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nats connect: %v\n", err)
		os.Exit(1)
	}
	defer nc.Drain()

	jsCtx, err := js.New(nc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jetstream new: %v\n", err)
		os.Exit(1)
	}

	// Create stream if it doesn't exist.
	_, _ = jsCtx.CreateStream(context.Background(), js.StreamConfig{
		Name:     "TASKS_CREATE",
		Subjects: []string{"tasks.create.>"},
	})
	fmt.Fprintln(os.Stderr, "[worker-sim] stream TASKS_CREATE ready")

	stm, err := jsCtx.Stream(context.Background(), "TASKS_CREATE")
	if err != nil {
		fmt.Fprintf(os.Stderr, "stream lookup: %v\n", err)
		os.Exit(1)
	}

	_, _ = stm.CreateOrUpdateConsumer(context.Background(), js.ConsumerConfig{
		Durable:       "worker-sim",
		DeliverPolicy: js.DeliverAllPolicy,
		AckPolicy:     js.AckExplicitPolicy,
		AckWait:       30 * time.Second,
	})

	con, err := stm.Consume(messagesHandler(jsCtx, failRate, maxDelay))
	if err != nil {
		fmt.Fprintf(os.Stderr, "consume: %v\n", err)
		os.Exit(1)
	}
	defer con.Stop()

	fmt.Fprintln(os.Stderr, "[worker-sim] subscribed to tasks.create.>")

	// Wait for interrupt signal for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Fprintln(os.Stderr, "[worker-sim] shutting down...")
}

func messagesHandler(jsCtx *js.APIContext, failRate float64, maxDelay time.Duration) js.OptionFunc {
	var total, acked, nakd atomic.Int64
	total.Add(1) // initial count so we print at least once in drain

	return js.ProcessMessages(func(msg js.Msg) error {
		var p struct {
			TenantID string `json:"tenant_id"`
			TaskID   string `json:"task_id"`
			Type     string `json:"type"`
		}
		if err := json.Unmarshal(msg.Data(), &p); err != nil {
			fmt.Fprintf(os.Stderr, "[worker-sim] parse fail: %v msg=%s\n", err, p.TaskID)
			return msg.Nack() // immediately nack malformed payloads
		}

		total.Add(1)

		// Simulate processing latency.
		time.Sleep(time.Duration(rand.Float64()*float64(maxDelay)))

		// Simulate random failures for testing retry/DLQ behavior.
		if rand.Float64() < failRate {
			nakd.Add(1)
			fmt.Fprintf(os.Stderr, "[worker-sim] FAIL task=%s (fail-rate=%0.0f%%)\n", p.TaskID, failRate*100)
			return msg.Nack() // triggers retry with backoff
		}

		status := "in_progress"
		if rand.IntN(2) == 0 {
			status = "done"
		}

		data, _ := json.Marshal(map[string]string{
			"task_id": p.TaskID,
			"status":  status,
		})
		_ = jsCtx.Publish(context.Background(), "tasks.progress."+p.TaskID, data)

		acked.Add(1)
		return msg.Ack()
	})
}
