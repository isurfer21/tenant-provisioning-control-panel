# 🛠 GOAL: T19_nats_publisher.md.md | 📊 ESTIMATE: 3 hrsh | 🪜 DEPENDS_ON: [T4]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T19: NATS JetStream Publisher

## Goal
Publish "task_created" events matching Revised Section 11 exactly.

## Implementation Instructions (Go/NATS io)
Create `internal/broker/jetstream_publisher.go`. Connect to NATS using JetStream context. 
Implement a robust `ProduceTask(ctx, task *model.Task)` method:
- Subject: `tasks.create.*`
- Payload MUST contain strict JSON matching Revised Section 11: `{ "event_id": "<task.id>", "event_type": "task_created", data: { id, tenant_id, type, status } }`. 

## Acceptance Criteria
- Producer publishes reliably to NATS JetStream. If the broker is down when triggered by Outbox Dispatcher (T18), it MUST retry with backoff rather than dropping messages silently or crashing. 

