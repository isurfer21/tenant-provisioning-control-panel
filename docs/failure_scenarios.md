# Failure Scenarios & Demo Guide

Reproducible negative-test cases for interview and demo purposes. Every scenario includes the exact CLI commands to trigger it, expected response codes, and what to look for in logs.

**Prerequisites:** All services running via `make boot`. The worker-sim uses `--fail-rate=0.0` by default — all scenarios require overriding this flag.

---

## Scenario 1: --fail-rate Worker Failure Simulation

Demonstrates graceful handling of a worker that fails to complete provisioning tasks.

### Steps

```bash
# 1. Start the worker simulator with 100% simulated failures
go run cmd/worker-sim/main.go --fail-rate=1.0

# (Or in docker-compose, override FAIL_RATE=1.0)
docker compose up --build -d worker-sim
```

```bash
# 2. Create a tenant via the API
curl -s -X POST http://localhost:8080/v1/tenants \
  -H 'Content-Type: application/json' \
  -d '{"slug":"fail-demo","name":"Failure Demo Tenant"}'
```

### What to observe

| Observable | Where to look | Details |
|---|---|---|
| Outbox event created | `worker-sim` stderr | `[worker-sim] FAIL task=<task-id>` logged for every received event, confirming the simulate-fail path |
| Task remains in `accepted`/`in_progress` | `curl http://localhost:8080/v1/tasks/<task-id>` | Tenant status stays `provisioning` because the worker never published a progress update |

### Cleanup

Stop the fail-rate-1 worker (Ctrl-C); deploy a normal one (`--fail-rate=0.0`) and re-create the tenant to see successful provisioning.

---

## Scenario 2: Concurrent PATCH Optimistic Lock Conflict

Demonstrates the `tenant_version_conflict` guard (T9) when two clients PATCH the same tenant simultaneously.

### Steps

```bash
# 1. Create a fresh tenant
curl -s -X POST http://localhost:8080/v1/tenants \
  -H 'Content-Type: application/json' \
  -d '{"slug":"race-demo","name":"Race Demo"}' > /tmp/tenant.json

TENANT_ID=$(cat /tmp/tenant.json | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['id'])")
VERSION=1

# 2. Two concurrent PATCH requests using the SAME version
curl -s -X PATCH http://localhost:8080/v1/tenants/$TENANT_ID \
  -H 'Content-Type: application/json' \
  -d '{"name":"Update A","version":'$VERSION'}' &

curl -s -X PATCH http://localhost:8080/v1/tenants/$TENANT_ID \
  -H 'Content-Type: application/json' \
  -d '{"name":"Update B","version":'$VERSION'}' &

wait
```

### Expected Result

| | Response |
|---|---|
| **First writer** | `200 OK` — body contains `"version": 2` |
| **Second writer** | `409 Conflict` — body contains `"code":"tenant_version_conflict"` (or similar) |

### Verify in logs

Check the API server log for the 409 response line — it should show the mapped error code from `internal/error/gin_error.go`.

---

## Scenario 3: Duplicate Event Replay (Idempotency)

Demonstrates T22: processed-events table prevents secondary mutations when a message is redelivered.

### Steps

```bash
# 1. Create a tenant → triggers an outbox event
curl -s -X POST http://localhost:8080/v1/tenants \
  -H 'Content-Type: application/json' \
  -d '{"slug":"dup-demo","name":"Dup Demo"}'

# (Ensure the worker-sim is running with --fail-rate=0.0 to consume and publish progress)

# 2. Inject a duplicate message directly into NATS JetStream via nats CLI:
nats stream get TASKS_CREATE
nats pub 'tasks.progress.<task_id>' '{"event_id":"<same-event-id>","tenant_id":"<uuid>","task_id":"<uuid>","status":"done"}'

# 3. Observe no double-mutation in the SQLite `tenants` / `tasks` table
```

### How to verify

Query the SQLite DB directly:

```bash
sqlite3 data/tenant.db "SELECT status, version FROM tenants WHERE slug='dup-demo';"
sqlite3 data/tenant.db "SELECT status FROM tasks WHERE tenant_id=(SELECT id FROM tenants WHERE slug='dup-demo');"
```

The `processed_events` table will contain one row for the duplicated event_id — proving the dedup key was applied on the first pass and silently rejected the second.

---

## Scenario 4: Out-of-Order Delivery (Stale Update Rejection)

Demonstrates T23: when a completed task delivers an `in_progress` update out of order, the FSM guard blocks it.

### Steps

```bash
# 1. Create a tenant and let it complete fully (worker-sim at --fail-rate=0.0)
curl -s -X POST http://localhost:8080/v1/tenants \
  -H 'Content-Type: application/json' \
  -d '{"slug":"ooo-demo","name":"Out of Order Demo"}'

# Wait for task → done (check via GET /v1/tasks/<id>)

# 2. Manually publish an OOO progress event (simulating a delayed network path):
nats pub 'tasks.progress.<task_id>' '{"event_id":"<uuid>","tenant_id":"<uuid>","task_id":"<task-id>","status":"in_progress"}'
```

### Expected Result

The API server's inbound consumer **rejects** the `done → in_progress` transition silently: a guard in `internal/lifecycle/task_fsm.go` blocks the illegal state regression. The task remains `done`, and no log error is emitted.

---

## Scenario 5: DLQ / Poison Payload Inspection

Demonstrates T24's Dead Letter Queue for malformed events.

### Steps

```bash
# 1. Publish a malformed event (missing required fields) to the create stream:
nats pub 'tasks.create.bogus' '{"partial":"data"}'

# 2. The worker-sim will NAK this message. On repeated failures (attempts >= 10),
#    NATS automatically moves the msg to its DLQ subject prefix:
#    
#    _JS.DELIVERED.<stream>.<consumer>  or  via the JetStream API:

nats stream info TASKS_CREATE
nats consumer info TASKS_CREATE worker-sim | grep -i drq
```

### What to look for

| Observable | Details |
|---|---|
| Worker stderr | `[worker-sim] parse fail: ...` when the malformed payload arrives |
| JetStream DLQ stats | `nats consumer info TASKS_CREATE worker-sim` shows a non-zero **Delivered Count** for the DLQ subject (or `"Drained"` > 0 in the consumer config) |

### Inspect DLQ messages directly

```bash
nats stream get TASKS_CREATE
nats kv get  # if a KV-backed DLQ bucket is configured
# Or: nats consumer messages TASKS_CREATE worker-sim -c 1 --subject '_NATS._DR.<topic>'
```

---

## Scenario Checklist for Demo

| # | Scenario | Time | Key Takeaway |
|---|---|---|---|
| 1 | `--fail-rate=1.0` worker | 2 min | System never crashes; events accumulate in outbox |
| 2 | Concurrent PATCH conflict | 1 min | Optimistic locking returns actionable 409 |
| 3 | Duplicate event replay | 2 min | Idempotency guard eliminates double-mutations |
| 4 | OOO delivery rejection | 2 min | FSM state regression is silently blocked |
| 5 | DLQ poison detection | 2 min | Malformed payloads are isolated, not lost |

**Total demo time: ~9 minutes.**

---

## Notes for the Interviewer

- All error responses follow a **standardized JSON schema**: `{"code": "...", "message": "...", "request_id": "..."}` (defined in `internal/error/gin_error.go`).
- The control plane is **stateless on the API layer** — any instance can pick up requests; NATS provides the stateful event backbone.
- A full end-to-end test (`TestE2ELifecycle` at `internal/lifecycle/e2e_lifecycle_test.go`) validates every path above through real HTTP + NATS traffic against an in-memory SQLite DB.
