# Chunk 8 — Automated Test Suite

**Goal**: Write comprehensive unit tests, integration tests, and concurrency race-condition tests that cover every state machine transition, optimistic locking behavior, idempotent message handling, and negative scenarios. All tests must run against the in-memory SQLite database (no real DB or broker required). Add test helpers to stub/mock NATS Broker using an interface-based approach.

---

## Prompt: Write the complete test suite

> Create Go test files under `internal/` and a main integration test script at `test/integration/run.sh` alongside it. Include everything needed for CI (linting + fmt + security scan via Makefile targets).

### Unit Tests — model validation (model/tenant_test.go, model/task_test.go)
Test validators exhaustively:
1. **Slug validation**: valid slug patterns (`a`, `ab`, `abc-xyz`, `test123`), invalid patterns (no lowercase start, no uppercase, too long (>28 chars), starts with number, dashes in wrong positions). Test edge lengths: minimum (3 chars), maximum (28 chars)
2. **Name validation**: non-empty passes; empty fails with correct error message
3. **Task validators**: valid task type list; invalid types rejected; tenant_id must be non-empty.
4. **State machine transitions** (`model/tenant_test.go`): use table-driven tests for all 8+ valid FSM transitions: `provisioning→active`, `provisioning→failed`, `active→updating`, `active→destroying`, `updating→active`, `updating→failed`, `destroying→destroyed`, `destroying→failed`, `failed→destroying`. All invalid transitions must return false (table-driven test all combinations systematically using a nested loop to verify every possible from→to pair).
5. **Task progress update validation** (`model/task_test.go`): valid progresses: accepted→in_progress, in_progress→done, in_progress→failed; invalid progresses: done→failed (stale), failed→done, in_progress→accepted, any→accepted (invalid direction), etc. Test with `CanAcceptProgressUpdate(currentStatus newStatus)`. Assert correct errors returned for illegal transitions and nil for valid ones.
6. **UUID generation**: auto-generated slugs are valid regex matches when not provided by user input

### Unit tests — store layer (`store/sqlite_test.go`
Write integration-style tests using the concrete SQLite implementation:
1. **Create and fetch tenant with slug, name, status provisioning, version initialized to 1.** Test that GetByID returns matching tenant; test invalid ID returns ErrNotFound error not panics or nil-safely.
2. **Uniqueness enforcement**: try creating two tenants with the same slug — first succeeds (INSERT), second fails due UNIQUE constraint violation caught by our code (check store.CreateTenant() returns an error that wraps or checks sqlite.ErrCode == "UNIQUE constraint failed"). 
3. **List with pagination/filtering** Test that passing status=active filters result; test page sizes of various lengths and offsets return correct subsets and totalCount totals are accurate. Verify empty filter defaults to no filtering (returns all tenants).
4. **Optimistic locking on updates**: call UpdateTenantStatus twice concurrently using two goroutines for the same tenant with the SAME version number — one must succeed, other fails (check RowsAffected == 0 from StoreUpdate and return error indicating version mismatch / optimistic lock conflict). Verify concurrency correctness across parallel requests simulating real world race conditions.
5. **Test task CRUD operations**: create a task, fetch by ID, update status to terminal state (done), verify updating it again with the same or different status fails due to guard clause in UpdateTaskStatus method that prevents stale/invalid states from being written to the database using `status IN ('accepted', 'in_progress')` condition in UPDATE statements.
6. **Rollback verification**: trigger a partial transaction where tenant insert succeeds but task insert fails (via test helper or mocked Store layer) — confirm no tenant was left behind in DB after rollback/commit failure

### Unit tests — state machine lifecycle (`lifecycle/manager_test.go`)
Test Manager orchestrates the complete workflow:
1. **Full happy path lifecycle** Test CreateTenant() → expect Tenant provisioning + Task deploy / accepted published; test subsequent calls that trigger updates and deletes follow correct FSM transitions per above table
2. **Update flow**: fetch active tenant then call UpdateTenant(…version=N) → succeeds if version matches, fails otherwise with proper error code and message showing current version to client (include the wrong vs right version in output so developer knows exactly what happened during debugging)
3. **Delete flow** Test DeleteTenant on an active tenant: returns Tenant destroyed + destroy task / verify final states are correct; also test from failed status → same result. Test invalid states return proper errors rather than panics or corrupting state).
4. **Terminal outcome handling** Simulate ApplyTaskProgress() receiving "done" for deploy-type task while the corresponding UpdateTenantStatus(…)"active") internally is called automatically via lifecycle flow — verify tenant final state reflects correctly what was done (done → active, failed → failed; destroyed → deployed) depending on task type
5. **Idempotent outcome application** Simulate same ApplyTaskProgress() being called twice for the same terminal status — second call must silently be no-op (same result as first but no error thrown).

