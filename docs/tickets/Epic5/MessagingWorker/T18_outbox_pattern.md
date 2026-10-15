# 🛠 GOAL: T18_outbox_pattern.md.md | 📊 ESTIMATE: 6 hrsh | 🪜 DEPENDS_ON: [T3, T5, T6]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T18: Transactional Outbox Pattern Implementation

## Goal
Guarantee "commit before publish" via the outbox table pattern (Revised Section 10).

## Implementation Instructions (Go/DB/Tx)
Implement `internal/service/outbox_service.go`. 
When a client creates or updates a tenant, execute strictly inside one DB Transaction:
```sql
BEGIN tx;
INSERT INTO tenants (...)
INSERT INTO tasks     (...)
INSERT INTO outbox_events (aggregate_id=payload='task_created')
COMMIT tx;
```
After successful `commit()` returns, invoke a publisher routine that reads all rows from `outbox_events` where `published = 0`, publishes them to NATS JetStream, and if successful, runs an `UPDATE outbox_events SET published=1 WHERE id=?`. Ensure publishing ONLY happens after the commit.

## Acceptance Criteria
- If `COMMIT tx` fails (e.g., DB constraint violated), no events are sent to NATS. 
- Exactly one event is emitted per task created or updated. No tasks exist in memory without a corresponding outbox record in the DB prior to publishing, and no published outbox record exists without a valid DB task/tenant.

