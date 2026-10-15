# Chunk 2 — Domain Model & Validators

**Goal**: Implement `internal/model/` with struct definitions, validation logic, and helper methods for both Tenant and Task domain types. Add enum-like constants for all status values.

---

## Prompt: Build the domain model

> Create two Go source files under `internal/model/tenant.go` and `internal/model/task.go` that define the complete domain model with validation. Here is the exact specification to implement verbatim:
>
> **Tenant struct** (`model.Tenant`):
> ```go
> type Tenant struct {
>     ID         string `json:"id"`
>     Slug       string `json:"slug" db:"slug"`
>     Name       string `json:"name" db:"name"`    // Required, non-empty.
>     Status     string `json:"status" db:"status"`          // "provisioning", "active", "updating", "destroying", "destroyed", "failed"
>     Version    int64  `json:"version" db:"version"`        // optimistic lock version (starts at 1)
>     CreatedAt  string `json:"created_at" db:"created_at"`  // ISO 8601, milliseconds, "Z" suffix
>     UpdatedAt  string `json:"updated_at" db:"updated_at"`   // ISO 8601, milliseconds, "Z" suffix
> }
> ```
> **Tenant constants**: define typed strings for each tenant status enum value as package-level const:
> • `const TenantStatusProvisioning = "provisioning"`
> • `const TenantStatusActive = "active"`
> • `const TenantStatusUpdating = "updating"`
> • `const TenantStatusDestroying = "destroying"`
> • `const TenantStatusDestroyed = "destroyed"`
> • `const TenantStatusFailed = "failed"`
> • `var TenantValidStates = []string{"provisioning", "active", "updating", "destroying", "destroyed", "failed"} (for validation)
>
> **Task struct** (`model.Task`):
> ```go
> type Task struct {
>     ID         string `json:"id"`
>     Type       string `json:"type" db:"type"`                // "deploy", "update", "destroy"
>     TenantID   string `json:"tenant_id" db:"tenant_id"`    // FK-like reference to the tenant.
>     Status     string `json:"status" db:"status"`            // "accepted", "in_progress", "done", "failed".
>     CreatedAt  string `json:"created_at" db:"created_at"`   // ISO 8601, milliseconds, "Z" suffix
>     UpdatedAt  string `json:"updated_at" db:"updated_at"`    // ISO 8601, milliseconds, "Z" suffix.
> }
> ```
> **Task constants**: task type constants:
> • `const TaskTypeDeploy = "deploy"` and similar for update/destroy.
> • `const (TaskStatusAccepted = "accepted", TaskStatusInProgress = "in_progress", TaskStatusDone = "done", TaskStatusFailed = "failed")`
> - `var ValidTaskTypes = []string{"deploy", "update", "destroy"}`
> - `var TerminalTaskStatuses = map[string]bool{"done": true, "failed": true} (for fast terminal check)
> • `var NonTerminalTaskStatuses = ...` for active check
>
> **Validator methods**: add these to the types as methods on the respective structs:
> 1. `(t *Tenant) ValidateNew() error`: if Slug empty, set via auto-generate UUID prefix + random suffix (e.g. "tenant-abc123"). Then validate slug regex `^[a-z][-a-z0-9]{1,26}[a-z0-9]$` length 3-28 chars. Validate `Name != ""`. Set `Version = 1`, `CreatedAt/UpdatedAt = now().UTC()`, and `Status = TenantStatusProvisioning`. Return validation error on first failure as a typed structured Error type or just go standard error return.
> 2. `(t *Task) ValidateNew() error`: ValidateType and tenantID exist. Set ID via UUID, Created/Updated time fields.
> 3. `(t *Tenant) ValidateUpdate(newName string) error`: Same slug+name checks; if name not provided in the partial update, return nil (it's not being changed). Return a typed structured validation error on failure for client consumption (e.g., `&ValidationError{Field: "slug", Message: "..."}, Field: "name"}`
> 4. `(t *Tenant) IsValidTransition(from, to string) bool` — implements the full tenant FSM:
>   - provisioning → active | failed
>   - active → updating (via PATCH request) or destroying (via DELETE request).
>   - updating → active | failed
>   - destroying → destroyed | failed
>   - failed → destroying (via DELETE from failed state).
>   All other transitions invalid: return false.
> 5. `(t *Task) ValidateProgressUpdate(newStatus string) error` — checks legal progress updates: "accepted" → "in_progress"; "in_progress" → {"done", "failed"} only (and returns an error for illegal status value or any out-of-order or back-transition). Use the TerminalState map in validation to reject stale updates.
> 6. `IsTerminalTaskStatus(status string) bool` — checks against the terminal map above.
>
> Add a helper function package-level utility: `NowMilliUTC() string` returns `time.Now().UTC().Format("2006-01-02T15:04:05.000Z")` — note the `.000Z` for milliseconds (not the built-in Go time format).
