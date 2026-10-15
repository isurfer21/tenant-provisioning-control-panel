# Implementation Roadmap — Tenant Provisioning Control Plane

**Language**: Go with SQLite + NATS JetStream  
**Total chunks**: 9 (each in its own doc under `/docs/`)  
**Estimated effort**: ~800-1200 lines of Go code total (excluding tests)  

---

## Chunk Index & Execution Order

| # | Document | What It Builds | Dependencies |
|---|----------|---------------|--------------|
| 1 | `03a-chunk1-project-scaffolding.md` | Go module, project tree, Docker Compose, Dockerfile, Makefile, `/healthz` endpoint | None |
| 2 | `03b-chunk2-domain-model-and-validators.md` | `model/` — Tenant & Task structs, validation, FSM transition helpers | Chunk 1 (Go module) |
| 3 | `03c-chunk3-persistence-store.md` | `store/` — repository interfaces + concrete SQLite impl | Chunks 1, 2 |
| 4 | `03d-chunk4-http-api-handlers.md` | `api/` — all REST endpoints with middleware & error handling | Chunks 1-3 |
| 5 | `03e-chunk5-state-machine-lifecycle.md` | `lifecycle/` — Manager orchestrating store + broker ops | Chunks 2, 3 |
| 6 | `03f-chunk6-message-broker.md` | `broker/` — NATS JetStream interface + impl (idempotent consumer) | Chunk 1 |
| 7 | `03g-chunk7-worker-simulator.md` | `cmd/worker-sim/main.go` — CLI worker that simulates provisioning | Chunks 1, 5 |
| 8 | `03h-chunk8-tests-integration.md` | Full test suite (unit, integration, concurrency) + Makefile targets | All above chunks |
| 9 | `03i-chunk9-readme-and-documentation.md` | README.md + .gitignore + LICENSE | All above chunks |

---

## Build Sequence

```bash
# Step 1: Scaffold project (chunk 1)
make up

# Steps 2-7: Implement core layers (chunks 2 through 7, in order as listed above)

# Step 8: Write tests (chunk 8)
make test

# Step 9: Add docs & finalize (chunk 9) # Final commit push public GitHub repo.
```

Each chunk document is designed to be a **standalone prompt** that an AI assistant or developer can execute in sequence. No prior knowledge of previous steps is required beyond "the Go project structure already exists". Each chunk's prompt explicitly states what interfaces and types it depends on from earlier chunks so there's no ambiguity about import paths or API contracts between layers.

---

## Dependency Graph Between Layers

```
cmd/server/main.go ──────────► internal/api/ ───────┐
                                                         │
internal/model/* ◄── internal/lifecycle/                │
                                                            HTTP request/response cycle
internal/store/* ◄── internal/lifecycle/ ◄── internal/api/    │
                                                                         ▼
internal/broker/nats_impl.go ◄── internal/lifecycle/        ─► external: SQLite DB / NATS JetStream cluster
cmd/worker-simulator/main.go ◄──── internal/broker/  ◄──────────┘```

---

## What's Excluded by Design (Out of Scope)

| Feature | Reason | When to Add Later|
|---------|--------|----------------- |
| PostgreSQL support | SQLite satisfies persistence requirement for local dev without setup overhead; swap later as needed via store interface abstraction in production environments (not needed for candidate evaluation).
| OpenTelemetry tracing | Nice-to-have but adds significant boilerplate; not required per assignment spec. Add metrics/tracing if time allows post-completion.
| gRPC sidecar service | The contract is RESTful HTTP only — no need to add gRPC complexity beyond the NATS messaging layer already handles async communication between services. |
| Auth middleware / JWT validation | Not mentioned in requirements; would require token management (auth server configuration) which adds scope unnecessarily for a take-home assignment focused on domain logic correctness. |

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| NATS JetStream streams don't auto-create without setup | Consumer panics at startup | Ensure broker Start() method creates streams/consumers via `JetStream.GetOrCreateStream()` calls during initialization before Subscribe() is called — handle case where stream names already exist gracefully by calling GetStreamInfo(name) first, then CreateStream only if not found yet (or use CreateOrUpdate helper). |
| In-memory SQLite persists data across test runs (state bleeds from one test into another) | Tests become flaky / order-dependent | Use a separate in-memory instance per test function: create new *sql connections on demand inside each test func; never share sqlite.DB between tests. Or use unique DB filenames per-test run instead if you must use file-based storage for larger datasets during testing scenarios that need persistence across functions. |
| Race conditions between worker goroutines updating same task concurrently | Test failures only happen sometimes (non-deterministic) | The optimistic locking guard in UpdateTaskStatus via `WHERE status IN ('accepted', 'in_progress')` clause prevents writing to terminal-state tasks — but ensure your concurrent testing chunk explicitly validates this guard handles true simultaneous writes correctly (not just sequential writes from separate goroutines which would work by chance). Use a real multi-core CPU with 10+ goroutines all trying to update same task simultaneously to trigger race conditions during execution not only sequential single core runs that might succeed by luck due to non-repeating patterns. |
| NATS messages delivered out-of-order for same task_id | Task regress state incorrectly (e.g getting "accepted" after done) handled already via FSM validation — CanAcceptProgressUpdate function blocks any invalid transitions silently ignoring them without error thrown but logging instead as this is expected behavior in distributed systems at least-once delivery models. |

---

## Final Checklist Before Public Push 

- [ ] All 9 chunk docs created under `/docs/`  
- [ ] Docker compose boot command works: `make up && sleep 3 && curl localhost:8080/healthz` returns 200 OK JSON response
- [ ] Tenant full lifecycle via API works without manual intervention — start provisioning task and watch status progress through active state after worker processes it automatically. Then update the tenant, then delete it while observing all transitions captured at each endpoint respectively with proper codes returned matching our contract (e.g not_found for deleted tenants trying to be fetched again, conflict errors when patching same version concurrently).  
- [ ] Worker simulation failure scenarios reproducible via `--fail-rate` CLI parameter documented in README examples section
- [ ] Concurrent PATCH race condition handled: send two simultaneous requests with identical version numbers — exactly one succeeds (200 OK) the other gets 409 VersionConflict error code as documented in our spec. Verify via logs printed during execution showing which succeeded vs failed respectively by examining HTTP status codes returned from both parallel curl execution results output captured in terminal console logs). 
- [ ] `make test` passes with zero failures (or expected failures only from integration tests that need Docker running; ensure Makefile has proper skip logic for those if docker isn't available or is still starting up.
- [ ] No secrets, passwords, API keys in git history at all — run `git log --oneline | wc -l` to verify clean commit history count matches expected number of commits made during development not accidentally pushed anything by mistake before finalizing repo setup with remote origin URL set correctly pointing your actual public GitHub repository location you intend to publish this project.
- [ ] README has all sections complete (Quick Start, Architecture diagram using Mermaid markdown syntax, API Usage Examples, Error Codes table, Worker Simulator CLI Flags reference table). Ensure mermaid renders properly when viewed on GitHub by testing locally or previewing the Markdown directly. Include architecture overview diagram at top-level above setup instructions so reader can quickly understand system layout before diving into code details.