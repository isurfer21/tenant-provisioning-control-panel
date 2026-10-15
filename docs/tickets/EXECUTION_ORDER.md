# 🧭 EXECUTION ORDER & DEPENDENCY MAP (P0-5)

This map dictates the exact sequential and parallel-safe order to implement the Tenant Provisioning Control Plane.
Run tickets strictly within each Phase before advancing to the next. 
Tickets are fully groomed with standardized `DEPENDS_ON:`, `ESTIMATE:` lines at the top of their respective docs.

---

## 🟦 PHASE 0: Foundation & Persistence (19 hrs total)
| #  | Ticket                               | Depends On | Est.  | Action                                                    |
|:---|:-------------------------------------|:-----------|:------|:----------------------------------------------------------|
| T1 | Bootstrap Go Project (Gin Framework) | None       | 4 hrs | Init module, Gin router, project tree                     |
| T2 | Docker Compose Environment           | **T1**     | 3 hrs | `docker-compose.yml` w/ NATS + SQLite volumes             |
| T3 | Database Migrations & Schema Setup   | None       | 3 hrs | Migration SQLs (tenants, tasks, outbox, processed_events) |
| T4 | Configuration Framework              | **T1**     | 2 hrs | Env loader (PORT, DB_PATH, NATS_URL, LOG_LEVEL)           |
| T5 | Tenant Domain Model                  | **T1**     | 3 hrs | Structs + validation for slug/regex/name                  |
| T6 | Task Domain Model                    | **T1**     | 2 hrs | Enums/types: deploy/update/destroy & statuses             |
| T7 | Tenant Repository (SQLite CRUD)      | **T3, T5** | 4 hrs | Create/List/Update/Delete implementations                 |
| T8 | Task Repository (SQLite CRUD)        | **T3, T6** | 3 hrs | Insert/List/Get updates via `status IN (...)` guard       |

---

## 🟨 PHASE 1: State Machines & Guards (17 hrs total)
| #   | Ticket                            | Depends On  | Est.  | Action                                                      |
|:----|:----------------------------------|:------------|:------|:------------------------------------------------------------|
| T9  | Optimistic Locking Implementation | **T7**      | 4 hrs | ` UPDATE ... WHERE version=? ` logic, RowsAffected check    |
| T10 | Tenant FSM                        | **T5**      | 5 hrs | Valid transition validation map (provisioning→active, etc.) |
| T11 | Task FSM                          | **T6**      | 2 hrs | Valid transition validation + terminal state guards         |
| T12 | Open-Task Guard                   | **T8, T11** | 3 hrs | Max 1 open task (accepted/in_progress) enforcement          |
| T13 | Slug Uniqueness Enforcement       | **T7, T9**  | 3 hrs | DB UNIQUE constraint catch + error mapping                  |

---

## 🟧 PHASE 2: HTTP API & Middleware (15 hrs total)
| #   | Ticket                          | Depends On                 | Est.  | Action                                                                                |
|:----|:--------------------------------|:---------------------------|:------|:--------------------------------------------------------------------------------------|
| T16 | Error Framework (Gin)           | None (Parallel to Phase 1) | 2 hrs | Standard JSON error contract: code/message/request_id                                 |
| T17 | Middleware Layer Implementation | **T4**                     | 2 hrs | RequestID UUID, Structured Logger, PanicRecovery hooks                                |
| T14 | Tenant API Endpoints (Gin)      | **T5, T9, T10, T12, T16**  | 8 hrs | POST/PATCH/DELETE CRUD matching Revised Spec Section 7                                |
| T15 | Task API Endpoints (Gin)        | **T8, T11, T16**           | 3 hrs | GET /v1/tasks/<built-in function id> and GET /v1/tenants/<built-in function id>/tasks |

---

## 🟥 PHASE 3: Outbox & Messaging Core (35 hrs total)
| #   | Ticket                            | Depends On           | Est.  | Action                                                                    |
|:----|:----------------------------------|:---------------------|:------|:--------------------------------------------------------------------------|
| T18 | Transactional Outbox Pattern      | **T3, T5, T6**       | 6 hrs | DB Tx: insert Tenant → Task → outbox_event → Commit → Publish             |
| T19 | NATS JetStream Publisher          | **T4**               | 3 hrs | Produce `task_created` events to `tasks.create.*` stream                  |
| T20 | Worker Simulator Process (CLI)    | **T4, T6**           | 5 hrs | CLI: `--fail-rate`, delay range, consume/progess updates loop             |
| T21 | Inbound Progress Consumer Logic   | **T7, T8, T10, T11** | 5 hrs | Consume `tasks progress.*` events → update task state + tenant FSM        |
| T22 | Idempotent Message Processing     | **T3, T21**          | 4 hrs | Check processed_events table (via event_id) before every write            |
| T23 | Order-Tolerant Processing Guard   | **T22**              | 3 hrs | Reject stale updates (done → in_progress must fail silently via FSM lock) |
| T24 | Retry Mechanism & Poison Handling | **T19, T21**         | 4 hrs | Exponential backoff + DLQ stream for malformed payloads                   |

---

## 🟪 PHASE 4: Testing & Concurrency (20 hrs total)
| #   | Ticket                                | Depends On     | Est.  | Action                                                          |
|:----|:--------------------------------------|:---------------|:------|:----------------------------------------------------------------|
| T25 | Tenant FSM Unit Tests                 | **T10**        | 3 hrs | Table-driven tests for all valid/invalid tenant transitions     |
| T26 | Task FSM Unit Tests                   | **T11**        | 2 hrs | Verify terminal protections on task statuses                    |
| T27 | Optimistic Locking Concurrency Test   | **T9, T14**    | 3 hrs | Parallel PATCH with same version → 1 success, 1 conflict        |
| T28 | Concurrent Create Race Test           | **T7, T13**    | 3 hrs | Parallel POST `slug=a` -> exactly one 201 success               |
| T29 | Duplicate Event Replay Test           | **T22**        | 2 hrs | Deliver same event twice; verify zero secondary mutations       |
| T30 | Out-of-Order Delivery Test            | **T23**        | 4 hrs | Send `done` then `in_progress`; verify state locks              |
| T31 | End-to-End Lifecycle Integration Test | **ALL PHASES** | 4 hrs | Full flow via HTTP client → NATS worker → DB state verification |

---

## 🟩 PHASE 5: Documentation & Demo Prep (6 hrs total)
| #   | Ticket                           | Depends On      | Est.  | Action                                                       |
|:----|:---------------------------------|:----------------|:------|:-------------------------------------------------------------|
| T32 | README Architecture & Setup Docs | **All Tickets** | 4 hrs | Mermaid diagram, quick start guide, API examples             |
| T33 | Failure Scenario Documentation   | **Full System** | 2 hrs | Guide for `--fail-rate=1.0`, concurrent PATCH conflict demos |

