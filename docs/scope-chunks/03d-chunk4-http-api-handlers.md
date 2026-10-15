# Chunk 4 — HTTP API Handlers (RESTful CRUD for Tenants, Read-Only for Tasks)

**Goal**: Implement all REST endpoints in `internal/api/` with proper routing, middleware, error handling, and state machine integration.

---

## Prompt: Build the API handlers

> Create a complete RESTful API implementation in Go using standard library `net/http`. Write this code in files under `internal/api/`:
>
> **router.go**: Registers all routes via `http.NewServeMux()`. Path patterns are explicit — no wildcard routing. Attach middleware chain: logging, panic recovery (recover from panics and return 500 JSON), request ID generation (`ulid.MustNew(ulid.Timestamp(time.Now()), random.Reader)`), and response writer enhancement for structured error responses.
>
> **tenant_handler.go** (Tenant CRUD):
> ```go
> type TenantHandler struct {
>    store      store.TenantStore
>    lifecycle  *lifecycle.Manager
> }

// Create endpoint handler:
func (h *TenantHandler) Create(w http.ResponseWriter, r *http.Request) {
    // Parse request body for name and optional slug (use model.Tenant.ValidateNew()
    // to auto-generate slug if not provided).
    // If validation fails return 422 {"error":{"code":"invalid_request","message":"..."}}.
    // Check uniqueness: call store.GetTenantBySlug(slug) -> if found, return 409 tenant_already_exists error.
   * Call lifecycle.CreateTenant(ctx, name, slug).
     This should: validate → insert Tenant (status→"provisioning") + Task of type "deploy" into both DB tables atomically using db.Tx → call broker.ProduceEvent(task) AFTER DB commit → return 201.
   // Return status code mappings: 4xx codes as above, plus the 500 for unhandled errors.
}

// Read endpoint handler:GET /v1/tenants/{tenantID}/ -> find by ID via store.GetTenantByID(id), if not found return 404. If exists return 200 with tenant object (JSON).
func (h *TenantHandler) Get(w http.ResponseWriter, r *http.Request){...}
  
// List endpoint:GET /v1/tenants?filter=active&page=1&perPage=50  -> parse query parameters offset and limit; filter is optional status parameter. If invalid page or per_page values return bad request with error code "invalid_request". Call store.ListTenants(ctx, ...with params parsed from query). Then respond with pagination envelope: {"items":[...tenant objects], "pagination":{"total":count, "next_offset":offset+perpage OR nil if no more results}}.
func (h *TenantHandler) Update(w http.ResponseWriter, r *http.Request){

// Patch endpoint:PATCH /v1/tenants/{ID}?version=3 → partial update with the new name extracted from JSON body parsed using json.NewDecoder(r.Body).Decode into a struct or interface with optional fields like `{ "name": "..." }` and check if version is set on the query string via r.URL.Query().Get("version") as int64 (not found returns 422 invalid_version). If tenant not fetched via ID → 404. Check optimistic lock: fetch tenant from store then compare the currentVersion == Version, If they differ return 409 version_conflict error with JSON message explaining concurrency conflict. Use Store.UpdateTenantName(tenantID, newName, passedVersion) to do atomic update. Then use lifecycle.ApplyUpdateTaskTransition(NEW_NAME). This should: set Tenant status=Updating; Create new Task of type "update" -> insert into DB + broker.Publish(newTask) and return HTTP 200 with updated tenant object as JSON response (status: updating, version: prev.version+1).}

// Delete endpoint: DELETE /v1/tenants/:id → fetch by ID if not found (404). Check current status is "active" OR "failed", else return 403 with message "tenant_update_not_allowed". Use lifecycle.StartDestroy(tenantID) which sets tenant to destroying (if from failed) or activating -> destroyed state. Create task type: destroy and return HTTP 204 No Content as standard REST DELETE pattern, but the tenant's latest status via GET will reflect its new state ("destroying" is the intermediate during worker processing). }

```In all handlers: validate request input; handle errors in error responses using a consistent error structure (code, message, request_id) that matches HTTP error codes for each type of failure.
Return standard JSON response format { "tenant": {...} } or { "tasks": [...] } with proper JSON media type set Content-Type application/json headers.

task_handler.go" read-only endpoint handlers:TaskHandler struct{ store:store.TaskStore }
func (h *TaskHandler) Get(w, r){// get from store.GetTaskByID taskID, if not exists return 404 TaskError. else return 200 JSON {"task":...} }

func ListByTenant(ctx context.Context, w, r *http.Request { // parse tenantID URL parameter for tenant ID and call ListTasksByTenant(tenantID) return HTTP 200 with envelope of tasks array. } ]

All handlers must have:
- Proper content-type headers set to application/json for responses (except DELETE which is no-content)
- Request logging middleware attached at the router level for all endpoints
- Recovery middleware so that any unhandled panic in a handler returns 500 JSON rather than crashing the server process
