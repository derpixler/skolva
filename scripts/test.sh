#!/usr/bin/env bash
# =============================================================================
# TP1 Test Suite — Walking Skeleton + CI/CD
# Runs all tests described in docs/tp1_testanleitung.md
#
# Usage:
#   ./scripts/test.sh              Interactive: select steps from menu
#   ./scripts/test.sh 01 02 04      Run specific steps non-interactively
#   ./scripts/test.sh --ci 04 06    CI mode: Ubuntu-compatible, skips Docker steps
# =============================================================================
set -euo pipefail

# --- CI Mode Detection ---
CI_MODE=false
if [[ "${1:-}" == "--ci" ]]; then
    CI_MODE=true
    shift
fi

# --- Colors ---
if $CI_MODE; then
    RED=''; GREEN=''; YELLOW=''; BLUE=''; BOLD=''; NC=''
else
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    BOLD='\033[1m'
    NC='\033[0m'
fi

# --- Go PATH detection ---
detect_go_path() {
    if $CI_MODE; then
        # CI: use go from PATH (setup-go action sets this)
        if command -v go &>/dev/null; then
            dirname "$(command -v go)"
            return
        fi
    fi
    for candidate in \
        "/opt/homebrew/opt/go@1.23/bin" \
        "/opt/homebrew/bin" \
        "/usr/local/go/bin" \
        "$HOME/go/bin" \
        "$HOME/sdk/go1.2"*"/bin"; do
        if [[ -x "$candidate/go" ]]; then
            echo "$candidate"
            return
        fi
    done
    # fallback: use whatever go is in PATH
    if command -v go &>/dev/null; then
        dirname "$(command -v go)"
        return
    fi
    echo ""
}

GO_BIN_DIR="$(detect_go_path)"
if [[ -z "$GO_BIN_DIR" ]]; then
    echo -e "${RED}ERROR: Go not found.${NC}"
    exit 1
fi
export PATH="$GO_BIN_DIR:$HOME/go/bin:$PATH"

# --- State ---
PASSED=0
FAILED=0

# --- Docker check (used by CI-safe skip) ---
docker_available() {
    docker info &>/dev/null 2>&1
}

# --- Tool checks ---
check_tool() {
    local name="$1" cmd="$2" hint="$3"
    if command -v "$cmd" &>/dev/null; then
        echo -e "  ${GREEN}[OK]${NC} $name"
        return 0
    else
        echo -e "  ${RED}[MISSING]${NC} $name — $hint"
        return 1
    fi
}

check_docker() {
    if docker info &>/dev/null 2>&1; then
        echo -e "  ${GREEN}[OK]${NC} Docker (running)"
        return 0
    else
        echo -e "  ${YELLOW}[WARN]${NC} Docker not running — integration tests + compose will be skipped"
        return 1
    fi
}

# --- Helpers ---
banner() {
    echo ""
    echo -e "${BLUE}============================================${NC}"
    echo -e "${BOLD}  $1${NC}"
    echo -e "${BLUE}============================================${NC}"
}

step() {
    local num="$1" desc="$2"
    echo ""
    echo -e "${YELLOW}[${num}]${NC} ${BOLD}${desc}${NC}"
}

