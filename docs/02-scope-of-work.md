# Scope of Work — Tenant Provisioning Control Plane

**Language**: Go  
**Primary stack**: Standard library `net/http` or `gin`, SQLite, NATS JetStream  
**Deployment**: Docker Compose (single `docker compose up` boots everything)

---

## 1. Project Structure

```
tenant-provisioning-control-plane/
├── docker-compose.yml          # Boots NATS + the Go service + worker simulator
├── Dockerfile                  # Multi-stage build: builder → minimal alpine image
├── Makefile                    # Single command boot (make up) and test (make test)
├── README.md                   # Setup docs, API examples, design decisions
├── cmd/
│   ├── server/main.go          # HTTP service entry-point (health, routers, signal handling)
│   └── worker-sim/main.go      # Worker simulator CLI (accepts --fail-rate, --min-delay-ms, --max-delay-ms)
├── internal/
│   ├── model/                  # Domain types: Tenant, Task, enums for status
│   │   ├── tenant.go           # Tenant struct + validation rules (slug regex, etc.)
│   │   └── task.go             # Task struct + lifecycle states
│   ├── store/                  # Persistence layer
│   │   ├── store.go            # Interface definition (TenantStore, TaskStore)
│   │   └── sqlite.go           # SQLite implementation using sqlc or raw database/sql.
│   ├── broker/                 # Message broker abstraction
│   │   ├── broker.go           # Interface (ProduceTask, ConsumeProgress, BindErrorHandler)
│   │   └── nats_impl.go        # NATS JetStream producer & consumer implementation
│   ├── api/                    # HTTP layer — all REST endpoints
│   │   ├── tenant_handler.go   # CRUD + state-machine guards: POST/PATCH/DELETE tenants
│   │   ├── task_handler.go     # Read-only list/get for tasks
│   │   └── router.go           # Route registration, middleware (logging, panic recovery)
│   ├── lifecycle/              # Tenant task lifecycle & state machine implementation
│   │   ├── tenant_fsm.go       # Validate transitions, apply terminal outcomes.
│   │   └── optimistic_lock.go  # Version-based concurrency protection
│   └── idempotency/           # Idempotent message consumer logic
│       └── consumer.go         # Deduplication map + stale-update rejection
├── testdata/                   # Fixture seeds, sample payloads, curl scripts for scenarios
└── .github/                    # If desired: CI workflow (linting + tests on push)
```

---

## 2. REST API Contract

### Tenants — `POST /v1/tenants`

| Aspect      | Detail                                                              |
| ----------- | ------------------------------------------------------------------- |
| Method      | `POST`                                                              |
| URL         | `/v1/tenants`                                                       |
| Request     | `{ name: string }` (slug optional, server-generated if omitted)    |
| Response    | 201 Created — `{ ... tenant fields including status="provisioning" ...}` |
|             | 409 Conflict — `{"error":"tenant_already_exists", ...}`            |

### Tenants GET `/v1/tenants`

| Aspect      | Detail                                                              |
| ----------- | ------------------------------------------------------------------- |
| Method      | `GET`                                                               |
| URL         | `/v1/tenants?status=active&page=1&per_page=20`                    |
| Response    | 200 OK — `{ items: [ ... ], pagination: { total, next_cursor } }` |

### Tenant GET `/v1/tenants/:id`

| Aspect      | Detail                                                    |
| ----------- | --------------------------------------------------------- |
| Method      | `GET`                                                     |
| URL         | `/v1/tenants/{tenantId}`                                |
| Response    | 200 OK — tenant object                                  |
|             | 404 Not Found — `{"error":"tenant_not_found", ...}`     |

### Tenant PATCH `/v1/tenants/:id`

| Aspect      | Detail                                                            |
| ----------- | ----------------------------------------------------------------- |
| Method      | `PATCH`                                                           |
| URL         | `/v1/tenants/{tenantId}?version=3`                              |
| Request     | `{ name: string }` (partial update)                             |
| Response    | 200 OK — updated tenant, status may be `"updating"`              |
|             | 409 Conflict — `{"error":"tenant_version_conflict", ...}`        |
|             | 422 Unprocessable Entity — `{"error":"tenant_update_not_allowed" ...}` (from invalid state transition) |

### Tenant DELETE `/v1/tenants/:id`

