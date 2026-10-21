#!/usr/bin/env bash
# ===================================================================
# demo-all.sh  —  One-shot orchestrator: runs scripts 02–07 in order.
# Prerequisite: run 01-start-infrastructure.sh once first.
# ===================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
API_URL="http://localhost:8080"

BOLD="\033[1m"; GREEN="\033[0;32m"; RED="\033[0;31m"; CYAN="\033[0;36m"; NC="\033[0m"

# Quick health check
if ! curl -sf "$API_URL/healthz" >/dev/null 2>&1; then
    echo -e "${RED}✗ API server not reachable. Run 01-start-infrastructure.sh first.${NC}"
    exit 1
fi

echo ""
printf "${BOLD}%s${NC}\n" "═══════════════════════════════"
printf "${BOLD} Full Demo Suite${NC}"
printf "${BOLD}%s${NC}\n\n"   "═══════════════════════════════"

for script in "$SCRIPT_DIR"/0{2,3,4,5,6,7}-*.sh; do
    base="$(basename "$script")"
    echo -e "\n  ${CYAN}▶${NC} Running: $base"
    echo "  ──────────────────────────────"
    bash "$script"
    echo ""
done

printf "${GREEN}%s${NC}\n\n" "✓ All demos complete."
