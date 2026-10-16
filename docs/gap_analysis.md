# Gap Analysis: Tenant Provisioning Control Plane

**Date:** 2026-10-16  
**Status:** ⚠️ ~65% complete — builds, boots, and has full domain/logic/test coverage. End-to-end messaging loop is **not connected**, so the async provisioning workflow doesn't function at runtime.  

---

## How This Was Compiled

Every source file, test file, migration, config, and doc was read against the requirements in:
- `Control_Plane_CANDIDATE_BRIEF.md` (the canonical spec)
- `02-scope-of-work.md` (detailed implementation plan)
- `Implementation_Tickets.md` (22+ tickets with acceptance criteria)

Results mapped to each ticket + the brief's functional/non-functional requirements.

---

## What's Complete ✅ (~65%)

These components match their acceptance criteria:

| Component | Files | Status | Notes |
|---|---|---|---|
| **Domain models** | `model/tenant.go`, `model/task.go` | ✅ | Tenant with slug/name/status/version/timestamps + validation. Task with deploy/update/destroy types, accepted/in_progress/done/failed statuses. |
| **Repositories** | `store/tenants.go`, `store/tasks.go` | ✅ | Full CRUD for both. Slug uniqueness enforced via DB UNIQUE constraint. Optimistic locking: `WHERE version=?` in tenant.Update. |
| **Outbox writes** | `outbox/outbox.go` — `CreateTenantTaskWithOutbox()` | ✅ | Writes tenant + task + outbox row in one DB tx. Commit before publish (pattern correct, loop missing — see GAP 1). |
| **Idempotency helpers** | `outbox/outbox.go` — `MarkProcessed()`, `IsProcessed()` | ✅ | `INSERT OR IGNORE` ensures same event_id applied only once. |
| **Tenant FSM** | `lifecycle/tenant_fsm.go` + tests | ✅ | All transitions: provisioning→active/failed, active↔updating, active/failed→destroying, destroying→destroyed/failed. Test file covers all valid and invalid transitions. |
| **Task FSM** | `lifecycle/task_fsm.go` + tests | ✅ | accepted→in_progress→done/failed. Terminal guards implemented. |
| **Open-task guard** | `lifecycle/open_task_guard.go` | ✅ | Blocks creation if tenant already has accepted/in_progress task. Test verifies exact-one-winner race condition. |
| **Order guard** | `lifecycle/order_guard.go` + tests | ✅ | Validates timestamp/sequence ordering, rejects terminal-state regressions. Guard exists but unused (see GAP 2). |
| **Tenant HTTP handler** | `handler/tenant_handler.go` | ⚠️ | POST/LIST/GET/PATCH all correct. DELETE is a stub that does NOT create a destroy task or transition to `destroying`. |
| **Task HTTP handler** | `handler/task_handler.go` | ⚠️ | GET-by-id and LIST OK per spec ("read-only"). But ALSO exposes direct POST which violates "tasks created only as side-effect of tenant API calls." |
| **Error framework** | `error/gin_error.go` | ✅ | Maps all codes (tenant_not_found, tenant_already_exists, tenant_update_not_allowed, tenant_version_conflict, task_not_found) with structured JSON `{error_code, error_message, request_id}`. |
| **Middleware chain** | `middleware/recovery.go`, `middleware/logging.go`, `middleware/request_id.go` + `router/router.go` | ✅ | PanicRecovery → RequestID → Logger wired at startup. Health endpoint exists. |
| **Config** | `config/config.go` | ✅ | Loads PORT, DB_PATH, NATS_URL from environment vars. |
| **Migrations** | `migrations/001_init.sql` + `migrate/migrator.go` | ✅ | Has all four tables: tenants, tasks, outbox_events, processed_events. Migrations auto-run at startup. |
| **Concurrency tests** | `store/optimistic_locking_test.go`, `store/concurrent_create_race_test.go` | ✅ | Proves optimistic lock + slug uniqueness race conditions work. |
| **E2E tests** | `lifecycle/e2e_lifecycle_test.go` | ✅ | Covers full create→deploy→active lifecycle; optimistic conflict; duplicate replay. |
| **Worker sim** | `worker-sim/main.go` | ⚠️ | Compiles (barely) — has arg parsing, NATS connection, stream+consumer setup, receives message. But doesn't publish progress updates back to the broker. |
| **Tests suite** | Makefile (`make test`, `make integration-test`) | ✅ | Wires up unit tests + race detector + fmt check. |
| **Docs** | `README.md`, `docs/failure_scenarios.md` | ✅ | Architecture diagram, setup instructions, API examples, trade-offs. Failure demo scenarios documented. |

---

## What's Broken / Missing (~35%)

Each gap maps to specific tickets and shows the exact file/line issue found during review.

### GAP 1: Outbox → NATS Publish Loop (blocks T18 + T19)

**Symptom:** Calling `POST /v1/tenants` writes tenant row, task row, and outbox event row to SQLite — but nothing publishes the event to NATS. Worker simulator never sees anything.

