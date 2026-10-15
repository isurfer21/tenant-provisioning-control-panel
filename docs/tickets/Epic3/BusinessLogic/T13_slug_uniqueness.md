# 🛠 GOAL: T13_slug_uniqueness.md.md | 📊 ESTIMATE: 3 hrsh | 🪜 DEPENDS_ON: [T7, T9]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T13: Slug Uniqueness Enforcement

## Goal
Prevent duplicate tenant creation globally, preserving slug even across destroyed tenants (Revised Section 5).

## Implementation Instructions (Go/Repository+DB)
1. Ensure `UNIQUE (slug)` constraint is applied on the `tenants` table in your DB schema (Ticket T3 handles this in migration).
2. Catch `uniqueViolation` errors from SQLite driver (`modernc.org/sqlite` or `mattn/go-sqlite3`) and map them to a clean repo error: `ErrSlugAlreadyExists`.
3. Validate the input slug against regex `^[a-z][-a-z0-9]{1,26}[a-z0-9]$` (must pass k8s namespace rules) prior to hitting the database.

## Acceptance Criteria
- Two parallel tenant creation requests using `POST /v1/tenants{slug: 'acme'}` return exactly one success and one `409 tenant_already_exists`. 

