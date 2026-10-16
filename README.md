# Tenant Provisioning Control Plane

A **multi-tenant SaaS control plane** built in Go (Gin) with event-driven tenant lifecycle management via NATS JetStream, the Transactional Outbox pattern, and SQLite persistence.

---

## Architecture

```mermaid
sequenceDiagram
    participant HTTP as API Server (:8080)
    participant OB as Outbox Table
    participant NATS as NATS JetStream
    participant Worker as Worker Simulator
    participant DB as SQLite

    HTTP->>DB: BEGIN TX
    HTTP->>DB: INSERT tenant (status=provisioning)
    HTTP->>DB: INSERT task (type, status=accepted)
    HTTP->>DB: INSERT outbox_events
    HTTP->>DB: COMMIT
    HTTP-->>Client: 201 Created

    NATS->>Worker: Consume tasks.create.<tenant_id>
    Worker->>Worker: Simulate work (+ random failure)
    Worker->>NATS: Publish tasks.progress.<task_id>
    NATS->>DB: Update task + tenant FSM state
```

### Component Map

| Layer          | Technologies                                                                                                                                                                                    |
|:---------------|:------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| API Gateway    | Gin (Go), middleware chain (PanicRecovery → RequestID → Structured Logger)                                                                                                                      |
| Persistence    | SQLite (`go-sqlite3`), schema migrations via `internal/migrate/`                                                                                                                                |
| Event Backbone | NATS JetStream (`tasks.create.>` stream, `tasks.progress.>` subject prefix)                                                                                                                     |
| Reliability    | Transactional Outbox (T18), Idempotent processing via `processed_events` table (T22), Exponential retry + DLQ (T24)                                                                             |
| Guards         | Optimistic locking (`UPDATE ... WHERE version=?`; T9), Tenant FSM validity map (T10), Task FSM terminal-state guards (T11), Open-task concurrency guard (T12), Slug uniqueness constraint (T13) |

---

## Quick Start

Requirements: [Go 1.22+](https://go.dev/dl/), [Docker + Docker Compose](https://docs.docker.com/compose/).

```bash
# 1. Build & bring up NATS, API, and Worker simulator
make boot
# Equivalent: docker compose up --build

# 2. Run tests (race detector enabled)
make test
# Equivalent: go test -race ./...

# 3. Format code
make fmt
```

### Service Ports

| Service        | Port   | Purpose            |
|:---------------|:-------|:-------------------|
| API Server     | `8080` | HTTP REST API      |
| NATS Dashboard | `8222` | JetStream admin UI |

### Environment Variables

| Variable    | Default               | Description                      |
|:------------|:----------------------|:---------------------------------|
| `PORT`      | `8080`                | HTTP listen port                 |
| `DB_PATH`   | `/app/data/tenant.db` | SQLite file path                 |
| `NATS_URL`  | `nats://nats:4222`    | NATS JetStream connection string |
| `LOG_LEVEL` | `INFO`                | Structured logger level          |

---

## API Reference

Base URL: `http://localhost:8080/v1`

### Tenant CRUD

```bash
# Create a tenant → status = provisioning (returns 201)
curl -s -X POST http://localhost:8080/v1/tenants \
  -H 'Content-Type: application/json' \
  -d '{"slug":"acme-corp","name":"Acme Corporation"}'

# List tenants (filter by status)
curl -s 'http://localhost:8080/v1/tenants?status=active&page=1&limit=20'

# Get a tenant by ID
curl -s http://localhost:8080/v1/tenants/<tenant-id>

# Update a tenant (requires correct version for optimistic locking)
curl -s -X PATCH http://localhost:8080/v1/tenants/<tenant-id> \
  -H 'Content-Type: application/json' \
  -d '{"name":"Acme Corp.","version":3,"status":"updating"}'

# Delete a tenant
curl -s -X DELETE http://localhost:8080/v1/tenants/<tenant-id>
```

### Health Check

```bash
curl -s http://localhost:8080/healthz
# → {"status":"ok"}
```

---

## Design Decisions

### Why Go & Gin?
- **Zero GC pauses under load** compared to Python for a control-plane that must handle concurrent tenant provisioning without jitter.
- **Gin's router** gives us explicit handler registration (one file, one `Register` method per resource), keeping the project tree flat and testable.

### Why NATS JetStream over Kafka?
- **Single dependency** instead of a ZooKeeper/KRaft cluster — ideal for a self-contained workshop/demo deployment via Docker Compose.
- **Built-in consumer groups, ack policy, and stream retention** let us implement T20 (worker simulator), T21 (inbound progress consumer), and T24 (retries + DLQ) with fewer infrastructure hoops.

### Why Transactional Outbox?
A direct emit after a DB commit creates a race: the tenant record exists but no event fires. The outbox table solves this by making **emit** merely an atomic DB row. A background fetcher (the worker-sim acts as both publisher and consumer in this demo) picks up `attempts < 10` rows, publishes them, and bumps attempts on failure.

### Why Optimistic Locking?
The control plane must prevent a classic TOCTOU: two API clients reading version `3`, both PATCHing with version `3`. The `UPDATE ... WHERE version=...` guard ensures only the first writer wins; others receive a `409 Conflict`.

---

## Worker Simulator (CLI)

The worker-sim consumes provisioning events and publishes progress updates. Run it standalone:

```bash
go run cmd/worker-sim/main.go --fail-rate=0.2 --max-delay=3s
```

| Flag          | Description                              | Default |
|:--------------|:-----------------------------------------|:--------|
| `--fail-rate` | Probability of a simulated failure (0–1) | `0.0`   |
| `--max-delay` | Max random processing delay              | `5s`    |

In Docker Compose, the worker uses `FAIL_RATE`, `MIN_DELAY_MS`, and `MAX_DELAY_MS` environment variables.

---

## Testing

All tests live alongside their source code:

```bash
# FSM validation table-driven tests
go test ./internal/lifecycle -run TestTenantFSM

# Optimistic locking concurrency test (race detector)
go test ./internal/store -run TestOptimisticLockingConcurrency

# Full end-to-end lifecycle
go test ./internal/lifecycle -run TestE2ELifecycle
```

CI-equivalent via Makefile: `make test`.
