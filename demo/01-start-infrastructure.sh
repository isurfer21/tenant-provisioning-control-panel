#!/usr/bin/env bash
# ===================================================================
# 01-start-infrastructure.sh
# Boots NATS (JetStream) + API server + worker simulator via
# docker-compose. Run this FIRST before any demo script.
# ===================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
NATS_URL="http://localhost:4222"
API_URL="http://localhost:8080"

# Color helpers
BOLD="\033[1m"; RED="\033[0;31m"; GREEN="\033[0;32m"; YELLOW="\033[0;33m"; NC="\033[0m"

print_header() {
    echo ""
    printf "${BOLD}══════════════════════════════════════════════${NC}\n"
    printf "${BOLD}%s${NC}\n" "$1"
    printf "${BOLD}══════════════════════════════════════════════${NC}\n"
    echo ""
}

# ── Step 1: clean up previous run ──────────────────────
echo -e "  ${RED}[CLEAN]${NC} Stopping any stale containers..."
cd "$PROJECT_ROOT"
docker compose down --remove-orphans 2>/dev/null || true
echo "      → Done."

# ── Step 2: build + start ──────────────────────────────
echo -e "\n  ${GREEN}[START]${NC} Building & launching NATS, API, worker-sim..."
docker compose up --build -d
echo "      → All images built. Services starting."

# ── Step 3: wait for NATS JetStream ────────────────────
echo -e "\n  ${YELLOW}[WAIT]${NC} Waiting for NATS JetStream (up to 15s)..."
for i in $(seq 1 15); do
    if docker exec tpcp-nats-1 nats info 2>/dev/null | grep -qi "jetstream"; then
        echo "      ✓ NATS ready on $NATS_URL (after ${i}s)"
        break
    fi
    sleep 1
done

# ── Step 4: wait for API server ────────────────────────
echo -e "\n  ${YELLOW}[WAIT]${NC} Waiting for API /healthz (up to 20s)..."
for i in $(seq 1 20); do
    status=$(curl -sf "$API_URL/healthz" 2>/dev/null | python3 -c "import sys,json;print(json.load(sys.stdin)['status'])" || true)
    if [ "$status" = "ok" ]; then
        echo "      ✓ API server healthy on $API_URL (after ${i}s)"
        break
    fi
    sleep 1
done

# ── Summary ────────────────────────────────────────────
print_header "Infrastructure Status"

echo -e "  ${BOLD}1) NATS JetStream:${NC}"
docker exec tpcp-nats-1 nats info 2>/dev/null | grep -E "JetStreams|JetStream memory|nats-server" | head -5 || echo ""

echo -e "\n  ${BOLD}2) API server (public /v1/status):${NC}"
curl -sf "$API_URL/v1/status" 2>/dev/null | python3 -m json.tool

echo -e "\n  ${BOLD}3) Running containers:${NC}"
docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null | head -5

echo ""
printf "${GREEN}%s${NC}\n" "✓ Infrastructure booted — proceed with demo scripts (02–07)."
echo ""