**What exists:**
- `CreateTenantTaskWithOutbox()` in `outbox/outbox.go` correctly writes all 3 rows in one tx (commit before publish pattern is right).
- `FetchPendingOutbox(db, limit)` returns rows where `attempts < 10`.
- `IncrementAttempts()` bumps the counter.
- A `Publisher` struct exists in... wait, **it does not**. There is no `nats/publisher.go`. The NATS publish infrastructure doesn't exist anywhere — the gap isn't just a missing loop, it's a missing entire package.

**What's needed:**
1. A new file (or function in `outbox/dispatcher.go`) that:
   - Connects to NATS JetStream
   - Creates/looksup stream (e.g., `TASKS_CREATE`)
   - Runs a ticker loop (every 2–5 seconds): fetch pending outbox rows → call `js.Publish(subject, payload)` → on success delete or mark published → on failure `IncrementAttempts()` and sleep using exponential backoff
2. Start this goroutine in `cmd/server/main.go` after JetStream context is established

**Files to modify/create:**
| File | Action |
|---|---|
| `internal/outbox/dispatcher.go` **(new)** | Dispatcher struct + ticker loop |
| `cmd/server/main.go` | Wire up NATS connection, pass JS context to dispatcher, start goroutine on startup |

### GAP 2: Inbound Consumer for Task Progress (blocks T21 + T22 + T23)

**Symptom:** Worker sim can simulate work (if we fix GAP 3), but the control-plane service has **zero code** that subscribes to `tasks.progress.*` messages. The full async loop is broken at both ends: nobody publishes out, and nobody consumes in.

**What exists (all dead code right now):**
- `IsProcessed()` / `MarkProcessed()` in `outbox/outbox.go` — correct but never called by a consumer
- `OrderGuard` struct + `Validate()` in `lifecycle/order_guard.go` — correct but unused anywhere
- Nothing subscribes to any NATS subject for inbound messages

