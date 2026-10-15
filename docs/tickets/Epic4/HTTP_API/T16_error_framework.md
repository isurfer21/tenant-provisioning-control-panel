# 🛠 GOAL: T16_error_framework.md.md | 📊 ESTIMATE: 2 hrsh | 🪜 DEPENDS_ON: [None]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T16: Error Handling Framework & Middleware Contract

## Goal
Standardize API error responses matching Revised Scope Section 9 (Error Contract).

## Implementation Instructions (Go/Gin Middleware)
Create a global middleware in Go that wraps all HTTP errors to output exactly this JSON shape:
```json
{ "error": { "code": "...", "message": "...", "request_id": "..." } }
```
Implement custom error codes (`tenant_not_found`, `tenant_version_conflict`, `task_not_found`) that the Gin handlers return. The middleware must inject a unique request ID into the context if not present, and log it with every response or error.

## Acceptance Criteria
- Any validation, optimistic lock failure, or missing record returns the exact required JSON contract instead of raw stack traces or plain strings. 

