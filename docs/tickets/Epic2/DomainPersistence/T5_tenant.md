# 🛠 GOAL: T5_tenant.md.md | 📊 ESTIMATE: 3 hrsh | 🪜 DEPENDS_ON: [T1]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications

# Ticket T5: Tenant Domain Model

## Goal
Implement the Tenant entity with strict validators matching Revised Scope.

## Implementation Instructions (Go/SQLite/Gin)
Define struct:
```go
type Tenant struct {
    ID         string  `json:"id"`
    Slug       string  `json:"slug" validate:"required,slug"`
    Name       string  `json:"name" validate:"required,alpha"` 
    Status     string  `json:"status"`
    Version    int64   `json:"version"`
    CreatedAt  string  `json:"created_at"` // ISO8601 milliseconds Z suffix
    UpdatedAt  string  `json:"updated_at"`
}
```
Implement `Validate() error` enforcing Revised Section 5:
- Regex for slug: `^[a-z][-a-z0-9]{1,26}[a-z0-9]$`. If passed via JSON and doesn't match regex, return `invalid_request` validation logic. (Or use `github.com/go-playground/validator/v10`).
- Timestamps must be generated in Go as `time.Now().Format(time.RFC3339Nano)` but trimmed to strip nanoseconds leaving milliseconds, appending 'Z'.

## Acceptance Criteria
- Struct contains all mandatory fields.
- Validating tenant creation throws validation errors matching the required JSON contract for invalid inputs (e.g., name empty or slug regex violation).