### Concurrency tests (`store/sqlite_test.go` and lifecycle/manager__test.go`)
These are essential for proving correctness under real-world load:
1. **Race condition for concurrent updates**: launch 5 goroutines, each calling UpdateTenant(tenantID newName fmt.Sprintf("name-%d", i), version). All pass same initial version number (the one before concurrent starts). Exactly ONE must succeed; the others get `ErrVersionConflict`. Verify by checking success/failed returns from all goroutines post-join.
2. **Race condition for tenant creation**: launch 5 goroutines each calling CreateTenant() with identical name and slug. Only one should succeed returning a newly-created tenant object. Others return `ErrAlreadyExists` (error code or message matching "tenant_already_exists"). Count successes — assert == 1.
3. **Concurrent task progress updates**: simulate worker reporting two different "done" statuses simultaneously for the exact same task_id using parallel goroutines in lifecycle tests—confirm only first one that transitions succeeds, subsequent attempts silently return nil without modifying data (idempotent guarantee holds even under genuine concurrency).

### Broker Tests (`broker/broker_nats_test.go`)
1. Integration test using Testify or similar assert framework: Start NATS server inside a docker container during test execution, connect and create consumer/producer streams in-memory test setup using `github.com/nats-io/nats-server/v2/test`. Run Produce + Consume cycle between producer/consumer on two separate goroutines. Verify task event arrives with correct payload fields intact
2. Idempotency: Publish the exact same message payload to NATS JetSteam twice (simulating at-least-once delivery guarantee not exactly-once). Ensure handler receives both but second gets dropped silently due to dedup key match in our wrapper before calling lifecycle consumer logic. Assert only one actual lifecycle.ApplyTaskProgress() invocation happened, not two duplicates
3. Consumer resilience: Inject a simulated broker network partition (via NATS mock that returns ErrTimeout) while the consumer is attempting to process 10 messages at once. Confirm the consumer survives by catching and logging all errors individually without panicking/aborting subscription loop entirely; log errors via a custom logger interface passed into the Manager instead of failing hard.

### Integration tests (`test/integration/api_test.go` + shell scripts)
Use real Docker compose setup for these: bring up entire stack via test helpers or `docker exec` commands run before suite starts. Write Go HTTP client code to exercise full tenant lifecycle:

```bash
# Example integration scenario in a bash test script that runs curl commands verifying expected JSON responses match our contract exactly (including error codes):
curl -X POST /api/v1/tenants {"name":"acme","slug":"acme"} 201 → check provisioning + deploy task created 
# ... more tests following this pattern for the full lifecycle)

Write Go code within these integration tests using standard library net/http client or Testify suite framework. For each scenario (happy path, failure paths (duplicate slug creation, optimistic lock conflict), and negative cases), write separate test functions that:
1. Setup a tenant via POST /v1/tenants with `{name:"test"}` then verify Tenant gets returned in provisioning status along with the initial Deploy Task created in accepted state; GET /v1/tasks/{task.id} returns the newly-created task with correct type=deploy and status=accepted fields
2. Call PATCH /v1/tenants/:id?version=&{name:"new-acme" — verify it moves Tenant to updating status and creates a new Update Task in accepted state (check both tenant object AND tasks list endpoint)
3. Simulate worker completing deploy task by having `lifecycle.ApplyTaskProgress(taskID, "done")` or using the real consumer path through actual NATS JetStream publish/subscribe (both approaches valid depending on integration depth desired). Either way simulate that outcome so we can confirm the tenant transitions to the final active status from provisioning via terminal task update mechanism)
4. Delete Tenant (DELETE /v1/tenants/:id). Expect Destroying state + new Destroy Task with correct lifecycle transition mapping as documented in FSM spec above

### Coverage and Makefile target for running everything (CI integration):
```bash make test: go test -race -count=1 ./...; coverage report > coverage.txt ```
Makefile should have these exact targets for CI compatibility:
• `make lint` → `golangci-lint run ./...`  
`make fmt` → `gofmt -s -w .` and `go fmt ./` (just standard tools)
• `make unit` → Go test with race flag enabled only on unit tests not integration to catch potential concurrency bugs quickly: `go test -race -count=1 ./internal/...` 

• `make integration` → runs the integration bash script (test/integration/run.sh which starts Docker compose bring up entire stack then exercises API endpoints via curl commands for verification)
- `make ci` → fmt + lint + unit integration all in sequence; run sequentially to validate everything at once.
