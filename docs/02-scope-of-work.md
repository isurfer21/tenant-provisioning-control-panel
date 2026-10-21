# Scope of Work — Tenant Provisioning Control Plane (Revised)

**Language:** Go  
**Primary Stack:** Gin, SQLite, NATS JetStream  
**Deployment:** Docker Compose (`docker compose up` boots complete system)  
**Architecture Style:** Control Plane + Worker Simulator + Event-Driven Processing

---

# 1. Objective

Build a multi-tenant SaaS Control Plane that:

1. Creates and manages tenant lifecycles.
2. Publishes asynchronous provisioning tasks.
3. Simulates worker processing through a message broker.
4. Tracks task progress.
5. Guarantees concurrency correctness.
6. Supports idempotent message consumption.
7. Survives restarts.
8. Demonstrates production-grade asynchronous workflow patterns.

---

# 2. High-Level Architecture

```
                        +-------------------+
                        |     REST API      |
                        |  Control Plane    |
                        +---------+---------+
                                  |
                       Creates Tenant + Task
                                  |
                                  v
                        +-------------------+
                        |     SQLite DB     |
                        | Tenant / Task     |
                        | Outbox Events     |
                        +---------+---------+
                                  |
                     Publish After Commit Only
                                  |
                                  v
                       +--------------------+
                       | NATS JetStream     |
                       | task_outbound      |
                       | task_progress      |
                       +---------+----------+
                                 |
                                 v
                      +---------------------+
                      | Worker Simulator    |
                      | CLI Process         |
                      +----------+----------+
                                 |
                        Progress Updates
                                 |
                                 v
                      +---------------------+
                      | Message Consumer    |
                      | Idempotent Handler  |
                      +---------------------+
```

---

# 3. Project Structure

```
tenant-provisioning-control-plane/

├── docker-compose.yml
├── Dockerfile.api
├── Makefile
├── README.md

├── cmd/
│   ├── server/
│   │   └── main.go
│   └── worker-sim/
│       └── main.go

├── internal/

│   ├── model/
│   │   ├── tenant.go
│   │   ├── task.go
│   │   └── event.go

│   ├── store/
│   │   ├── sqlite.go
│   │   ├── tenant_repository.go
│   │   ├── task_repository.go
│   │   └── outbox_repository.go

│   ├── api/
│   │   ├── tenant_handler.go
│   │   ├── task_handler.go
│   │   └── router.go

│   ├── service/
│   │   ├── tenant_service.go
│   │   ├── task_service.go
│   │   └── outbox_service.go

│   ├── lifecycle/
│   │   ├── tenant_fsm.go
│   │   ├── task_fsm.go
│   │   └── optimistic_lock.go

│   ├── broker/
│   │   ├── publisher.go
│   │   ├── consumer.go
│   │   └── jetstream.go

│   ├── idempotency/
│   │   └── consumer.go

│   ├── outbox/
│   │   └── dispatcher.go

│   └── middleware/
│       ├── logging.go
│       ├── recovery.go
│       └── request_id.go

├── migrations/
│   ├── 001_init.sql
│   ├── 002_outbox.sql
│   └── 003_processed_events.sql

├── tests/
│   ├── unit/
│   ├── integration/
│   └── concurrency/

└── .github/workflows/
```

---

# 4. Domain Model

## Tenant

```json
{
  "id": "uuid",
  "slug": "customer-a",
  "name": "Customer A",
  "status": "active",
  "version": 3,
  "created_at": "...",
  "updated_at": "..."
}
```

### Tenant Rules

|Property|Rule|
|---|---|
|id|Server generated UUID|
|slug|Required|
|slug|Regex: `^[a-z][-a-z0-9]{1,26}[a-z0-9]$`|
|slug|Unique forever, including destroyed tenants|
|name|Non-empty|
|version|Increment on every mutation|
|timestamps|ISO8601 UTC with milliseconds|

### Concurrency Requirement

Concurrent creates using the same slug:
```
Request A -> SUCCESS
Request B -> tenant_already_exists
```
Exactly one success is allowed.

---

## Task
```json
{
  "id": "uuid",
  "type": "deploy",
  "tenant_id": "uuid",
  "status": "accepted",
  "created_at": "...",
  "updated_at": "..."
}
```

### Task Types
```
deploy
update
destroy
```

### Task Status
```
accepted
in_progress
done
failed
```

---

# 5. State Machines

## Task FSM

```
accepted
    |
    v
in_progress
   / \
  v   v
done failed
```

### Rules

```
accepted -> in_progress
in_progress -> done
in_progress -> failed

done = terminal
failed = terminal
```

No transition may regress state.

---

## Tenant FSM

```
provisioning -> active      (deploy done)
provisioning -> failed      (deploy failed)

active       -> updating    (PATCH)
updating     -> active      (update done)
updating     -> failed      (update failed)

active       -> destroying  (DELETE)
failed       -> destroying  (DELETE)

destroying   -> destroyed   (destroy done)
destroying   -> failed      (destroy failed)
```

### Invalid Transitions

Rejected with:
```json
{
  "error": {
    "code": "tenant_update_not_allowed"
  }
}
```

---

# 6. Open Task Rule

A tenant may have:
```
MAX 1 OPEN TASK
```
Open states:
```
accepted
in_progress
```
Examples:
```
Tenant ACTIVE
   -> PATCH
   -> UPDATE TASK CREATED

Tenant UPDATING
   -> PATCH
   -> REJECT
```

No additional mutation is allowed while another task is open.

---

# 7. REST API

## Tenants

### POST /v1/tenants
```json
{
  "name": "Customer A",
  "slug": "customer-a"
}
```

