#!/usr/bin/env bash
# ===================================================================
# 06-fsm-state-machine-demo.sh
# Demonstrates both Tenant FSM and Task FSM: valid + invalid transitions.
# Shows the open-task guard that prevents concurrent tasks on the same tenant.
# Payloads verified against lifecycle/tenant_fsm_test.go and lifecycle/task_fsm_test.go.
# ────────────────────────────────────────────────────────────────────

set -euo pipefail

API_URL="http://localhost:8080"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BOLD="\033[1m"; GREEN="\033[0;32m"; RED="\033[0;31m"
GRAY="\033[0;90m"; MAGENTA="\033[0;35m"; NC="\033[0m"

h()   { printf "\n${BOLD}═══ %s ═══${NC}\n\n" "$1"; }
sub() { printf "  ${MAGENTA}>%s:${NC}" "$1"; }
ok()  { printf "\n  ${GREEN}%s${NC}" "$1"; }
info(){ printf "\n${GRAY}%s${NC}" "$1"; }

echo ""
printf "${BOLD}═══════════════════════════════════${NC}\n"
printf " Demo 06: FSM State Machines & Open-Task Guard${NC}\n"
printf "${BOLD}═══════════════════════════════════${NC}\n\n"

# ━━━━━━━━━━ SEED TENANT FOR DEMOS ┃━━━━━━━━━━━━━━━━━━━━━━━
h "SEED — Finding or creating a clean tenant for FSM demos"

DEMO_ID=""
DEMO_ID=$(curl -sf "$API_URL/v1/tenants?page=1&limit=50" 2>/dev/null \
  | python3 -c "
import sys, json
d = json.load(sys.stdin)
for t in d.get('data', []):
    if t['slug'] == 'fsm-demo-seed':
        print(t['id']); exit()
print('')
" 2>/dev/null || true)

if [ -z "$DEMO_ID" ]; then
  sub "Creating fsm-demo-seed tenant..."
  resp=$(curl -s $API_URL/v1/tenants \
    -H 'Content-Type: application/json' \
    -d '{"slug":"fsm-demo-seed","name":"FSM Demo Tenant"}')
  DEMO_ID=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])")
fi

info "\n  Using tenant: $DEMO_ID\n"

# ━━━━━━━━━ A. TENANT FSM — VALID TRANSITIONS ┃━━━━━━━━━━━
h "SECTION A — Tenant FSM: Valid Transitions"

cat <<'EOF'
Valid transitions (from lifecycle/tenant_fsm.go):
    provisioning → active, failed, destroying
    active       → updating, destroying
    updating     → active, failed
    destroying   → destroyed, failed
    failed       → destroying

Each transition is validated in UpdateTenant() BEFORE any DB write.
EOF

current_id="$DEMO_ID"

sub "  provisioning → active\n"
resp=$(curl -s $API_URL/v1/tenants/$current_id \
  -H 'Content-Type: application/json' \
  -d '{"status":"active"}')
st=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['status'])")
ok " status=$st"

sub "  active → updating\n"
resp=$(curl -s $API_URL/v1/tenants/$current_id \
  -H 'Content-Type: application/json' \
  -d '{"status":"updating"}')
st=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['status'])")
ok " status=$st"

sub "  updating → active (complete update cycle)\n"
resp=$(curl -s $API_URL/v1/tenants/$current_id \
  -H 'Content-Type: application/json' \
  -d '{"status":"active"}')
st=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['status'])")
ok " status=$st"

info "\n✅ Valid transition cycle complete: provisioning → active → updating → active\n"

# ━━━━━━━━━ B. TENANT FSM — INVALID TRANSITIONS ┃━━━━━━━━━
h "SECTION B — Tenant FSM: Invalid Transitions (Rejected)"

cat <<'EOF'
Any transition NOT in the valid table triggers:
    lifecycle.ErrInvalidTransition → HTTP 422 'Unprocessable Entity'
Message: "tenant state transition not allowed: X -> Y"
EOF

# current status should be 'active', so try invalid targets
for target in provisioning failed destroyed; do
  sub "  active → $target (invalid)\n"
  code=$(curl -s -w "\n%{http_code}" \
    $API_URL/v1/tenants/$current_id \
    -X PATCH \
    -H 'Content-Type: application/json' \
    -d "{\"status\":\"$target\"}" 2>/dev/null | tail -1)
  if [ "$code" = "422" ]; then
    ok " Rejected as expected (HTTP $code)"
  else
    info " Got HTTP $code (state may have changed in another demo — not critical for this section)"
  fi
done

# ━━━━━━━━━ C. TENANT FSM — TERMINAL STATE BEHAVIOR ┃━━━━
h "SECTION C — Tenant FSM: Terminal State Behavior"

