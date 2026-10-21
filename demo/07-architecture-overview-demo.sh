#!/usr/bin/env bash
# ===================================================================
# 07-architecture-overview-demo.sh
# Walks through the entire project's architecture, layers, patterns,
# and key design decisions — with source references.
# ────────────────────────────────────────────────────────────────────

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BOLD="\033[1m"; GREEN="\033[0;32m"; RED="\033[0;31m"
CYAN="\033[0;36m"; YELLOW="\033[0;33m"; GRAY="\033[0;90m"; MAGENTA="\033[0;35m"; BLUE="\033[1;34m"; ORANGE="\033[0;33m"
NC="\033[0m"

h()   { printf "\n${BOLD}${MAGENTA}═══ %s ═══${NC}\n\n" "$1"; }
sub() { printf "  ${CYAN}▸ %s:${NC}" "$1"; }
ok()  { printf "\n  ${GREEN}✓ %s${NC}" "$1"; }
info(){ printf "${GRAY}%b${NC}" "$1"; }

echo ""
printf "${BOLD}${ORANGE}"
printf "  ═══════════════════════════════════════════════════${NC}\n"
printf "           ${BOLD}Demo 07: Architecture & Project Walkthrough${NC}         \n"
printf "  ═══════════════════════════════════════════════════${NC}"
printf "${NC}\n\n"

# ━━━━━━━━━━━ A. DIRECTORY STRUCTURE ┃━━━━━━━━━━━━━━━━━━━━━━━
h "A-1 — Project Directory Layout"

cat <<'EOF'
tenant-provisioning-control-plane/
├── cmd/
│   ├── server/main.go          # Entry-point: HTTP server (Gin, NATS client, consumer)
│   └── worker-sim/main.go      # Simulated worker: consumes outbox → publishes progress
├── internal/
│   ├── config/config.go        # Environment-based config (DB path, NATS URL, retries)
│   ├── handler/
│   │   ├── tenant_handler.go   # POST/GET/PATCH/DELETE /v1/tenants/:id
│   │   └── task_handler.go     # GET /v1/tasks/:id (read-only)
│   ├── lifecycle/
│   │   ├── tenant_fsm.go       # Tenant state machine + ErrInvalidTransition
│   │   ├── task_fsm.go         # Task state machine + terminal/invalid transitions
│   │   ├── open_task_guard.go  # Prevents creating tenants if open tasks exist
│   │   ├── order_guard.go      # Ensures sequential updates to same tenant
│   │   └── retry.go            # Retry logic for outbox/NATS operations
│   ├── middleware/
│   │   ├── logging.go          # Request/response logging middleware
│   │   ├── recovery.go         # Panic recovery → HTTP 500
│   │   └── request_id.go       # Unique request ID (Gin middleware)
│   ├── model/
│   │   ├── tenant.go           # Tenant ORM struct + validations
│   │   └── task.go             # Task ORM struct + status validation
│   ├── nats/dispatcher.go      # NATS publisher for outbox events
│   ├── outbox/
│   │   ├── adapter.go          # SQLite back-end for outbox_events table
│   │   └── outbox.go           # Outbox pattern: commit event → async publish
│   ├── service/
│   │   ├── tenant_service.go   # Core business logic: CRUD + transitions
│   │   └── task_service.go     # Task lookup, status updates, FSM validation
│   ├── store/
│   │   ├── tenants.go          # SQLite CRUD for tenants table
│   │   ├── tasks.go            # SQLite CRUD for tasks table
│   │   ├── optimistic_locking_test.go  # Version-based concurrency control
│   │   └── concurrent_create_race_test.go # Guard integration tests
│   └── error/
│       ├── gin_error.go        # Standard error → HTTP mapping (400/404/422/500)
├── router/router.go            # Gin router setup, middleware wiring
├── go.mod / go.sum             # Go dependencies
└── docker-compose.yml          # Multi-service orchestration (server + NATS + worker-sim)
EOF

