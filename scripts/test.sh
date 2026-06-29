#!/usr/bin/env bash
# =============================================================================
# Skolva Test Suite — TP1 + TP2 dispatcher
#
# Usage:
#   ./scripts/test.sh              Interactive: select steps from menu
#   ./scripts/test.sh 01 04 06      Run specific steps non-interactively
#   ./scripts/test.sh --ci 04 06    CI mode: Ubuntu-compatible, skips Docker
#   ./scripts/test.sh all           Run all steps in order
# =============================================================================
set -euo pipefail

# source shared library
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/common.sh"
source "$SCRIPT_DIR/tests/tp2.sh"

# ============================================================================
# TP1-specific steps (curated from the original Phase 1 test suite)
# ============================================================================

s_tp1_unit() {
    banner "TP1 Unit Tests"

    # Core unit tests (config, types, errors, hooks, ai, jobs) moved to the
    # skolva-core repo and run in its own CI.

    step "TP1.7" "Plugins — Empty Registry"
    run_go_test "Plugin tests" ./plugins/ -v

    step "TP1.8" "Main — Health Endpoint"
    run_go_test "Health endpoint tests" ./cmd/api/ -v
}

s_tp1_integration() {
    banner "TP1 Integration Tests (requires Docker)"

    if ! docker_available; then
        echo -e "  ${YELLOW}[SKIP]${NC} Docker not available — skipping TP1 integration tests"
        return
    fi

    # Core integration tests (database, jobs worker) moved to skolva-core.

    step "TP1.11" "App — Router with real DB (health + unhealthy)"
    run_go_test "App router integration tests" ./internal/app/ -v -count=1 -run "TestNewRouter"
}

# ============================================================================
# Aggregated steps
# ============================================================================

s_all_unit()          { s_tp1_unit; tp2_unit; }
s_all_integration()   { s_tp1_integration; tp2_integration; }
s_run_all() {
    s_prerequisites; s_build; s_lint
    s_tp1_unit; s_tp1_integration
    tp2_unit; tp2_integration
    s_coverage; s_docker_compose; s_docker_build; s_makefile; s_full_check
}

# ============================================================================
# Menu
# ============================================================================

ALL_STEPS=(
    "all:Run all steps in order"
    "01:Prerequisites Check"
    "02:Build"
    "03:Lint"
    "04:All Product Unit Tests"
    "04a:Product: plugins + main health"
    "04b:Product: auth (password, JWT, permissions) + OpenAPI parity (app)"
    "05:All Product Integration Tests (needs Docker)"
    "05a:Product: app router health + unhealthy (needs Docker)"
    "05b:Product: auth + crm + groups (needs Docker)"
    "06:Full Test Suite + Coverage"
    "07:Docker Compose (up → check → down)"
    "08:Docker Image Build + Health"
    "09:Makefile Targets"
    "10:Full Check (build+lint+test+coverage)"
)

show_menu() {
    echo ""
    echo -e "${BOLD}Available Test Steps${NC}"
    echo "──────────────────────────────────────"
    for entry in "${ALL_STEPS[@]}"; do
        local key="${entry%%:*}" desc="${entry#*:}"
        printf "  %-6s %s\n" "${key}" "${desc}"
    done
    echo ""
}

run_step() {
    local step="$1"
	case "$step" in
        all)   s_run_all ;;
        help|--help) show_menu; echo "Usage: $0 [--ci] [step...]  (steps: $(echo "${ALL_STEPS[@]}" | grep -oE '[0-9a-z]+:' | tr -d ':' | tr '\n' ' '))" ;;
        01)    s_prerequisites ;;
        02)    s_build ;;
        03)    s_lint ;;
        04)    s_all_unit ;;
        04a)   s_tp1_unit ;;
        04b)   tp2_unit ;;
        05)    s_all_integration ;;
        05a)   s_tp1_integration ;;
        05b)   tp2_integration ;;
        06)    s_coverage ;;
        07)    s_docker_compose ;;
        08)    s_docker_build ;;
        09)    s_makefile ;;
        10)    s_full_check ;;
        *)     echo -e "${RED}Unknown step: $step${NC}"; return 1 ;;
    esac
}

# ============================================================================
# Main
# ============================================================================

if [[ -f "go.mod" ]]; then true
elif [[ -f "../go.mod" ]]; then cd ..
else echo -e "${RED}ERROR: Must run from project root (where go.mod is)${NC}"; exit 1; fi

# --- logging ---
mkdir -p logs
LOG_FILE="logs/test-$(date +%Y%m%d-%H%M%S).log"
exec > >(tee "$LOG_FILE") 2>&1

if [[ "$CI_MODE" == "true" ]]; then CI_LABEL=" (CI mode)"; else CI_LABEL=""; fi
echo -e "${BOLD}Skolva Test Suite (TP1 + TP2)${NC}${CI_LABEL}"
echo "Project: $(pwd)"
echo "Go:      $(go version)"

if [[ $# -eq 0 ]]; then
    show_menu
    echo -n "Which step(s)? Enter number(s) separated by space [all]: "
    read -r selection
else
    selection="$*"
fi

if [[ -z "$selection" ]]; then
    if $CI_MODE; then selection="04 06"; else selection="all"; fi
fi

echo ""; echo -e "${BOLD}Running: ${selection}${NC}"; echo ""

for s in $selection; do run_step "${s,,}" || true; done
summary

if $CI_MODE && [[ $FAILED -gt 0 ]]; then exit 1; fi
