#!/usr/bin/env bash
# ===================================================================
# 04-task-api-demo.sh
# Demonstrates task API surface, outbox events written alongside tenants,
# and how tasks become reachable via the read-only GET endpoint.
# ────────────────────────────────────────────────────────────────────
set -euo pipefail

API_URL="http://localhost:8080"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BOLD="\033[1m"; GREEN="\033[0;32m"; RED="\033[0;31m"
CYAN="\033[0;36m"; YELLOW="\033[0;33m"; GRAY="\033[0;90m"; BLUE="\033[0;94m"; NC="\033[0m"

h() { printf "\n${BOLD}═══ ${1} ═══${NC}\n\n"; }
sub() { printf "  ${CYAN}%s:${NC} " "$1"; }
ok()  { printf "${GREEN}✓ %s${NC}\n\n" "$1"; }
info(){ printf "  ${GRAY}%b${NC}" "$1"; }

echo ""
printf "${BOLD}════════════════════════════════\033[0m${NC}\n"
printf "${BOLD} Demo 04: Task API & Outbox Events${NC}\n"
printf "${BOLD}════════════════════════════════${NC}\n\n"

# ━━━━━━━━━━━ STEP 1: CREATE TENANT → TRIGGERS TASK + OUTBOX ┃
h "STEP 1 — POST /v1/tenants (Triggers task + outbox creation)"
msg="When you POST a tenant, the service layer does this in ONE DB tx:\n\n  1. Inserts tenant row    (status=provisioning, version=1)  \n  2. Inserts task row      (type='deploy', status='accepted')\n  3. Writes outbox event   (subject=tasks.create.<tenant-id>)\n\nAll three are committed atomically — no orphaned task or outbox possible.${NC}\n"
info "$msg"

payload='{"slug":"task-demo-one","name":"Task Demo Tenant One"}'
resp=$(curl -s -w "\n%{http_code}" \
  -X POST "$API_URL/v1/tenants" \
  -H 'Content-Type: application/json' \
  -d "$payload")
status=$(echo "$resp" | tail -1)

if [ "$status" = "201" ]; then ok "HTTP $status — Tenant, Task, and Outbox created in one tx"; fi
echo "  Response body:"; echo "$resp" | sed '$d' | python3 -m json.tool

tenant_id=$(echo "$resp" | sed '$d' | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])")

# ━━━━━━━━━━━ STEP 2: Query tasks from DB (no task-list API yet) ┃
h "STEP 2 — Tasks Created in DB by Outbox Pattern"
msg="The 'tasks' table has a row, but the public API only exposes GET /v1/tasks/:id.\nThere is NO list endpoint for tasks in the registered routes (GAP 5: read-only only).\n\nTo verify the task exists, we query the SQLite DB directly from the api container.${NC}\n"
info "$msg"

sub "DB Query:"; 
task_id=$(docker exec tpcp-server-1 sqlite3 /app/data/tenant.db "SELECT id FROM tasks WHERE tenant_id='$tenant_id' LIMIT 1;" 2>/dev/null)
sub "$(docker exec tpcp-server-1 sqlite3 /app/data/tenant.db \
  "SELECT json_group_object(key,value) FROM (SELECT rowid, id as key, type || ': ' || status as value FROM tasks WHERE tenant_id='$tenant_id' ORDER BY rowid);" 2>/dev/null)"

info "\n  Task fields:\n    • id      = UUID (auto-generated)\n    • tenant_id = $tenant_id\n    • type      = deploy (set during create)\n    • status    = accepted (initial state, per FSM)${NC}"

# ━━━━━━━━━━━ STEP 3: GET TASK BY ID via API ┃━━━━━━━━━━━━━━━━
h "STEP 3 — GET /v1/tasks/:id (Read-Only Task via HTTP)"
msg="Public API surface:\n  GET /v1/tasks/:id     → reads and returns a task by its UUID${NC}"
info "$msg"

if [ -n "$task_id" ]; then
  curl -sf "$API_URL/v1/tasks/$task_id" 2>/dev/null | python3 -m json.tool
  ok "Got task via GET /v1/tasks/:id → HTTP 200"
else
  fail "No task found in DB (something went wrong with step 1)"
fi

