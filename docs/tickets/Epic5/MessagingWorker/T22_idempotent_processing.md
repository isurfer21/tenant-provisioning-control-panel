# 🛠 GOAL: T22_idempotent_processing.md.md | 📊 ESTIMATE: 4 hrsh | 🪜 DEPENDS_ON: [T3, T21]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T22: Idempotent Message Processing

## Goal
Handle duplicate deliveries safely using `processed_events` table (Revised Section 12 & 22).

## Implementation Instructions (Go/DB)
Before applying any work from a consumer inbound event, look up its `event_id` in the `processed_events` SQLite table:
```sql SELECT count(1) FROM processed_events WHERE event_id = ? ` If count > 0, acknowledge the event immediately as a no-op (skip processing). If not found, perform business logic and then INSERT INTO processed_events(event_id, now()). 
This guarantees duplicate NATS messages change absolutely nothing.

## Acceptance Criteria
- The exact same event delivered twice by NATS changes the DB state once and only once. Second delivery is silently ignored.

