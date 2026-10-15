# 🛠 GOAL: T24_retry_poison_handling.md.md | 📊 ESTIMATE: 4 hrsh | 🪜 DEPENDS_ON: [T19, T21]

---
## 🔗 Execution Context
- Run this prompt **strictly after** its dependencies finish and commit. 
- Execute step 1 in full before moving to step 2. 

---
## ✍️ Implementation Goal (Revised Blueprint)
> You are an expert Go engineer implementing a multi-tenant SaaS control plane with Gin, SQLite, NATS JetStream, and event-driven architecture. Build the EXACT logic defined below using standard libraries unless specified otherwise. Do not invent requirements; stick strictly to Revised Scope Sections 3, 4, 5, and 6.

---
## ⚙️ Directives & Specifications
# Ticket T24: Retry Mechanism & Poison Message Handling

## Goal
Make the consumer resilient against broker faults and malformed payloads (Revised Section 14 & 24).

## Implementation Instructions (Go/Consumer)
- **Exponential Backoff**: If publishing an event fails due to a broker/network transient error, log with backoff delays (1s, 2s, 4s, 8s...). 
- **Poison Messages**: If a message payload cannot be decoded as valid JSON or has invalid schema (`event_id` missing), DO NOT panic. Log the malformed event, move it explicitly to an internal "Dead Letter Queue" stream via `nats.Publish("dead_letter", bad_payload)`, and mark/ACK it so the consumer loop doesn't starve on the same message repeatedly forever.

## Acceptance Criteria
- Malformed payloads never crash the consumer process. 
- Transient NATS issues are handled gracefully rather than aborting all processing immediately.

