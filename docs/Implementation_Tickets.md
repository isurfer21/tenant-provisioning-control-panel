This assignment is large enough that I would structure it into **5 epics, 22 implementation tickets, and 7 testing tickets**. This sequence also allows you to build and demo incrementally.

# Epic 1: Foundation & Infrastructure

## T1. Bootstrap Go Project

### Goal

Create repository structure and build system.

### Acceptance Criteria

- Go module initialized
- Folder structure created
- Makefile added
- Dockerfile.api added
- Service starts successfully

### Estimate

4 hrs

---

## T2. Docker Compose Environment

### Goal

Boot all dependencies with one command.

### Acceptance Criteria

- SQLite volume configured
- NATS JetStream configured
- API service starts
- Worker service starts
- `docker compose up` brings up complete ecosystem

### Estimate

3 hrs

---

## T3. Database Migrations

### Goal

Create persistent schema.

### Deliverables

```
tenants
tasks
outbox_events
processed_events
```

### Acceptance Criteria

- Migrations run automatically
- Restart preserves data

### Estimate

3 hrs

---

## T4. Configuration Framework

### Goal

Support configuration through environment variables.

### Acceptance Criteria

```
PORT
DB_PATH
NATS_URL
LOG_LEVEL
```

supported.

### Estimate

2 hrs

---

# Epic 2: Domain & Persistence Layer

---

## T5. Tenant Domain Model

### Goal

Implement Tenant entity.

### Acceptance Criteria

Fields:

```
id
slug
name
status
version
timestamps
```

Validation:

```
slug regex
non-empty name
```

### Estimate

3 hrs

---

## T6. Task Domain Model

### Goal

Implement Task entity.

### Acceptance Criteria

Task types:

```
deploy
update
destroy
```

Task statuses:

```
accepted
in_progress
done
failed
```

### Estimate

2 hrs

---

## T7. Tenant Repository

### Goal

Persistence CRUD operations.

### Acceptance Criteria

Supported:

```
Create
Get
List
Update
Delete
```

### Estimate

4 hrs

---

## T8. Task Repository

### Goal

Store and retrieve task data.

### Acceptance Criteria

Supported:

```
Insert task
Get task
List tasks
Update task status
```

### Estimate

3 hrs

---

## T9. Optimistic Locking

### Goal

Implement version-based updates.

### Acceptance Criteria

```
UPDATE ... WHERE version=?
```

used consistently.

Only one concurrent update succeeds.

### Estimate

4 hrs

---

# Epic 3: State Machine & Business Logic

---

## T10. Tenant State Machine

### Goal

Implement all legal tenant transitions.

### Acceptance Criteria

Supported transitions:

```
provisioning -> active
provisioning -> failed

active -> updating
updating -> active
updating -> failed

active -> destroying
failed -> destroying

destroying -> destroyed
destroying -> failed
```

Invalid transitions rejected.

### Estimate

5 hrs

---

## T11. Task State Machine

### Goal

Implement task lifecycle validation.

### Acceptance Criteria

```
accepted -> in_progress
in_progress -> done
in_progress -> failed
```

Terminal states protected.

### Estimate

2 hrs

---

## T12. Open Task Guard

### Goal

Enforce single open task rule.

### Acceptance Criteria

Tenant cannot have:

```
accepted
```
or
```
in_progress
```

task already active.

### Estimate

3 hrs

---

## T13. Slug Uniqueness Enforcement

### Goal

Prevent duplicate tenant creation.

### Acceptance Criteria

- Unique DB constraint
- Destroyed tenants still reserve slug
- Concurrent creates produce exactly one winner

### Estimate

3 hrs

---

# Epic 4: HTTP API

---

## T14. Tenant API

### Goal

Implement Tenant CRUD.

### Endpoints

```
POST   /v1/tenants
GET    /v1/tenants
GET    /v1/tenants/{id}
PATCH  /v1/tenants/{id}
DELETE /v1/tenants/{id}
```

### Acceptance Criteria

All state machine rules enforced.

### Estimate

8 hrs

---

## T15. Task API

### Goal

Read-only task tracking.

### Endpoints

```
GET /v1/tasks/{id}
GET /v1/tenants/{tenantId}/tasks
```

### Estimate

3 hrs

---

