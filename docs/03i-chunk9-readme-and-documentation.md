# Chunk 9 — README.md & Architecture Documentation

**Goal**: Create the public-facing `README.md` with complete setup instructions, API usage examples, architecture diagram (Mermaid), design decisions, and negative scenario reproduction. Also create a simple `.gitignore` and license file.

---

## Prompt: Write the documentation

> Create the following project files at the root of the repository:

### 1. README.md with these exact sections:
```markdown
# Tenant Provisioning Control Plane

A multi-tenant provisioning control plane that orchestrates async tenant lifecycle events via HTTP API + message broker (NATS JetStream). Written in **Go** for concurrency correctness and deployment simplicity.

## Quick Start

### Prerequisites
- Go 1.22+
- Docker & Docker Compose V2

### One-command boot
```bash
git clone <repo-url> && cd tenant-provisioning-control-plane
make up   # builds + boots NATS + control-plane + worker-sim via docker compose
```

### Running tests
```bash make test   # unit + integration suites
make integration  # end-to-end against live Docker environment
## API Usage Examples

### Create a tenant (start provisioning flow)
```bash curl -X POST http://localhost:8080/v1/tenants \
  -H "Content-Type: application/json" \
  -d '{"name": "Acme Corp", "slug": "acmecorp"}'
# Returns 201 with { tenant: {... status: "provisioning"}, task: {type:"deploy", status:"accepted"} }
## Watch the lifecycle via HTTP GET /v1/tenants/{id} — it will eventually reach active as the worker processes the deploy task.

### List all tenants (with pagination and optional status filter)
```bash curl http://localhost:8080/v1/tenants?status=active&page=1&per_page=20
# Returns { items: [...], pagination: { total, next_offset } }`
 ### Update a tenant (moves to updating → awaiting worker completes update)
 ```bash curl -X PATCH http://localhost:8080/v1/tenants/{id}?version=3 \   # optimistic lock version required!
   -H "Content-Type: application/json" \
   -d '{"name": "Acme Corp Updated"}'
# Returns 200 with status updating + new update-type task created and published to broker for worker processing.

### Delete tenant (only from active or failed states)
``curl -X DELETE http://localhost:8060/v1/tenants/{id} 
# Returns 204 No Content; Tenant transitions to destroying while destroy task is processed by worker. 

## Architecture  
(Mermaid diagram here showing HTTP API layer, NATS JetStream broker streams (task_outbound and progress stream), SQLite local persistence, Worker Simulator as a side-car consumer process — arrows showing request/response flows between components)

## Design Decisions & Trade-offs
### Why Go? 1. Native goroutines provide true concurrency for worker pools without OS-level overhead versus Python's GIL limitations. 2. No try-catch or dynamic behavior — explicit error handling (`if err != nil`) keeps control flow visible and predictable under load unlike Python exception chains which obscure code paths when debugging race conditions in production at scale. 3. Go fmt + staticcheck provide instant feedback catching type mismatches, unused variables, dead code etc before runtime unlike linters like flake8+mypy which only partially catch such issues at lint time leaving logic bugs through to runtime silently in Python projects where they are harder to find post-deploy than a Go compilation error.)

### SQLite for persistence during development/CI
Chosen over PostgreSQL for simplicity — zero configuration; no setup required. Uses modernc/sqlite driver (pure-Go pure-SQLite implementation, avoiding CGo dependency). In production this would be swapped with Postgres using the same repository interface without changes to codebase logic only requiring a different Store implementation)

NATS JetStream chosen over RabbitMQ and Kafka because:
1. Simpler operational model (single binary, no ZooKeeper/manager needed; can run in Docker in seconds versus hours for Kafka setup). 

### Exactly once delivery is impossible — we design for at least-once instead. Using idempotent consumers deduplication on message event_id keys + ordering tolerance via FSM guards prevents double-application or regressions despite duplicate deliveries from NATS guarantees (at-least once, no exactly one guarantee exists anywhere in distributed systems).

## Negative Scenario Scenarios & Reproduction
### Simulate worker failure scenario by starting the worker simulator with:
```bash cd cmd/worker-simulator/ go run . --fail-rate=1.0 --min-delay-ms=200 --max-delay-ms=500  # all tasks will fail

Create a tenant then observe status transitions from provisioning → failed as expected because of simulated 100% failure rate on worker side:
curl -X POST http://localhost:8080/v1/tenants \   -d '{"name":"fail-test"}'
curl http://localhost:8060/v1/tenants/<id> # status will be "failed" not active! (because worker simulated 1 failure rate) 

### Test optimistic locking by sending two concurrent PATCH requests with the same initial version number for updating a tenant
```bash curl -X PATCH ... ?version=1 \    & // first request sends {"name":"first-update"}
curl-X PATCH...?version=1\ -d'{"name":"second-update"}' simultaneously via parallel execution or scripts (e.g use `&` in bash to run both curl commands concurrently: `curl XPATCH ... version=1-d..."first">& curl-XPATCH version= 1-d second "& wait`) Verify exactly ONE succeeds 200 and the other gets 409 conflict due to concurrent update violation)`.

## Error Codes Reference
| code | HTTP status | When it triggers | 
|---|---|------  
| tenant_not_found | 404 | GET/PATCH/DELETE by non-existent ID
tenant_already_exists | 409 | POST duplicate slug creation attempt |  

### Tenant_update_not_allowed | 422 | Invalid state transition (e.g., PATCH from non-active status) 
tenant_version_conflict | 409 | Version mismatch during concurrent updates.

## Worker Simulator CLI Flags
| Argument | Default | Description  
--fail-rate float 0.0 Probability[0-1] of failure; 1=always fail, 0=never fail
--min-delay-ms int 5 minimum delay before processing task (simulated work time) ms) 
--max-delay-ms int 3 maximum delay max processing wait up to this limit 
--broker-url string nats://localhost:4222 NATS connection address for publishing/comsuming messages |

## License  
MIT (or whatever license you want - recommend MIT for public repos).
```

### 2. Root `.gitignore` file
Include common Go exclusions, but critically exclude secrets and any database files that might get checked in accidentally:
- *.db files (SQLite data)
- .env* files (secret environment variable files that shouldn't be committed!)
- vendor/ for modules vendored locally
- .DS_Store (macOS system files)
- cmd/server/*.exe build artifacts  
- coverage.txt test output reports generated during test runs.

### 3. Root LICENSE file with MIT standard license text included — recommend this as it's permissive and common in open source projects.