Creates:
```
Tenant(status=provisioning)
Task(type=deploy,status=accepted)
```
Returns:
```
201 Created
```
Possible Errors
```
409 tenant_already_exists
422 invalid_request
```

---

### GET /v1/tenants

Filtering:
```
status
page
per_page
```
Returns paginated list.

---

### GET /v1/tenants/{id}

Returns tenant.

Possible errors:
```
404 tenant_not_found
```

---

### PATCH /v1/tenants/{id}?version=3

Allowed only from:
```
active
```
Creates:
```
update task
tenant -> updating
```
Possible errors:
```
404 tenant_not_found
409 tenant_version_conflict
422 tenant_update_not_allowed
```

---

### DELETE /v1/tenants/{id}

Allowed from:
```
active
failed
```
Creates:
```
destroy task
tenant -> destroying
```
Possible errors:
```
404 tenant_not_found
422 tenant_update_not_allowed
```

---

# 8. Task APIs

## GET /v1/tasks/{taskId}

Retrieve task details.

---

## GET /v1/tenants/{tenantId}/tasks

List tenant tasks.

Filters:
```
status
type
page
per_page
```
Tasks are read-only.

---

# 9. Error Contract

```json
{
  "error": {
    "code": "tenant_version_conflict",
    "message": "Concurrent modification detected",
    "request_id": "req-123"
  }
}
```

Supported Codes:

```
tenant_not_found
tenant_already_exists
tenant_update_not_allowed
tenant_version_conflict
task_not_found
invalid_request
```

---

# 10. Transactional Outbox Pattern

## Requirement

Task events MUST NOT be published before database commit.

---

### Transaction Flow

```
BEGIN TX

Insert Tenant
Insert Task
Insert Outbox Event

COMMIT

Publish Event
Mark Outbox Published
```

Guarantees:

```
No event on rollback
No missing task for emitted event
No emitted event without persisted record
```

---

# 11. Messaging Design

## Stream: task_outbound

Control Plane → Worker

Subject:

```
tasks.create.*
```

Payload:

```json
{
  "event_id": "<task_id>",
  "event_type": "task_created",
  "data": {
    "id": "...",
    "tenant_id": "...",
    "type": "deploy",
    "status": "accepted"
  }
}
```

---

## Stream: task_progress

Worker → Control Plane

Subject:
```
tasks.progress.*
```

Payload:

```json
{
  "event_id": "progress-event-uuid",
  "data": {
    "task_id": "...",
    "status": "in_progress"
  }
}
```

---

# 12. Idempotent Consumer

Every progress message contains:
```
event_id
```
Used as idempotency key.

### Processed Events Table
```sql
processed_events
(
    event_id TEXT PRIMARY KEY,
    processed_at DATETIME
);
```

Rules:
```
Duplicate event -> Ignore

First receipt -> Apply

Second receipt -> No-op
```

---

# 13. Order-Tolerant Processing

Consumer must never regress state.

Example:
```
done arrives
then in_progress arrives
```

Result:
```
keep done
ignore stale event
```

Examples:
```
done      -> ignore future updates
failed    -> ignore future updates
```

---

# 14. Message Reliability

## Retry Strategy

```
Exponential backoff

1s
2s
4s
8s
16s
```

For transient failures.

---

## Poison Messages

Malformed events:
```
Log error
Move to DLQ
Continue consuming
```
Consumer must never crash.

---

# 15. Worker Simulator

CLI:
```
worker-sim </span>
  --fail-rate=0.2 </span>
  --min-delay-ms=500 </span>
  --max-delay-ms=3000
```
### Behaviour

```
Receive accepted task

Publish:
1. in_progress

Then:
2. done OR failed
```

Failure probability controlled by:
```
--fail-rate
```

---

# 16. Concurrency Requirements

## Requirement 1

Two parallel PATCH requests:
```
version=3
version=3
```

Expected:
```
One succeeds
One fails with tenant_version_conflict
```

---

## Requirement 2

Two parallel tenant creates:
```
slug=my-tenant
slug=my-tenant
```

Expected:
```
One succeeds
One returns tenant_already_exists
```

---

# 17. Database Schema

### tenants
```
id
slug UNIQUE
name
status
version
created_at
updated_at
```
### tasks
```
id
tenant_id
type
status
created_at
updated_at
```
### outbox_events
```
id
aggregate_id
payload
published
created_at
```
### processed_events
```
event_id
processed_at
```

---

# 18. Testing Scope

## Unit Tests

- Tenant FSM
- Task FSM
- Validation rules
- Version checks
- Outbox service

## Integration Tests

- Create → Deploy → Active lifecycle
- Update lifecycle
- Destroy lifecycle
- Failure transitions
- Duplicate message handling
- Out-of-order message handling

## Concurrency Tests

- Parallel PATCH race
- Parallel CREATE race
- Duplicate event replay

---

# 19. Quality Gates

```
make fmt

make lint

make test

make integration-test

make ci
```

Includes:

- gofmt
- staticcheck
- go test -race
- integration suite
- optional Trivy scan

---

# 20. Acceptance Criteria

The solution is complete when:

1. `docker compose up` boots entire system.
2. Tenant creation automatically triggers provisioning.
3. Worker emits progress updates.
4. Tenant transitions to active after deploy completion.
5. Tasks are queryable via APIs.
6. State machine rules are enforced.
7. Duplicate messages are safely ignored.
8. Out-of-order messages do not regress state.
9. Concurrent PATCH requests cannot both succeed.
10. Concurrent tenant creation with same slug produces exactly one success.
11. Data survives service restart.
12. All tests pass via a single command.
13. README documents architecture, trade-offs, setup, API examples, and failure scenarios.

This revised scope now aligns very closely with the candidate brief and should be sufficient as a development blueprint for implementation.