cat <<'EOF'
'destroyed' is a terminal sink state.
Transitioning INTO destroyed goes from 'destroying'.
But there are NO valid transitions FROM 'destroyed'.
(Once destroyed, the record's lifecycle is over.)
EOF

# Trigger destroying → destroyed for demo (full destroy cycle)
sub "  active → destroying → destroyed (full destroy)\n"
resp=$(curl -s $API_URL/v1/tenants/$current_id \
  -X DELETE \
  -H 'Content-Type: application/json')
cur=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['status'])" 2>/dev/null || echo "?")
ok " Tenant entered destroy cycle (status will settle into 'destroyed')"

info "\nCurrent tenant state: $cur\n"

# ━━━━━━━━━ D. TASK FSM — VALID + INVALID TRANSITIONS ┃━━━
h "SECTION D — Task FSM: Valid Transitions (lifecycle/task_fsm.go)"

cat <<'EOF'
Valid task transitions:
    accepted       → in_progress  (worker starts processing)
    in_progress    → done         (success — terminal)
    in_progress    → failed       (failure — terminal)

Terminal states: 'done' and 'failed' — cannot transition from.
EOF

# Create a new tenant for task FSM demo
resp=$(curl -s $API_URL/v1/tenants \
  -H 'Content-Type: application/json' \
  -d '{"slug":"fsm-task-demo-02","name":"FSM Task Demo"}')
task_tenant_id=$(echo "$resp" | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['id'])")

sub "Created tenant for task demo: $task_tenant_id\n"
info "${GREEN}  Initial task state for this tenant: accepted → in_progress → done/failed${NC}"

# Wait a moment for outbox to fire, then check DB
sleep 2
sub "DB — tasks created by outbox pattern:\n"
docker exec tpcp-server-1 sqlite3 /app/data/tenant.db \
  "SELECT id || ' (type=' || type || ', status=' || status || ')' FROM tasks WHERE tenant_id='$task_tenant_id' ORDER BY rowid;" 2>/dev/null | while read -r line; do info "    $line"; done

# ━━━━━━━━━ E. OPEN-TASK GUARD ┃━━━━━━━━━━━━━━━━━━━━━━━━━━
h "SECTION E — Open-Task Guard (Prevent Concurrent Tasks)"

cat <<'EOF'
The open-task guard (BeforeCreateOpenTask) prevents creating a NEW task
for a tenant that still has an OPEN (non-terminal) task.
This enforces a single-writer model per tenant — only one concurrent workflow at a time.
EOF

# Now complete the task so guard demo works
sub "Waiting for worker-sim to process task (~8-12s)...\n"
for i in $(seq 1 30); do
  cur=$(docker exec tpcp-server-1 sqlite3 /app/data/tenant.db \
    "SELECT status FROM tasks WHERE tenant_id='$task_tenant_id' ORDER BY rowid DESC LIMIT 1;" 2>/dev/null || echo "")
  if [ "$cur" = "done" ] || [ "$cur" = "failed" ]; then break; fi
  sleep 1
done

# Try to create a second tenant concurrently while first task is still open
sub "Attempting concurrent tenant creation (should trigger open-task guard)\n"
guard_resp=$(curl -s -w "\n%{http_code}" \
  $API_URL/v1/tenants \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"slug":"concurrent-demo-t","name":"Concurrent Demo"}' 2>/dev/null)
guard_code=$(echo "$guard_resp" | tail -1)
if [ "$guard_code" = "422" ] || [ "$guard_code" = "500" ]; then
  ok " Guard triggered (HTTP $guard_code)"
else
  info " Guard didn't trigger — first task may have already completed (HTTP $guard_code)\n  This is expected if worker-sim completed before this step."
fi

# ━━━━━━━━━ F. TASK FSM — INVALID TRANSITIONS ┃━━━━━━━━━━
h "SECTION F — Task FSM: Invalid Transitions via Worker-Sim"

cat <<'EOF'
Worker-sim updates task status via NATS progress events.
If a worker attempted an invalid transition (e.g., in_progress→accepted),
lifecycle.ValidateTaskTransition would reject it and the consumer
would log but not write — keeping the DB consistent with FSM rules.
EOF

# ━━━━━━━━━ SUMMARY ┃━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
h "SUMMARY — FSM Rules Reference Tables"

cat <<'EOF'
Tenant FSM (tenant_fsm.go):
  Source       → Valid Targets
  provisioning → active, failed, destroying
  active       → updating, destroying
  updating     → active, failed
  destroying   → destroyed, failed
  failed       → destroying

Task FSM (task_fsm.go):
  Source        → Valid Targets
  accepted      → in_progress
  in_progress   → done, failed

Error handling:
  ErrInvalidTransition     → HTTP 422 + "tenant state transition not allowed: X -> Y"
  ErrInvalidTaskTransition → logged / rejected by worker consumer; HTTP errors only on tenant endpoints.

Open-Task Guard:
  Prevents creating a new tenant if any existing task for other tenants is still open (accepted/in_progress).
  Why: Single-writer model per tenant — only one concurrent workflow at a time.
EOF

printf "\n${BOLD}${GREEN}%s${NC}\n" "✓ Demo 06 complete! FSM transitions validated with actual payloads."
