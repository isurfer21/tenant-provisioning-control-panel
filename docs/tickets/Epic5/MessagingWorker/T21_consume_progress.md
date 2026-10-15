# 🛠 GOAL: T21_consume_progress.md.md | 📊 ESTIMATE: 5 hrsh | 🪜 DEPENDS_ON: [T7, T8, T10, T11]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T21: Inbound Progress Consumer Logic

## Goal
Consume and process worker updates, updating Tasks + Tenants accordingly (Revised Section 21).

## Implementation Instructions (Go/NATS JetStream)
Implement `internal/broker/progress_consumer.go`:
- Subscribe to NATS stream on `tasks.progress.*`.
- On receiving an update payload: Apply Task FSM transition. If task reaches a terminal state (`done` or `failed`), trigger the tenant service logic that mutates the Tenant status (e.g., deploy done -> active). Update both tables in one DB transaction per progress update.

## Acceptance Criteria
- Worker updates correctly mutate Tenant and Task states asynchronously without requiring direct API calls. 