| Aspect      | Detail                                                             |
| ----------- | ------------------------------------------------------------------ |
| Method      | `DELETE`                                                           |
| URL         | `/v1/tenants/{tenantId}`                                         |
| Response    | 200 OK — deleted tenant, status may be `"destroying"`             |
|             |   403 Forbidden (status != `active` or `failed`)                  |
|             |   404 Not Found                                                   |

### Tasks GET `/v1/tenants/:id/tasks?page=1&per_page=20`

| Aspect      | Detail                                                    |
| ----------- | --------------------------------------------------------- |
| Method      | `GET`                                                     |
| URL         | `/v1/tenants/{tenantId}/tasks`                          |
| Response    | 200 OK — list of tasks for tenant (read-only)           |

### Task GET `/v1/tasks/:id`

| Aspect      | Detail                                                    |
| ----------- | --------------------------------------------------------- |
| Method      | `GET`                                                     |
| URL         | `/v1/tasks/{taskId}`                                    |
| Response    | 200 OK — task object                                    |
|             | 404 Not Found — `{"error":"task_not_found", ...}`       |

### Health Check

| Aspect      | Detail                                                    |
| ----------- | --------------------------------------------------------- |
| Method      | `GET`                                                     |
| URL         | `/healthz`                                                |
| Response    | 200 OK — `{"status": "ok"}`                             |

---

## 3. Error Catalog (machine-readable)

| `code`                       | HTTP status | Triggering condition                                            |
| ---------------------------- | ----------- | --------------------------------------------------------------- |
| `tenant_not_found`           | 404         | GET/PATCH/DELETE by non-existent ID                             |
| `tenant_already_exists`      | 409         | POST with slug colliding an existing or destroyed tenant        |
| `tenant_update_not_allowed`  | 422         | PATCH from invalid state (not `active`)                         |
| `tenant_version_conflict`    | 409         | PATCH with wrong `version` query param                          |
| `task_not_found`             | 404         | GET `/v1/tasks/{taskId}` missing                               |
| `invalid_request`            | 422         | Validation failure (empty name, bad slug, bad version number)   |

All errors share this shape:
```json
{
  "error": {
    "code": "tenant_version_conflict",
    "message": "Concurrent mutation detected; please refresh and retry",
    "request_id": "abc-123"
  }
}
```

---

## 4. Message Broker — NATS JetStream Streams & Consumers

| Stream                    | Subject Pattern                        | Purpose                            |
| ------------------------- | -------------------------------------- | ---------------------------------- |
| `task_outbound`           | `tasks.create.*`                       | Control plane → worker (publishes new deploy/update/destroy tasks) |
| `task_progress`           | `tasks.progress.*`                     | Worker → control plane (progress updates: accepted → in_progress → done/failed) |

Each stream has a single durable consumer per tenant (for ordering within the same task lifecycle).

**Producer envelope**:
```json
{
  "type": "task_created",
  "event_id": "<task.id>",
  "data": {
    "id": "...",
    "type": "deploy|update|destroy",
    "tenant_id": "...",
    "status": "accepted"
  }
}
```

**Consumer envelope**:
```json
{
  "kind": "task_progress_update",
  "event_id": "<unique-uuid-per-delivery>",      // idempotency key
  "data": {
    "task_id": "...",                            // stable reference to the task
    "status":         "in_progress|done|failed"
    "updated_at":     "2025-..."
  }
}
```

---

## 5. Worker Simulator — CLI Flags

| Flag             | Type   | Default      | Description                                            |
| ---------------- | ------ | ------------ | ------------------------------------------------------ |
| `--fail-rate`    | float  | `0.0`        | Probability of task failure (0.0 = never, 1.0 = always) |
| `--min-delay-ms` | int    | `200`       | Minimum sleep before responding (simulated work time)   |
| `--max-delay-ms` | int    | `2000`      | Maximum sleep before responding                         |
| `--broker-url`   | string | `nats://localhost:4222` | NATS connection URL                              |

---

## 6. Quality Gates (via Makefile / Docker)

| Gate              | Tool               | Command                           |
| ----------------- | ------------------ | --------------------------------- |
| Formatting        | `gofmt -s -w .`    | `make fmt`                          |
| Static analysis   | `staticcheck ./...` | `make lint`                        |
| Security scanner  | `trivy fs --severity HIGH,CRITICAL .` | `make security` (optional) |
| Unit tests        | `go test -race -count=1 ./...`    | `make test`                   |
| Integration tests | `./test/integration/run.sh`       | `make integration-test`      |
| Full CI           | `make ci`          | Formatter + linter + security scan + unit + integration
