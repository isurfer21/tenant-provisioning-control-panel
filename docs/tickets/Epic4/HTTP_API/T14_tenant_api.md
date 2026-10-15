# 🛠 GOAL: T14_tenant_api.md.md | 📊 ESTIMATE: 8 hrsh | 🪜 DEPENDS_ON: [T5, T9, T10, T12, T16]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T14: Tenant API Endpoints (Gin Framework)

## Goal
Build the RESTful HTTP API for tenants matching Revised Scope Section 7.

## Implementation Instructions (Go/Gin)
Implement handlers in `internal/api/tenant_handler.go`:
- `POST /v1/tenants` -> Creates provisioning tenant + deploy task, returns `201 Created`. Must return `409` if slug exists or `422` if request invalid.
- `GET /v1/tenants/{id}` -> Returns tenant or `404 tenant_not_found`.
- `PATCH /v1/tenants/{id}?version=X` -> Only allowed from `active`, creates update task. Enforces optimistic lock. Returns `409` if version mismatches.
- `DELETE /v1/tenants/{id}` -> Allowed only from `active` or `failed`, transitions to `destroying`. Returns task list updated.

All handlers integrate with the FSM logic (T10) and Open Task Guard (T12).

## Acceptance Criteria
- API endpoints perfectly match Revised Scope Section 7 HTTP mapping, including exact status codes (201, 409, 422, 404).