# ━━━━━━━━━━━ B. HIGH-LEVEL ARCHITECTURE DIAGRAM ┃━━━━━━━━━
h "A-2 — High-Level Architecture Diagram"

cat <<'EOF'
  ┌─────────────────────────────────────────────────────────────┐
  │                    Client (curl / frontend)                 │
  └──────────────────┬──────────────────────────────────────────┘
                     │ HTTP POST/GET/PATCH/DELETE /v1/*.
                     ▼
┌──────────────────────────────────────────────────────────────────┐
│                      tpcp-server-1 (HTTP API)                    │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐ │
│  │ TenantHandler│  │ TaskHandler  │  │    Gin Middleware     │ │
│  │ CRUD Routes  │  │ GET :id only │  │ logging / recovery /  │ │
│  └──────┬───────┘  └──────────────┘  │     req-id           │ │
│         │                             └──────────┬───────────┘ │
│         ▼                                        │             │
│  ┌────────────────────────────────────────────────┴───────────┐ │
│  │                   TenantService / TaskService              │ │
│  │   • FSM validation (tenant_fsm.go / task_fsm.go)          │ │
│  │   • Optimistic locking (version column)                    │ │
│  │   • Outbox event creation (single DB tx)                  │ │
│  └─────────────────────┬──────────────────────────────────────┘ │
└────────────────────────┼────────────────────────────────────────┘
                         │ Single DB transaction:
                         │ INSERT tenant+task+outbox_event
              ┌──────────┴──────────┐
              ▼                     ▼
  ┌───────────────────┐   ┌─────────────────────┐
  │   SQLite DB       │   │   NATS JetStream     │
  │   (tenant.db)     │   │   stream: TASKS_CREATE│
  │                   │   │   outbox_events table │
  │ ┌───────────────┐ │   │                     │
  │ │ tenants       │ │   │  ┌────────────────┐ │
  │ │ tasks         │ │   │  │NATS Publisher:   │ │
  │ │ outbox_events │ │◄──│  │ scans interval │ │
  │ └───────────────┘ │   │  │ commits events  │ │
  └───────────────────┘   │  └───────┬────────┘ │
                           └──────────┼──────────┘
                                      │
                            NATS subject: tasks.create.<id>
                                      ▼
              ┌───────────────────────────────────────┐
              │       tpcp-sim-1 (Worker-Sim)          │
              │   ┌──────────────────────────────────┐ │
              │   │  • Subscribe to NatsJetStream     │ │
              │   │  • Poll outbox_events periodically│ │
              │   │  • Read task type + tenant_id     │ │
              │   │  • Simulate work (100–3000ms)    │ │
              │   │  • Publish tasks.progress.<id>    │ │
              │   └──────────────────────────────────┘ │
              └───────────────────────┬────────────────┘
                                      │ NATS progress event
                                      ▼
              ┌───────────────────────────────────────┐
              │     Back to API Server Consumer         │
              │   • Receives tasks.progress.<task-id>  │
              │   • Calls task_service.UpdateProgress ()│
              │   • validateTaskTransition → update DB │
              │   → status accepted→in_progress→done/f │
              └────────────────────────────────────────┘
EOF

# ━━━━━━━━━━━ C. KEY DESIGN PATTERNS ┃━━━━━━━━━━━━━━━━━━━━━
h "A-3 — Key Design Patterns"

cat <<'EOF'
1. OUTBOX PATTERN (internal/outbox/)
   Problem: How to guarantee event publication without losing messages.
   Solution: Write outbox_event row in the SAME DB transaction as the
             tenant/task insert. A background goroutine (dispatcher.go)
             scans pending events and publishes to NATS, incrementing
             attempts column on each try (max 10 retries).

2. OPTIMISTIC LOCKING (internal/service/tenant_service.go + store/)
   Problem: Concurrent UPDATE requests could overwrite each other.
   Solution: Every tenant row has a version column. On PATCH with a
             non-zero version, the WHERE clause includes WHERE version = N.
             Zero rows affected → ErrOptimisticLock conflict returned.

3. TENANT FSM (internal/lifecycle/tenant_fsm.go)
   Problem: Prevent invalid state transitions (e.g., active→provisioning).
   Solution: A map[string][]string of valid transitions. ValidateTenantTransition()
             is called in UpdateTenant() BEFORE any DB write. Returns
 * ErrInvalidTransition → HTTP 422.

4. TASK FSM (internal/lifecycle/task_fsm.go)
   Problem: Workers need a constrained workflow for task lifecycle.
   Solution: accepted→in_progress→done|failed. Terminal states block further
             transitions. Validated in ValidateTaskTransition() inside consumer.

5. OPEN-TASK GUARD (internal/lifecycle/open_task_guard.go)
   Problem: Prevent creating a new tenant while other tenants have pending tasks.
   Solution: Check if any task exists with status NOT IN ('done','failed') before
             allowing CREATE. Prevents resource contention per-tenant.

6. ORDER GUARD (internal/lifecycle/order_guard.go)
   Problem: Concurrent PATCH requests for the same tenant must be serialized.
   Solution: Uses an in-memory sync.Map-based mutex keyed by tenant_id to
             serialize concurrent updates, avoiding race conditions on DB writes.
EOF

# ━━━━━━━━━━━ D. API ENDPOINTS REFERENCE ┃━━━━━━━━━━━━━━━━━━━
h "A-4 — Complete API Surface"

cat <<'EOF'
┌───────┬──────────────────┬──────────────────────────┬─────────────────────┐
│ Method│ Endpoint          │ Request Body              │ Response            │
├───────┼──────────────────┼──────────────────────────┼─────────────────────┤
│POST   │/v1/tenants       │{slug, name}             │ 201: {data:{id,     │
│       │                  │                          │ slug, name, status:  │
│       │                  │                          │ provisioning,        │
│       │                  │                          │ version: 0}}        │
│       │                  │                          │ 400: validation err │
├───────┼──────────────────┼──────────────────────────┼─────────────────────┤
│GET    │/v1/tenants       │?status=<state>&page=&   │ 200: {data:[...],   │
│       │                  │ limit=                   │ count}              │
├───────┼──────────────────┼──────────────────────────┼─────────────────────┤
│GET    │/v1/tenants/:id   │ —                        │ 200: {data:{...}}   │
│       │                  │                          │ 404: not found      │
├───────┼──────────────────┼──────────────────────────┼─────────────────────┤
│PATCH  │/v1/tenants/:id   │{name?, status?, version?}│ 200: {data:{...}}   │
│       │                  │ → update tenant fields,  │ 422: FSM violation │
│       │                  │ validates transition     │                     │
├───────┼──────────────────┼──────────────────────────┼─────────────────────┤
│DELETE │/v1/tenants/:id   │ —                        │ 202: destroy cycle   │
│       │                  │                          │ started (status=    │
│       │                  │                          │ destroying)         │
│       │                  │                          │ 422: invalid state ┤
├───────┼──────────────────┼──────────────────────────┼─────────────────────┤
│GET    │/v1/tasks/:id     │ —                        │ 200: {data:{...}}   │
│       │                  │                          │ 404: not found      │
└───────┴──────────────────┴──────────────────────────┴─────────────────────┘

NOT exposed (by design):
  POST/PUT/DELETE /v1/tasks — tasks are managed via outbox events only
  GET /v1/tasks?tenant=    — no list endpoint; query SQLite directly if needed
EOF

# ━━━━━━━━━━━ E. DATA MODEL REFERENCE ┃━━━━━━━━━━━━━━━━━━━━
h "A-5 — Data Model Reference"

cat <<'EOF'
┌───────────────────────────┐     ┌───────────────────────┐
│  tenants                  │     │  tasks                │
├───────────────────────────┤     ├───────────────────────┤
│ id        (UUID, PK)      │◄────id (FK → tenants)      │
│ slug      (string, UNI)   │     │ tenant_id (string FK) │
│ name      (string)        │     │ type        (string)  │
│ status    (enum*)         │     │ status    (enum**)    │
│ version   (int64)         │     │ payload   (JSON?)     │
│ created_at (timestamp)    │     │ attempts  (int)       │
│ updated_at (timestamp)    │     │ created_at            │
└───────────────────────────┘     │ updated_at            │
                                  └───────────────────────┘

┌───────────────────────────┐
│  outbox_events            │
├───────────────────────────┤
│ id        (UUID, PK)      │
│ tenant_id (string FK)     │
│ subject   (string)        │  tasks.create.<tenant-id>
│ payload (BLOB/JSON)       │  {taskType: "deploy", taskID}
│ status    (enum)          │  pending | published | failed
│ attempts(int)             │
│ created_at (timestamp)    │
└───────────────────────────┘

Tenant statuses (*): provisioning | active | updating | destroying | destroyed | failed
Task statuses (**): accepted | in_progress | done | failed
EOF

# ━━━━━━━━━━━ F. DEPLOYMENT ARCHITECTURE ┃━━━━━━━━━━━━━━━━━
h "A-6 — Deployment Docker Composition"

cat <<'EOF'
┌────────────────────── docker-compose.yml ────────────────────┐
│                                                              │
│  service: tpcp-server    → Go HTTP API + NATS client       │
│    port: 8080                                              │
│    volume: data:/app/data (SQLite persistent)              │
│                                                              │
│  service: tpcp-nats      → NATS JetStream broker           │
│    port: 4222, 8222 (dashboard)                             │
│    stream: TASKS_CREATE (for outbox_events)                │
│                                                              │
│  service: tpcp-sim       → Worker simulator               │
│    watches NATS > tasks.create.<subject>                    │
│    simulates async processing                               │
│    publishes progress via outbox retry loop                 │
└──────────────────────────────────────────────────────────────┘
EOF

# ━━━━━━━━━━━ G. ERROR MAPPING REFERENCE ┃━━━━━━━━━━━━━━━━━
h "A-7 — Error → HTTP Mapping"

cat <<'EOF'
Error Type                      → HTTP Code → User Message
─────────────────────────────────────────────────────────
Invalid JSON / bind fail        → 400       → "invalid request body"
Slug fails regex validation      → 400       → "invalid request body"
Field missing (slug/name)          → 400       → "invalid request body"
ErrOptimisticLock                → 409       → "record has been modified, retry"
ErrNotFound                       │ 404       → "not found"
lifecycle.ErrInvalidTransition   │ 422       │ "tenant state transition not allowed: X -> Y"
lifecycle.ErrInvalidTaskTransition│(consumer)│  logged, not written to DB
Unknown/unexpected               │ 500       │ "internal error"

All errors include request_id (middleware/request_id.go) for tracing.
EOF

# ━━━━━━━━━━━ H. RUN THE DEMOS IN ORDER ┃━━━━━━━━━━━━━
h "A-8 — Recommended Demo Sequence (Run All In Order)"

cat <<'EOF'
Execute these scripts sequentially for the full walkthrough:

  ./demo/01-start-infrastructure.sh   → Start API, NATS, worker-sim
  ./demo/02-tenant-lifecycle-demo.sh  → Full CRUD lifecycle on tenants
  ./demo/03-error-handling-demo.sh    │ Validation errors & FSM violations
  ./demo/04-task-api-demo.sh          │ Task creation, fetching via API + DB
│ ./.demo/05-outbox-worker-demo.sh   │ Worker-sim consumes outbox → updates tasks
  ./demo/06-fsm-state-machine-demo.sh │ Valid/invalid transitions for both FSMs
  ./demo/demo-all.sh                  │ Orchestrate (runs 01–04 automatically)

Each script demonstrates real HTTP calls with actual payloads and verified responses.
EOF

printf "\n${BOLD}${GREEN}%s${NC}\n\n" "✓ Demo 07 complete! Architecture overview displayed."
