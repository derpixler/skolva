#!/usr/bin/env bash
# =============================================================================
# Skolva Test Suite — shared library (harness + global steps)
# Sourced by per-phase step files and the dispatcher.
# =============================================================================
set -euo pipefail

# --- CI Mode Detection ---
if [[ "${1:-}" == "--ci" ]]; then
    CI_MODE=true; shift
else
    CI_MODE=false
fi

# --- Colors ---
if $CI_MODE; then
    RED=''; GREEN=''; YELLOW=''; BLUE=''; BOLD=''; NC=''
else
    RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
    BLUE='\033[0;34m'; BOLD='\033[1m'; NC='\033[0m'
fi

# --- Go PATH detection ---
detect_go_path() {
    if $CI_MODE; then
        if command -v go &>/dev/null; then
            dirname "$(command -v go)"; return
        fi
    fi
    for candidate in \
        "/opt/homebrew/opt/go@1.23/bin" "/opt/homebrew/bin" \
        "/usr/local/go/bin" "$HOME/go/bin" "$HOME/sdk/go1.2"*"/bin"; do
        if [[ -x "$candidate/go" ]]; then echo "$candidate"; return; fi
    done
    if command -v go &>/dev/null; then dirname "$(command -v go)"; return; fi
    echo ""
}

GO_BIN_DIR="$(detect_go_path)"
if [[ -z "$GO_BIN_DIR" ]]; then
    echo -e "${RED}ERROR: Go not found.${NC}"; exit 1
fi
export PATH="$GO_BIN_DIR:$HOME/go/bin:$PATH"

# --- State ---
PASSED=0; FAILED=0

# --- Docker / tool checks ---
docker_available() { docker info &>/dev/null 2>&1; }

check_tool() {
    local name="$1" cmd="$2" hint="$3"
    if command -v "$cmd" &>/dev/null; then echo -e "  ${GREEN}[OK]${NC} $name"; return 0
    else echo -e "  ${RED}[MISSING]${NC} $name — $hint"; return 1; fi
}

check_docker() {
    if docker info &>/dev/null 2>&1; then echo -e "  ${GREEN}[OK]${NC} Docker (running)"; return 0
    else echo -e "  ${YELLOW}[WARN]${NC} Docker not running — integration tests + compose will be skipped"; return 1; fi
}

# --- Output helpers ---
banner() {
    echo ""; echo -e "${BLUE}============================================${NC}"
    echo -e "${BOLD}  $1${NC}"; echo -e "${BLUE}============================================${NC}"
}
step() {
    local num="$1" desc="$2"
    echo ""; echo -e "${YELLOW}[${num}]${NC} ${BOLD}${desc}${NC}"
}

run_cmd() {
    local desc="$1"; shift
    echo -e "  ${BLUE}\$${NC} $*"
    if "$@"; then echo -e "  ${GREEN}[PASS]${NC} ${desc}"; PASSED=$((PASSED + 1)); return 0
    else echo -e "  ${RED}[FAIL]${NC} ${desc}"; FAILED=$((FAILED + 1)); return 1; fi
}

run_go_test() {
    local desc="$1" pkg="$2"; shift 2
    echo -e "  ${BLUE}\$${NC} go test $* ${pkg}"
    if go test -count=1 "$@" "$pkg" 2>&1 | sed 's/^/  /'; then
        echo -e "  ${GREEN}[PASS]${NC} ${desc}"; PASSED=$((PASSED + 1)); return 0
    else local rc=$?; echo -e "  ${RED}[FAIL]${NC} ${desc} (exit code $rc)"; FAILED=$((FAILED + 1)); return 1; fi
}

summary() {
    echo ""; echo -e "${BLUE}============================================${NC}"
    local total=$((PASSED + FAILED))
    echo -e "  Results: ${GREEN}${PASSED} passed${NC}, ${RED}${FAILED} failed${NC}, ${total} total"
    if [[ $FAILED -eq 0 ]]; then echo -e "  ${GREEN}${BOLD}All checks passed.${NC}"
    else echo -e "  ${RED}${BOLD}${FAILED} check(s) failed.${NC}"; fi
    echo -e "${BLUE}============================================${NC}"
}

# ============================================================================
# Global (phase-independent) steps
# ============================================================================

s_prerequisites() {
    banner "Prerequisites Check"
    echo ""; echo "  Go path: $GO_BIN_DIR/go ($(go version))"
    if $CI_MODE; then
        echo -e "  ${GREEN}[OK]${NC} golangci-lint (separate CI job)"
        echo -e "  ${GREEN}[OK]${NC} sqlc (not needed for test job)"
    else
        check_tool "golangci-lint" "golangci-lint" "brew install golangci-lint"
        check_tool "sqlc"          "sqlc"            "brew install sqlc"
    fi
    if $CI_MODE; then echo -e "  ${GREEN}[OK]${NC} Docker (GitHub Actions service)"
    else check_docker; fi
}

s_build()    { banner "Build";   run_cmd "go build ./..." go build ./...; }
s_lint()     { banner "Lint (golangci-lint)"; run_cmd "golangci-lint" golangci-lint run ./...; }

s_coverage() {
    banner "Full Test Suite + Coverage Report"
    echo ""; run_cmd "go test all packages with coverage" go test -count=1 -coverprofile=coverage.out ./...
    echo ""
    local total; total=$(go tool cover -func=coverage.out 2>/dev/null | grep total | awk '{print $3}')
    echo -e "  ${BOLD}Total Coverage: ${GREEN}${total}${NC}"
    local threshold=75; local pct; pct=$(echo "$total" | sed 's/%//')
    if echo "$pct $threshold" | awk '{exit ($1 >= $2 ? 0 : 1)}'; then
        echo -e "  ${GREEN}[PASS]${NC} Coverage ${total} >= ${threshold}% threshold"; PASSED=$((PASSED + 1))
    else
        echo -e "  ${RED}[FAIL]${NC} Coverage ${total} < ${threshold}% threshold"; FAILED=$((FAILED + 1))
    fi
    echo ""; echo "  Per-package coverage:"
    go tool cover -func=coverage.out | grep -v "total:" | grep "0.0%" | sed 's/^/  /'
}

