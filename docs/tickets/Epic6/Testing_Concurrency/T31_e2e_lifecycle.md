# 🛠 GOAL: T31_e2e_lifecycle.md.md | 📊 ESTIMATE: 4 hrsh | 🪜 DEPENDS_ON: [ALL PHASES 0-3]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T31: End-to-End Lifecycle Integration Test

## Goal
Verify the full automated provisioning flow from signup to active (Revised Section 20).

## Implementation Instructions (Integration Testing via docker compose + testcontainers)
Write a single E2E integration test using Go's net/http client: Send API request -> verify provisioning state -> wait for worker to simulate processing the NATS message natively in docker-compose network -> verify DB Tenant is now `active` and Task is `done`. 