## T16. Error Handling Framework

### Goal

Standardize all API responses.

### Acceptance Criteria

Errors return:

```json
{
  "error": {
    "code": "...",
    "message": "...",
    "request_id": "..."
  }
}
```

### Estimate

2 hrs

---

## T17. Middleware Layer

### Goal

Cross-cutting concerns.

### Acceptance Criteria

Implemented:

```
Request ID
Logging
Recovery
```

### Estimate

2 hrs

---

# Epic 5: Messaging & Worker Processing

---

## T18. Outbox Pattern

### Goal

Guarantee "commit before publish".

### Acceptance Criteria

Single transaction:

```
Tenant
Task
Outbox Event
```

saved together.

Publishing occurs only after commit.

### Estimate

6 hrs

---

## T19. NATS Producer

### Goal

Publish task-created events.

### Acceptance Criteria

Event contains:

```
task_id
tenant_id
status
type
```

### Estimate

3 hrs

---

## T20. Worker Simulator

### Goal

Simulate provisioning lifecycle.

### Acceptance Criteria

Supports flags:

```
--fail-rate
--min-delay-ms
--max-delay-ms
```

Publishes:

```
in_progress
done/failed
```

### Estimate

5 hrs

---

## T21. Progress Consumer

### Goal

Consume worker updates.

### Acceptance Criteria

Updates:

```
task state
tenant state
```

appropriately.

### Estimate

5 hrs

---

## T22. Idempotent Message Processing

### Goal

Handle duplicate deliveries.

### Acceptance Criteria

Uses:

```
processed_events
```

table.

Repeated message is ignored.

### Estimate

4 hrs

---

## T23. Order-Tolerant Processing

### Goal

Prevent state regression.

### Acceptance Criteria

Example:

```
DONE
```

then

```
IN_PROGRESS
```

results in:

```
DONE
```

### Estimate

3 hrs

---

## T24. Retry & Poison Message Handling

### Goal

Make consumer resilient.

### Acceptance Criteria

- Exponential backoff
- Dead-letter stream
- Consumer never crashes from bad payload

### Estimate

4 hrs

---

# Epic 6: Testing

---

## T25. Tenant FSM Unit Tests

Test all valid and invalid transitions.

### Estimate

3 hrs

---

## T26. Task FSM Unit Tests

Test task lifecycle transitions.

### Estimate

2 hrs

---

## T27. Optimistic Locking Concurrency Test

### Scenario

```
PATCH(version=3)
PATCH(version=3)
```

Expected:

```
1 success
1 conflict
```

### Estimate

3 hrs

---

## T28. Concurrent Create Race Test

### Scenario

```
POST slug=a
POST slug=a
```

Expected:

```
1 success
1 duplicate
```

### Estimate

3 hrs

---

## T29. Duplicate Event Replay Test

### Scenario

Same message delivered twice.

### Expected

No duplicate processing.

### Estimate

2 hrs

---

## T30. Out-of-Order Delivery Test

### Scenario

```
DONE
IN_PROGRESS
```

### Expected

No state regression.

### Estimate

2 hrs

---

## T31. End-to-End Lifecycle Test

### Scenario

```
Create Tenant
↓
Deploy Task
↓
Worker Processes
↓
Tenant Active
```

### Expected

Lifecycle completes without intervention.

### Estimate

4 hrs

---

# Epic 7: Documentation & Demo

## T32. README

### Contents

- Architecture diagram
- Setup instructions
- API examples
- Design decisions
- Trade-offs

### Estimate

4 hrs

---

## T33. Failure Scenario Documentation

### Include

```
--fail-rate=1
```

demo.

Duplicate message demo.

Out-of-order message demo.

### Estimate

2 hrs

---

# Recommended Execution Order

```
P0
├── T1
├── T2
├── T3
├── T5
├── T6
├── T7
├── T8

P1
├── T10
├── T11
├── T12
├── T13
├── T14
├── T15
├── T16

P2
├── T18
├── T19
├── T20
├── T21
├── T22
├── T23
├── T24

P3
├── T25-T31
├── T32
└── T33
```

For interview delivery, I'd focus on completing **T1-T24 first**, because they directly satisfy the functional and non-functional requirements. Testing and documentation can then be completed as the final hardening phase.