run_cmd() {
    local desc="$1"
    shift
    echo -e "  ${BLUE}\$${NC} $*"
    if "$@"; then
        echo -e "  ${GREEN}[PASS]${NC} ${desc}"
        PASSED=$((PASSED + 1))
        return 0
    else
        echo -e "  ${RED}[FAIL]${NC} ${desc}"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

run_go_test() {
    local desc="$1" pkg="$2"
    shift 2
    echo -e "  ${BLUE}\$${NC} go test $* ${pkg}"
    if go test -count=1 "$@" "$pkg" 2>&1 | sed 's/^/  /'; then
        echo -e "  ${GREEN}[PASS]${NC} ${desc}"
        PASSED=$((PASSED + 1))
        return 0
    else
        local rc=$?
        # go test exits 1 on test failure, 0 otherwise. just report.
        echo -e "  ${RED}[FAIL]${NC} ${desc} (exit code $rc)"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

summary() {
    echo ""
    echo -e "${BLUE}============================================${NC}"
    local total=$((PASSED + FAILED))
    echo -e "  Results: ${GREEN}${PASSED} passed${NC}, ${RED}${FAILED} failed${NC}, ${total} total"
    if [[ $FAILED -eq 0 ]]; then
        echo -e "  ${GREEN}${BOLD}All checks passed.${NC}"
    else
        echo -e "  ${RED}${BOLD}${FAILED} check(s) failed.${NC}"
    fi
    echo -e "${BLUE}============================================${NC}"
}

# ============================================================================
# Test Steps
# ============================================================================

s01_prerequisites() {
    banner "1. Prerequisites Check"
    echo ""
    echo "  Go path: $GO_BIN_DIR/go ($(go version))"
    if $CI_MODE; then
        echo -e "  ${GREEN}[OK]${NC} golangci-lint (separate CI job)"
        echo -e "  ${GREEN}[OK]${NC} sqlc (not needed for test job)"
    else
        check_tool "golangci-lint" "golangci-lint" "brew install golangci-lint"
        check_tool "sqlc"          "sqlc"            "brew install sqlc"
    fi
    if $CI_MODE; then
        echo -e "  ${GREEN}[OK]${NC} Docker (GitHub Actions service)"
    else
        check_docker
    fi
}

s02_build() {
    banner "2. Build"
    run_cmd "go build ./..." go build ./...
}

s03_lint() {
    banner "3. Lint (golangci-lint)"
    run_cmd "golangci-lint" golangci-lint run --no-config --timeout 5m ./...
}

s04_unit_tests() {
    banner "4. Unit Tests (no Docker required)"
    step "4.1" "Config"
    run_go_test "Config tests" ./internal/core/config/ -v

    step "4.2" "Types — Decimal + Duration"
    run_go_test "Types tests" ./internal/core/types/ -v

    step "4.3" "Errors — PG Error Mapping"
    run_go_test "Error tests" ./internal/core/errors/ -v

    step "4.4" "Hooks — HookManager + CRUDHooks + Plugin Registry"
    run_go_test "Hooks tests" ./internal/core/hooks/ -v

    step "4.5" "Middleware — CORS, RequestID, Auth, Actor"
    run_go_test "Middleware tests" ./internal/core/middleware/ -v

    step "4.6" "AI — Noop Provider"
    run_go_test "AI tests" ./internal/core/ai/ -v

    step "4.7" "Jobs — Scheduled Jobs (unit)"
    run_go_test "Jobs unit tests" ./internal/core/jobs/ -v -run "TestDefault|TestScheduled|TestRegister"

    step "4.8" "Plugins — Empty Registry"
    run_go_test "Plugin tests" ./plugins/ -v

    step "4.9" "Main — Health Endpoint"
    run_go_test "Health endpoint tests" ./cmd/api/ -v
}

s05_integration_tests() {
    banner "5. Integration Tests (requires Docker)"

    if ! docker_available; then
        echo -e "  ${YELLOW}[SKIP]${NC} Docker not available — skipping integration tests"
        return
    fi

    step "5.1" "Database — Pool creation, health, schema execution"
    run_go_test "Database integration tests" ./internal/core/database/ -v -count=1

    step "5.2" "Jobs — River Worker"
    run_go_test "Worker integration test" ./internal/core/jobs/ -v -run "TestNewWorker"

    step "5.3" "App — Router with real DB (health + unhealthy)"
    run_go_test "App router integration tests" ./internal/app/ -v -count=1
}

s06_coverage() {
    banner "6. Full Test Suite + Coverage Report"
    echo ""
    run_cmd "go test all packages with coverage" go test -count=1 -coverprofile=coverage.out ./...
    echo ""

    local total
    total=$(go tool cover -func=coverage.out 2>/dev/null | grep total | awk '{print $3}')
    echo -e "  ${BOLD}Total Coverage: ${GREEN}${total}${NC}"

    local threshold=75
    local pct
    pct=$(echo "$total" | sed 's/%//')
    if echo "$pct $threshold" | awk '{exit ($1 >= $2 ? 0 : 1)}'; then
        echo -e "  ${GREEN}[PASS]${NC} Coverage ${total} >= ${threshold}% threshold"
        PASSED=$((PASSED + 1))
    else
        echo -e "  ${RED}[FAIL]${NC} Coverage ${total} < ${threshold}% threshold"
        FAILED=$((FAILED + 1))
    fi

    echo ""
    echo "  Per-package coverage:"
    go tool cover -func=coverage.out | grep -v "total:" | grep "0.0%" | sed 's/^/  /'
}

s07_docker_compose() {
    banner "7. Docker Compose — Infrastructure"

    if $CI_MODE; then
        echo -e "  ${YELLOW}[SKIP]${NC} CI mode — docker-compose skipped"
        return
    fi
    if ! docker info &>/dev/null 2>&1; then
        echo -e "  ${YELLOW}[SKIP]${NC} Docker not available"
        return
    fi

    step "7.1" "Start services"
    run_cmd "docker compose up -d" docker compose up -d

    step "7.2" "Wait for healthy"
    echo "  Waiting for containers to become healthy..."
    sleep 3
    docker compose ps 2>/dev/null | sed 's/^/  /'

    step "7.3" "PostgreSQL — table count"
    local table_count
    table_count=$(docker compose exec -T postgres psql -U vv -d vv -t -c \
        "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public';" 2>/dev/null | tr -d '[:space:]')
    if [[ -n "$table_count" && "$table_count" -ge 10 ]]; then
        echo -e "  ${GREEN}[PASS]${NC} PostgreSQL: ${table_count} tables"
        PASSED=$((PASSED + 1))
    else
        echo -e "  ${RED}[FAIL]${NC} PostgreSQL: could not query table count"
        FAILED=$((FAILED + 1))
    fi

    step "7.4" "MinIO — health check"
    local minio_status
    minio_status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 http://localhost:9000/minio/health/live 2>/dev/null || echo "000")
    if [[ "$minio_status" == "200" ]]; then
        echo -e "  ${GREEN}[PASS]${NC} MinIO: HTTP ${minio_status}"
        PASSED=$((PASSED + 1))
    else
        echo -e "  ${RED}[FAIL]${NC} MinIO: HTTP ${minio_status}"
        FAILED=$((FAILED + 1))
    fi

    step "7.5" "Stop services"
    run_cmd "docker compose down -v" docker compose down -v
}

s08_docker_build() {
    banner "8. Docker Image Build"

    if $CI_MODE; then
        echo -e "  ${YELLOW}[SKIP]${NC} CI mode — Docker build skipped"
        return
    fi
    if ! docker info &>/dev/null 2>&1; then
        echo -e "  ${YELLOW}[SKIP]${NC} Docker not available"
        return
    fi

    step "8.1" "Build image"
    run_cmd "docker build" docker build -t skolva:latest .

    step "8.2" "Image exists"
    if docker images skolva --format "{{.Repository}}:{{.Tag}} — {{.Size}}" 2>/dev/null | head -1; then
        echo -e "  ${GREEN}[PASS]${NC} Image created"
        PASSED=$((PASSED + 1))
    else
        echo -e "  ${RED}[FAIL]${NC} Image not found"
        FAILED=$((FAILED + 1))
    fi

    step "8.3" "Health check via docker-compose.prod.yml"
    echo "  Starting app with production compose..."
    docker compose -f docker-compose.prod.yml up -d 2>/dev/null | sed 's/^/  /'

    local health=""
    echo "  Waiting for app to become healthy (max 30s)..."
    for i in $(seq 1 15); do
        health=$(curl -s --max-time 3 http://localhost:8080/api/health 2>/dev/null || true)
        if echo "$health" | grep -q '"healthy"'; then
            echo "  Response after ${i}s: ${health}"
            break
        fi
        sleep 2
    done
    health=${health:-'{"status":"unreachable"}'}

    if echo "$health" | grep -q '"healthy"'; then
        echo -e "  ${GREEN}[PASS]${NC} Health endpoint returned healthy"
        PASSED=$((PASSED + 1))
    else
        echo "  Response: ${health}"
        echo -e "  ${YELLOW}App logs (last 10 lines):${NC}"
        docker compose -f docker-compose.prod.yml logs --tail 10 app 2>/dev/null | sed 's/^/    /' || true
        echo -e "  ${RED}[FAIL]${NC} Health endpoint failed"
        FAILED=$((FAILED + 1))
    fi

    step "8.4" "Cleanup prod compose"
    docker compose -f docker-compose.prod.yml down -v 2>/dev/null | sed 's/^/  /'
    echo -e "  ${GREEN}[OK]${NC} Prod environment cleaned up"
}

s09_makefile() {
    banner "9. Makefile Targets"

    step "9.1" "make tidy"
    run_cmd "make tidy" make tidy

    step "9.2" "make build"
    run_cmd "make build" make build

    step "9.3" "make clean"
    make clean 2>/dev/null | sed 's/^/  /'
    echo -e "  ${GREEN}[OK]${NC} make clean"
}

s10_full_check() {
    banner "10. Full Check — Build + Lint + Test + Coverage"

    step "10.1" "Build"
    go build ./... 2>&1 | sed 's/^/  /' || true

    step "10.2" "Lint"
    golangci-lint run --no-config --timeout 5m ./... 2>&1 | sed 's/^/  /' || true

    step "10.3" "Test + Coverage"
    go test -count=1 -coverprofile=coverage.out ./... 2>&1 | sed 's/^/  /'

    step "10.4" "Coverage summary"
    go tool cover -func=coverage.out 2>/dev/null | grep -E "ok |total:" | sed 's/^/  /' || true

    local total
    total=$(go tool cover -func=coverage.out 2>/dev/null | grep total | awk '{print $3}' | sed 's/%//')
    if [[ -n "$total" ]]; then
        echo -e "  ${BOLD}Total Coverage: ${GREEN}${total}%${NC}"
    fi
}

# ============================================================================
# Menu
# ============================================================================

ALL_STEPS=(
    "all:Run all steps in order"
    "01:Prerequisites Check"
    "02:Build"
    "03:Lint"
    "04:Unit Tests (no Docker)"
    "05:Integration Tests (needs Docker)"
    "06:Full Test Suite + Coverage"
    "07:Docker Compose (up → check → down)"
    "08:Docker Image Build + Health"
    "09:Makefile Targets"
    "10:Full Check (build+lint+test+coverage)"
)

show_menu() {
    echo ""
    echo -e "${BOLD}Available Test Steps${NC}"
    echo "──────────────────────"
    for entry in "${ALL_STEPS[@]}"; do
        local key="${entry%%:*}"
        local desc="${entry#*:}"
        printf "  %-6s %s\n" "${key}" "${desc}"
    done
    echo ""
}

run_step() {
    local step="$1"
    case "$step" in
        all)
            s01_prerequisites; s02_build; s03_lint; s04_unit_tests
            s05_integration_tests; s06_coverage; s07_docker_compose
            s08_docker_build; s09_makefile; s10_full_check
            ;;
        01) s01_prerequisites ;;
        02) s02_build ;;
        03) s03_lint ;;
        04) s04_unit_tests ;;
        05) s05_integration_tests ;;
        06) s06_coverage ;;
        07) s07_docker_compose ;;
        08) s08_docker_build ;;
        09) s09_makefile ;;
        10) s10_full_check ;;
        *)
            echo -e "${RED}Unknown step: $step${NC}"
            return 1
            ;;
    esac
}

# ============================================================================
# Main
# ============================================================================

# Ensure we run from project root
if [[ -f "go.mod" ]]; then
    true
elif [[ -f "../go.mod" ]]; then
    cd ..
else
    echo -e "${RED}ERROR: Must run from project root (where go.mod is)${NC}"
    exit 1
fi

echo -e "${BOLD}TP1 Test Suite${NC}${CI_MODE:+ (CI mode)}"
echo "Project: $(pwd)"
echo "Go:      $(go version)"

if [[ $# -eq 0 ]]; then
    show_menu
    echo -n "Which step(s)? Enter number(s) separated by space [all]: "
    read -r selection
else
    selection="$*"
fi

# Default to all if empty
if [[ -z "$selection" ]]; then
    if $CI_MODE; then
        selection="04 06"  # CI default: unit tests + coverage
    else
        selection="all"
    fi
fi

echo ""
echo -e "${BOLD}Running: ${selection}${NC}"
echo ""

for s in $selection; do
    run_step "${s,,}" || true  # lowercase, continue on error
done

summary

# CI mode: exit with failure code if any step failed
if $CI_MODE && [[ $FAILED -gt 0 ]]; then
    exit 1
fi
