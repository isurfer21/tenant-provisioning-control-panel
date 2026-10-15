# 🛠 GOAL: T33_failure_scenarios.md.md | 📊 ESTIMATE: 2 hrsh | 🪜 DEPENDS_ON: [Full System Artifacts]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T33: Failure Scenario Documentation & Demo Prep

## Goal
Create the `failure_scenarios.md` documenting how to reproduce negative test cases for interview/demo purposes (Revised Scope Epic 7).

## Implementation Instructions
- Document exact CLI commands using `--fail-rate=1.0` to demonstrate graceful failure handling by the worker simulator without system crashes.
- Show curl or Postman examples demonstrating the duplicate message logic and concurrent PATCH `tenant_version_conflict` errors.
- Include steps on how to inspect logs of the NATS JetStream DLQ (Dead Letter Queue) if a poison payload is generated during demo.

## Acceptance Criteria
- Documentation explicitly proves system resilience against invalid inputs, network faults, and state regressions as defined in the Revised Scope.