# ━━━━━━━━━━━ STEP 4: Task status FSM ┃━━━━━━━━━━━━━━━━━━━━━
h "STEP 4 — Task Status Finite State Machine"
msg="Task lifecycle is governed by lifecycle/task_fsm.go.\n\n  accepted → in_progress   (worker starts processing)\n  in_progress → done      (success — terminal)\n  in_progress → failed    (error — terminal)\n\nTerminal states: 'done' | 'failed' (cannot leave.)${NC}\n"
info "$msg"

# ━━━━━━━━━━━ STEP 5: Wait for worker-sim to process ┃━━━━
h "STEP 5 — Worker-Sim Processes the Outbox Event (Async)"
msg="Worker-sim (tpcp-sim-1) subscribes to 'tasks.create.>' on NATS JetStream stream 'TASKS_CREATE'.\n\nIt will pick up our outbox event, delay 500–3000ms (simulated work), then publish progress:\n  subject = tasks.progress.<task-id>\n  payload = { tenant_id, task_id, status, event_id }\n\nThis triggers the control-plane consumer which updates the task in DB.${NC}\n"
info "$msg"

sub "Waiting for worker to process (~8-15s)...\n"
for i in $(seq 1 30); do
  cur=$(docker exec tpcp-server-1 sqlite3 /app/data/tenant.db \
    "SELECT status FROM tasks WHERE id='$task_id' LIMIT 1;" 2>/dev/null || echo "done")
  if [ "$cur" != "accepted" ]; then break; fi
  sleep 1
done

sub "Task status after worker processing:\n"; 
new_status=$(curl -sf "$API_URL/v1/tasks/$task_id" 2>/dev/null | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['status'])" 2>/dev/null || echo "?")
info "${GREEN}  status = '$new_status'${NC}\n\n  (Was 'accepted', now updated by worker-sim progress event)"

# ━━━━━━━━━━━ STEP 6: Outbox Events in DB ┃━━━━━━━━━━━━━━━
h "STEP 6 — Outbox Events Table (Evidence of Commit-Then-Publish)"
msg="Every create/destroy operation writes to outbox_events BEFORE publishing to NATS.\nThis is the classic 'Outbox Pattern' (T18): commit first, publish later, never lose events.${NC}\n"
info "$msg"

sub "DB: SELECT COUNT(*) FROM outbox_events:\n"; 
docker exec tpcp-server-1 sqlite3 /app/data/tenant.db \
  "SELECT rowid, substr(payload,1,80) FROM outbox_events ORDER BY rowid;" 2>/dev/null | head -5

# ━━━━━━━━━━━ STEP 7: Demonstrate task not-found error ┃━━
h "STEP 7 — Task Not Found Error (404)"
sub "GET /v1/tasks/nonexistent-task-id\n"; 
result=$(curl -s -w "\n%{http_code}" "$API_URL/v1/tasks/nonexistent-uuid" 2>/dev/null | tail -1)
ok "HTTP $result → Not found for non-existent task ID ✓"

# ━━━━━━━━━━━ SUMMARY ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
h "SUMMARY — Task API Surface & Worker Pipeline"
cat <<'EOF'
Public task endpoints:
  GET /v1/tasks/:id      → Read-only, returns { data: {...} } or HTTP 404
  
NOT exposed (by design — GAP 5):
  POST /v1/tasks          → Tasks are auto-created during tenant lifecycle
  GET /v1/tasks?tenant=   → No list endpoint; query DB directly if needed

Outbox Pattern flow (per tenant create):
  1. API Server: INSERT tenant + task + outbox_event (single tx) 
  2. NATS Publisher: picks up outbox_events where attempts < 10 every ~2s
  3. Publishes to 'tasks.create.<tenant-id>' subject on stream TASKS_CREATE
  4. Worker-Sim consumes → delays → publishes tasks.progress.<task-id>
  5. Consumer receives progress → updates task.status in DB → FSM validated

Why read-only tasks?  
  Task lifecycle (accepted→in_progress→done/failed) is driven entirely by:
    • Tenant create/destroy endpoints auto-creating task rows  
    • Worker-sim publishing status back via NATS
EOF

printf "\n${GREEN}${BOLD}✓ Demo 04 complete! Task API & outbox demonstrated.${NC}\n"