s_docker_compose() {
    banner "Docker Compose — Infrastructure"
    if $CI_MODE; then echo -e "  ${YELLOW}[SKIP]${NC} CI mode — docker-compose skipped"; return; fi
    if ! docker info &>/dev/null 2>&1; then echo -e "  ${YELLOW}[SKIP]${NC} Docker not available"; return; fi
    step "start" "Start services"
    run_cmd "docker compose up -d" docker compose up -d
    step "wait"  "Wait for healthy"
    echo "  Waiting for containers to become healthy..."; sleep 3
    docker compose ps 2>/dev/null | sed 's/^/  /'
    step "pg"    "PostgreSQL — table count"
    local table_count; table_count=$(docker compose exec -T postgres psql -U vv -d vv -t -c \
        "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public';" 2>/dev/null | tr -d '[:space:]')
    if [[ -n "$table_count" && "$table_count" -ge 10 ]]; then
        echo -e "  ${GREEN}[PASS]${NC} PostgreSQL: ${table_count} tables"; PASSED=$((PASSED + 1))
    else echo -e "  ${RED}[FAIL]${NC} PostgreSQL: could not query table count"; FAILED=$((FAILED + 1)); fi
    step "minio" "MinIO — health check"
    local minio_status; minio_status=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 http://localhost:9000/minio/health/live 2>/dev/null || echo "000")
    if [[ "$minio_status" == "200" ]]; then echo -e "  ${GREEN}[PASS]${NC} MinIO: HTTP ${minio_status}"; PASSED=$((PASSED + 1))
    else echo -e "  ${RED}[FAIL]${NC} MinIO: HTTP ${minio_status}"; FAILED=$((FAILED + 1)); fi
    step "down"  "Stop services"
    run_cmd "docker compose down -v" docker compose down -v
}

s_docker_build() {
    banner "Docker Image Build"
    if $CI_MODE; then echo -e "  ${YELLOW}[SKIP]${NC} CI mode — Docker build skipped"; return; fi
    if ! docker info &>/dev/null 2>&1; then echo -e "  ${YELLOW}[SKIP]${NC} Docker not available"; return; fi
    step "build" "Build image"
    run_cmd "docker build" docker build -t skolva:latest .
    step "exists" "Image exists"
    if docker images skolva --format "{{.Repository}}:{{.Tag}} — {{.Size}}" 2>/dev/null | head -1; then
        echo -e "  ${GREEN}[PASS]${NC} Image created"; PASSED=$((PASSED + 1))
    else echo -e "  ${RED}[FAIL]${NC} Image not found"; FAILED=$((FAILED + 1)); fi
    step "health" "Health check via docker-compose.prod.yml"
    echo "  Starting app with production compose..."
    docker compose -f docker-compose.prod.yml up -d 2>/dev/null | sed 's/^/  /'
    local health=""
    echo "  Waiting for app to become healthy (max 30s)..."
    for i in $(seq 1 15); do
        health=$(curl -s --max-time 3 http://localhost:8080/api/health 2>/dev/null || true)
        if echo "$health" | grep -q '"healthy"'; then echo "  Response after ${i}s: ${health}"; break; fi; sleep 2
    done
    health=${health:-'{"status":"unreachable"}'}
    if echo "$health" | grep -q '"healthy"'; then
        echo -e "  ${GREEN}[PASS]${NC} Health endpoint returned healthy"; PASSED=$((PASSED + 1))
    else
        echo "  Response: ${health}"; echo -e "${YELLOW}App logs (last 10 lines):${NC}"
        docker compose -f docker-compose.prod.yml logs --tail 10 app 2>/dev/null | sed 's/^/    /' || true
        echo -e "  ${RED}[FAIL]${NC} Health endpoint failed"; FAILED=$((FAILED + 1))
    fi
    step "cleanup" "Cleanup prod compose"
    docker compose -f docker-compose.prod.yml down -v 2>/dev/null | sed 's/^/  /'
    echo -e "  ${GREEN}[OK]${NC} Prod environment cleaned up"
}

s_makefile() {
    banner "Makefile Targets"
    step "tidy"  "make tidy";  run_cmd "make tidy"  make tidy
    step "build" "make build"; run_cmd "make build" make build
    step "clean" "make clean"; make clean 2>/dev/null | sed 's/^/  /'; echo -e "  ${GREEN}[OK]${NC} make clean"
}

s_full_check() {
    banner "Full Check — Build + Lint + Test + Coverage"
    step "build"   "Build";   go build ./... 2>&1 | sed 's/^/  /' || true
    step "lint"    "Lint";    golangci-lint run ./... 2>&1 | sed 's/^/  /' || true
    step "test"    "Test + Coverage"; go test -count=1 -coverprofile=coverage.out ./... 2>&1 | sed 's/^/  /'
    step "summary" "Coverage summary"; go tool cover -func=coverage.out 2>/dev/null | grep -E "ok |total:" | sed 's/^/  /' || true
    local total; total=$(go tool cover -func=coverage.out 2>/dev/null | grep total | awk '{print $3}' | sed 's/%//')
    if [[ -n "$total" ]]; then echo -e "  ${BOLD}Total Coverage: ${GREEN}${total}%${NC}"; fi
}
