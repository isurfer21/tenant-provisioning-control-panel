# Chunk 5 — State Machine Lifecycle Manager

**Goal**: Implement `internal/lifecycle/tenant_fsm.go` that orchestrates tenant state transitions, task management, and atomic DB + broker operations.

---

## Prompt: Build the lifecycle manager

> Create `internal/lifecycle/manager.go` with a `Manager` struct that coordinates tenant lifecycle events end-to-end by combining store operations with broker publishing. Write all logic in Go — no external packages beyond stdlib + model/store/broker from your own codebase.

> **Struct & constructor:**
```go
type Manager struct {
    store   TenantAndTaskStore  // interface combining both
    broker  Broker              // our NATS abstraction
}
func NewManager(s TenantAndTaskStore, b Broker) *Manager
```

> Implement these six methods on `Manager`:

### 1. CreateTenant(name string, slug string) (*model.Tenant, *model.Task, error)
- **Step A**: Construct `tenant := model.NewTenantWithSlugOrGenerate(name, slug)` (call `tenant.ValidateNew()` internally). If validation fails return the validation error directly.
- **Step B**: Create a new `task := taskWithType(tenant.ID, "deploy")` and call `task.ValidateNew()`. Any errors return immediately as bad request responses.
- **Step C**: Use a database transaction (`Tx`). Insert tenant into SQLite via `s.CreateTenant(txCtx, tenant)`, then insert the task via `s.CreateTask(txCtx, task)`. Commit the transaction (if rollback occurs due to any error — don't publish).
- **Step D**: After successful commit, call `broker.PublishTask(task)` with exactly one event for this task. This is critical: broker publish must happen AFTER transaction commits. Log warning if publish fails but still return success (eventual consistency). If publish fails return no error and let metrics/alerting catch the gap rather than failing the API response.
- **Return**: (*tenant, *task, nil) on success with status provisioning + deploy task in accepted state.

### 2. UpdateTenant(tenantID string, newName string, version int64) (*model.Tenant, *model.Task, error)
- Fetch existing tenant from store by ID (must return 404 if not found).
- Validate current status is "active" via `tenant.IsValidTransition("active", "updating")` → if false return `tenant_update_not_allowed` error.
- Apply optimistic locking check: fetch the latest version from DB; compare against passed `version`. If versions differ return `tenant_version_conflict` (with current stored version in response).
- Execute `UpdateTenantName(newName, version)` within a Tx — this increments version atomically. Status becomes "updating" via a second update call or combined into the one SQL statement as: UPDATE tenants SET name=?, status='updating', version=version+1 WHERE id=? AND version=?
- Create `task := model.NewTask(tenantID, "update")` within the same or follow-up Tx to insert the task. Both statements must be in a single transaction for atomicity.
- After commit publish via broker (same pattern as #1).
- Return updated tenant and new task.

### 3. DeleteTenant(tenantID string) (*model.Tenant, *model.Task, error)
- Fetch existing tenant from store by ID. Must return 404 if not found.
- Validate: only allow DELETE when status is `"active"` or `"failed"`. If neither → `tenant_update_not_allowed` with a descriptive message (e.g., "Can only delete tenants in active or failed state, current is $currentStatus").
- Determine the target transition: from "active" to "destroying" OR from "failed" directly to "destroying". Validate via IsValidTransition() as a safety check both are valid legal transitions.
- Execute `UpdateTenantStatus("destroying")` on tenant within Tx; insert destroy task; commit Tx then broker.publish — all same transactional ordering guarantee.

### 4. ApplyTaskProgress(taskID string, newStatus string) error
- Called from the Message consumer loop when processing inbound Progress update messages from workers. This is the **inbound** path of the messaging system.
- Fetch current task state from store via `GetTaskByID(tenantID)`
- First check: call `model.CanAcceptProgressUpdate(currentTask.Status, newStatus)`. Invalid progression or invalid terminal → return a specific error type `ErrInvalidStatusTransition{...}` for this to be logged but not retried (dead letter the message).
- Validate idempotency: if current task status is already `newStatus` or a later state (e.g., getting "done" again after done), skip silently — return nil without DB update. This is our **idempotent guarantee**.
- Call `UpdateProgress(taskID, newStatus)` within a Tx. On completion call ApplyTenantStateUpdateOnTerminalTaskOutcome(task.Status) as the next step (only if the newStatus is a terminal state).

### 5. func (m *Manager) ApplyTenantStateUpdateOnTerminalTaskOutcome(prevTaskStatus string){...}
- After updating task status to done/failed call this method: Check if terminal task outcome was: "done" or "failed".
- Fetch tenant by its tenant_id. Then determine the target transition using valid FSM transitions and call `UpdateTenantStatus(target)` — use the correct mapping based on Task.Type: 
  - TaskTypeDeploy + done → active; failed → failed  
  - TaskTypeUpdate + done → active; failed → failed
  - TaskTypeDestroy + done → destroyed; failed → failing
- If tenant status already equals target state OR invalid transition detected (e.g., already "destroyed") skip silently. Idempotent guarantee on the tenant side too!

### FSM Valid Transition Logic (use this verbatim)
```text
provisioning -> active | failed  
active       -> updating | (via PATCH request) or  destroying (via DELETE request)! 
   updating  -> active | failed
   destroying-> destroyed | failed
   failed    -> destroying (only on another DELETE after already failed)
All others invalid.
```

Write the code with explicit error types for each possible failure mode: validation errors, store-related errors, broker-related errors, and FSM transition errors. Each should return its own typed Go error (`&ErrValidationMessage{...}`, `ErrStoreNotFound`, etc.) so callers can branch appropriately. Use standard Go error wrapping with `fmt.Errorf("%w", originalError)` where needed (e.g., wrapping a StoreNotFoundError as "tenant not found" with context message).
