# 🛠 GOAL: T17_middleware.md.md | 📊 ESTIMATE: 2 hrsh | 🪜 DEPENDS_ON: [T4]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T17: Middleware Layer Implementation

## Goal
Implement cross-cutting concerns via Gin middleware hooks (Revised Section 20).

## Implementation Instructions (Go/Gin)
Write `internal/middleware/`:
1. `RequestID()`: Generates a v4 UUID for every inbound request, injects it into context so subsequent logs and errors carry the ID.
2. `Logger()`: Logrus or slog wrapper writing structured JSON access logs to stdout/stderr with method, path, status_code, and duration.
3. `PanicRecovery()`: Catch any unexpected panics in HTTP handlers (e.g., nil pointer dereference) and map them cleanly to a `{ "error": { "code": "internal_server_error", ... } }` 500 response rather than crashing the server process.

## Acceptance Criteria
- Logs are structured, request IDs persist across stack traces for troubleshooting, and the API remains resilient against unhandled panics in business logic. 

