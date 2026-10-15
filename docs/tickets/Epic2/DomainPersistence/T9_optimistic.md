# 🛠 GOAL: T9_optimistic.md | 📊 ESTIMATE: 4h | 🪜 DEPENDS_ON: [T7_repository]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T9: Optimistic Locking Implementation

## Goal
Strictly implement version-based concurrency protection for tenant updates (Revised Section 16). 

## Implementation Instructions (Go/DB)
This logic must live inside `store/tenant_repository.go Update()` method alongside the SQL update statement. 
Query: `UPDATE tenants SET version = version + 1 WHERE id = ? AND version = ?`
When processing the HTTP PATCH request, extract `version=X` from QueryParams or JSON body (per spec). Fetch existing tenant, check DB against this requested version using `WHERE version = X`. 

## Acceptance Criteria
- If two parallel requests pass `version=3`, the first succeeds. The second attempt executing the UPDATE statement will return `RowsAffected == 0`. Your implementation must catch this `0 rows` result and map it to an error mapped exactly to the `tenant_version_conflict` standard JSON contract in Revised Section 19.