**What's needed:**
1. A new package (e.g., `internal/consumer/consumer.go`) that:
   - Subscribes to JetStream subject `tasks.progress.*`
   - On each message: unmarshal `{task_id, status}`, validate state transition using `ValidateTaskTransition()`, check `IsProcessed()` for idempotency, skip if already processed, otherwise call `taskRepo.UpdateStatus()`, and on terminal outcomes (done/failed) update the corresponding tenant via FSM
   - Handles malformed messages gracefully (don't crash; log and ack/skip)
2. Start consumer goroutine in `cmd/server/main.go`

**Files to modify/create:**
| File | Action |
|---|---|
| `internal/consumer/consumer.go` **(new)** | Subscribe + parse + FSM validation + DB update pipeline |
| `cmd/server/main.go` | Wire up consumer; start goroutine after NATS connected |

### GAP 3: Worker Simulator Doesn't Publish Progress (blocks T20)

**What exists:**
- Parses `--fail-rate=` and `--max-delay=` args ✅
- Connects to NATS + creates stream with correct subjects (`tasks.create.>`, `tasks.progress.>`) ✅
- Creates consumer on durable `worker-sim` ✅
- Has `handleInBound()` that sleeps, randomly succeeds or fails

**What's wrong:**
1. Line 60: `handler := func(msg js.Msg) { handleInBound(context.Background(), cfg{failRate, maxDelay}, msg) }` — references type `cfg` (defined on line 75). This **does compile** because the Go compiler resolves forward references within the same package. The real issue is that `handleInBound()` does not publish progress back to NATS — it just sleeps and acks/naks without writing any message.
2. No mechanism in the worker sim to actually emit `in_progress` or `done`/`failed` updates over `tasks.progress.<taskID>` 

**What's needed:**
- Inside `handleInBound()`: after the sleep, build a publish payload with event_id (task_id) + task_id + status → call `js.Publish("tasks.progress."+taskID, jsonPayload)` before acknowledging

**Files to modify:**
| File | Action |
|---|---|
| `cmd/worker-sim/main.go` | Add publish calls inside `handleInBound()` after simulated work; add proper ack/nak logic |

### GAP 4: DELETE /v1/tenants/:id Is Stubbed (blocks T14 full lifecycle)

**What's wrong in `tenant_handler.go`:**
```go
func (h *TenantHandler) Delete(c *gin.Context) {
    id := c.Param("id")
    result, err := h.service.GetTenantByID(c.Request.Context(), id)
    if err != nil {
        handleError(c, err)
        return
    }
    _ = result // ← DEAD CODE — result is just discarded
    c.JSON(http.StatusOK, gin.H{"data": map[string]string{"status": "deleted"}})
}
```

Does **not**:
1. Validate tenant status allows deletion (must be `active` or `failed`)
2. Create a destroy task
3. Write outbox event
4. Transition tenant to `destroying`
5. Verify FSM transition is valid before allowing

**What's needed:**
- FSM guard: if status not in {active, failed} → return 422 `tenant_update_not_allowed`
- Create task row + write to outbox (same pattern as `CreateTenantTaskWithOutbox`)
- Update tenant table `SET status='destroying' WHERE id=? AND version=?`
- Return success response with new state

**Files to modify:**
| File | Action |
|---|---|
| `internal/handler/tenant_handler.go` — Delete method | Implement full lifecycle per spec |

### GAP 5: POST /v1/tasks Should Not Exist (spec violation)

**What's wrong:**
- `router/router.go` registers `t.POST("", h.Create)` which allows direct task creation via HTTP. The spec says: "Tasks are created **only** as a side effect of a tenant API call." Tasks must NOT be directly creatable from the outside.

**What's needed:**
- Remove or comment out the POST route in either `router.go` or `task_handler.go`. Keep GET-only routes (list, get-by-id) which are explicitly "read-only access to tasks."

**Files to modify:**
| File | Action |
|---|---|
| `router/router.go` | Remove `t.POST("", h.Create)` registration |

### GAP 6: Retry Policy / DLQ Logic Is Dead Code (blocks T24 when combined with GAP 1)

**What exists but never fires:**
- `RetryPolicy` struct, `DLQError`, `RetryPolicy.NextDelay()`, `RetryPolicy.ShouldRetry()` — all properly implemented in `lifecycle/retry.go`. But nothing calls them because the dispatch loop (GAP 1) that would need these doesn't exist.

**What's needed:**
- Once GAP 1 is fixed, wire `RetryPolicy.NextDelay()` into the dispatcher for retry delays and route persistent failures to `tasks.dlq` via the publish logic.

**Files to modify:**
| File | Action |
|---|---|
| Already will be covered by GAP 1 (`internal/outbox/dispatcher.go`) | Wire RetryPolicy + DLQ into dispatcher loop |

---

## Ticket Coverage Matrix

| Implementation Ticket | Status | Blocked By Gaps |
|---|---|---|
| **T18** Outbox pattern writes in tx | ✅ | — writes to DB correctly |
| **T19** NATS publisher | ❌ | GAP 1 + GAP 6 (no publish loop or Publisher pkg exists) |
| **T20** Worker sim | ⚠️ | GAP 3 (receives but doesn't publish progress back) |
| **T21** Consumer subscribes to updates | ❌ | GAP 2 (zero consumer code exists) |
| **T22** Idempotent processing | ⚠️ | GAP 2 (`MarkProcessed` helper exists but not called) |
| **T23** Order-tolerant consumer | ⚠️ | GAP 2 (`OrderGuard` exists but unused) |
| **T24** Retry/DLQ for consumer resilience | ❌ | GAP 1 + GAP 6 (retry logic never invoked anywhere) |
| **T14** Tenant API full CRUD | ⚠️ | GAP 4 (DELETE stubbed), GAP 5 (POST /tasks exists but shouldn't) |
| **T15** Task API read-only | ✅ GETs OK; needs POST removal for purity | GAP 5 |
| **T16** Error framework | ✅ | All codes covered, JSON shape correct |
| **T17** Middleware chain | ✅ | PanicRecovery + RequestID + Logger wired |

---

## Recommended Fix Order (Dependency Chain)

```
Phase 0: Verify baseline
  └── docker compose up → check NATS running + service starts + migrations applied
  
Phase 1: Outbox publish bridge (blocks nothing downstream, must go first)
  ├── internal/outbox/dispatcher.go (new — ticker loop + NATS JetStream publish)
  └── cmd/server/main.go (wire in JS context, start dispatcher goroutine)
  
Phase 2: Complete worker sim (independent fix, but needed for full e2e)
  └── cmd/worker-sim/main.go (add publish calls to tasks.progress.*)

Phase 3: Inbound consumer (blocks tenant state machine updates from workers)
  ├── internal/consumer/consumer.go (new — subscribe to tasks.progress.*)
  │     → parse payload, IsProcessed(idempotent check), OrderGuard(order validation)
  │     → UpdateStatus(task), FSM transition tenant on terminal task outcomes
  └── cmd/server/main.go (start consumer goroutine)

Phase 4: HTTP API correctness
  ├── internal/handler/tenant_handler.go — Implement Delete handler with full lifecycle
  └── router/router.go — Remove POST /v1/tasks route
```

---

## Risks & Gotchas for the Fix-Writers

1. **Start race:** NATS may not be ready when the service starts. Add a connect loop or healthcheck in `docker-compose.yml` (`depends_on: nats, condition: service_healthy`).
2. **WAL mode:** Ensure SQLite is opened with `_journal=WAL` to avoid "database is locked" under concurrent requests (especially during the optimistic locking tests).
3. **NATS JetStream API version mismatch:** The worker sim uses `js.StreamConfig{Subjects: []string{"tasks.create.>", "tasks.progress.>"}}`. The new dispatcher and consumer must use the same stream name/subjects.
4. **Worker sim publish vs control plane consume routing:** If the dispatcher publishes to the same JetStream as the worker sim's producer, there could be a loop (worker receives its own published messages). Need separate streams or careful subject scoping:
   - Dispatcher writes to `TASKS_CREATE` stream → subject `tasks.create.<tenantID>`
   - Worker sim writes to `TASKS_PROGRESS` stream → subject `tasks.progress.<taskID>`
   - Consumer subscribes only to `tasks.progress.*` — never subscribes to task create updates.
