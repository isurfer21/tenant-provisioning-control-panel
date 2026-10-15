# Chunk 6 — Message Broker Abstraction & NATS JetStream Integration

**Goal**: Implement `internal/broker/` with a clean interface abstraction and the concrete NATS JetStream implementation for both outbound task publishing and inbound progress consumption.

---

## Prompt: Build the message broker layer

> Create two Go source files under `internal/broker/broker.go` and `internal/broker/nats_impl.go`.

### 1. Broker abstraction (broker.go)
Define an interface that the Manager only knows about:
```go
type Broker interface {
    // StartConsuming(ctx context.Context, handler TaskProgressHandler) error
    // ProduceTask(ctx context.Context, task *model.Task) error }
type TaskProgressHander func(ctx context.Context, event ProgressEvent) error

type ProgressEvent struct {    
    EventID string `json:"event_id"`    // Unique per-delivery idempotency key
    TaskID  string `json:"task_id"`        // Stable task reference for correlation
    Status  string `json:"status"        "in_progress|done|failed"}`
    UpdatedAt string          ```go type ProgressEvent struct {
    EventID   string `json:"event_id"`     // Unique per-delivery idempotency key
    TaskID    string `json:"task_id"`      // Stable reference to the task being updated.}  ```

> Add the following methods: StartConsuming() (async), ProduceTask(), and Close(). The producer must publish AFTER a DB Tx commits, never before. Use a dedicated NATS connection for publish with a separate connection for consumption. Ensure no interleaved writes or stale subscriptions exist between connections.

### 2. NATS JetStream implementation (nats_impl.go)
Use the package `github.com/nats-io/nats.go` and specifically use **JetStream** features (not legacy pub/sub):
- Connect with `nats.Connect(natsURL)` using options for reconnect (backoff, timeout).
- On startup initialize a JetStream context: `js, err := nc.JetStream()`.
- Create two streams if they don't already exist (use `JetStream.CreateStream()` with appropriate configurations):
  - Stream name "TASKS" (for both inbound/outbound — subjects differentiate):
    ```go
    { Name: "TASKS", Subjects: []string{"tasks.create.>", "tasks.progress.>"}, MaxAge: time.Hour * 24 }
    ```
  - Two durable consumers for the same stream on different filters: one on `tasks.create.*` for producers, one on `tasks.progress.*` filtered to only incoming updates from workers (use FilterSubject in consumer config).
- Producer must publish with **confirmation**: `js.PublishMsg(m)` where Msg has headers containing metadata like X-Nats-Deliveried-Count=1. This ensures we detect poison messages later if needed.

### 3. Consumer implementation: the key requirement here is idempotency and order tolerance
The consumer callback (`handler`) receives each message. Implement this logic inside the wrapper in nats_consumer.go before handing off to the caller (lifecycle manager):

- **Idempotency check**: maintain an in-memory Set of last N processed EventIDs (use a sync.Mutex-protected *golang.org/x/exp/slices for memory safety if we want to track thousands; alternatively limit to say 1000 entries using LRU). If the incoming message `event_id` is already in the set, deliver NOOP silently.
- **Dead-letter handling**: if handler returns an unhandled error that isn't recoverable (e.g., validation), call `msg.NatsAck()` but also log a specific "poison message detected". Never call `msg.Nak()` as this will replay indefinitely. Always Acknowledge poison messages to prevent consumer starvation.
- **Error handling**: if the handler returns an error, implement exponential backoff retry logic (up to 5 attempts with jitter): `delay = min(baseDelay * math.Pow(2, attempt) + randomJitter, maxDelayCaps)`. After max retries exceeded call DeadLetter() / Ack manually to prevent infinite replay.
- **Backpressure**: don't let the consumer goroutine block indefinitely; use a bounded worker pool of 5 concurrent processing workers via channels. If the channel buffer is full (say capacity = 10), drop oldest message in the queue when full (fail closed, never fail open). Log this explicitly as "message dropped due to backpressure".

After writing the code, ensure both Producer and Consumer implement graceful Shutdown:
- Producer returns on context cancellation via context.Done()
- Consumer calls `msg.Ack()` properly for each received message to acknowledge it before processing completes (before acknowledging or NACKing). Use `msg.TerminalContext(done)`. Don't block on close.
