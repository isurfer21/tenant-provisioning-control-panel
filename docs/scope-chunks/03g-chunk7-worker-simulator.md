# Chunk 7 — Worker Simulator CLI

**Goal**: Implement `cmd/server/sim/main.go` (or under cmd/worker-sim/) that connects to NATS, subscribes to task_outbound events, simulates provisioning work, and publishes progress back. It should accept configurable failure and delay params at startup. No HTTP endpoint needed — pure CLI tool consuming from the broker.

---

## Prompt: Build the worker simulator

> Create a Go CLI application at `cmd/worker-simulator/main.go` (a separate main package) that connects to a NATS JetStream instance, listens for task_outbound events, simulates provisioning work with configurable speed and failure rate, then publishes progress updates back on the broker's progress stream.

### CLI flags
Use `flag` package or `github.com/spf13/pflag` for these parameters:
```bash
--fail-rate float       Probability of task failing (0.0 = always success; 1.0 = always fails) (default 0)
--min-delay-ms int      Minimum simulated work time in milliseconds (default 500)
--max-delay-ms int      Maximum simulated work time in milliseconds (default 3000)
--broker-url string     NATS server URL (default "nats://localhost:4222")
```

### Core subscription logic
1. Connect to NATS via `nats.Connect(flags.BrokerURL)` with reconnection enabled (reconnect attempts → unlimited, wait on reconnect 50ms).
2. Get JetStream context: `js, _ := nc.JetStream()`.
3. Subscribe to the consumer stream's filter subject ("tasks.create.>"):
   ```go
   ch := make(chan *nats.Msg, 10) // bounded channel
   jc, err := js.SubscribeQueue("progress_updates", "...", func(msg *nats.Msg){ ch <- msg }, ... )
   if err != nil { log.Fatalf(...) }
   defer jc.Drain()
   ```

### Message handling loop (run in worker goroutines from a pool of 3 concurrent workers)
For each incoming message received on channel:
1. Parse the JSON data to extract task_id, task_type, tenant_id fields (use json.Unmarshal into a ProgressEnvelope struct).
2. Extract `event_id` from NATS headers or envelope to use as our local idempotency tracker. Skip if already processed (keep last-processed set with at most 100 entries).
3. Calculate simulated delay: `delay = min_delay + rand.Intn(max_delay - min_delay)` milliseconds using a proper random seed per process (`rand.New(rand.NewSource(time.Now().UnixNano()))`).
4. Apply failure check: if `rand.Float64() < fail_rate` → status is "failed" else "done". Use deterministic seeded randomness if possible so tests are reproducible. Log the simulated state transition: `log.Printf("Processing task %s [%s] for tenant %s — simulating work (%dms) then marking as %v", taskID, type_, tenantID, delay, status)`.
5. Wait for the calculated delay using `time.Sleep(time.Duration(delay) * time.Millisecond)`.
6. After delay publish progress back to NATS using JetStream: construct the Progress envelope JSON with EventID (generate a new UUID per delivery as our idempotency key), TaskId (the stable task.id from input), Status ("done" or "failed"), UpdatedAt = now UTC in milliseconds RFC 3339 format. Publish to subject `tasks.progress.<task_id>`.
7. Acknowledge the original message via msg.Ack().

### Graceful shutdown
- Trap SIGINT and SIGTERM signals using signal.Notify. On signal: drain pending messages from processing queue (or just stop accepting new subscriptions and let in-flight tasks complete). Call jc.Drain() then nc.Close() then log "Worker simulator shutting down...".

After completion the file must also have a `main()` function that ties everything together: parse flags, initialize logger with timestamp+level format, start the subscription loop (blocking until signal), print a startup summary (broker url / fail-rate values).

Add a simple test (`cmd/worker-simulator/main_test.go` if in own package or inline) showing the worker correctly parses CLI args and generates random delays within configured boundaries. Use `t.Errorf()` to assert that calculated delay falls between min_delay and max_delay bounds across iterations. This validates the simulator itself works as expected before integration testing.
