# Testing

## Quick Reference

```bash
# Interactive menu (all test steps, TP1 + TP2)
./scripts/test.sh

# Run specific steps
./scripts/test.sh 01 02 03      # prerequisites, build, lint
./scripts/test.sh 04             # all unit tests (TP1 + TP2, no Docker)
./scripts/test.sh 04b            # TP2 unit tests only
./scripts/test.sh 05             # all integration tests (needs Docker)
./scripts/test.sh 05b            # TP2 integration tests only
./scripts/test.sh 06             # full test suite + coverage (≥75%)
./scripts/test.sh all            # run everything

# CI mode (no colors, no interactive menu)
./scripts/test.sh --ci 04 06     # unit tests + coverage (CI-safe)
./scripts/test.sh --ci 01 04 05 06  # prereqs + unit + integration + coverage
```

## Structure

```
scripts/
  test.sh             # Dispatcher (TP1 + TP2, menu, CI mode)
  lib/
    common.sh         # Shared harness (colors, helpers, global steps)
  tests/
    tp2.sh            # TP2 curated steps (unit §1.1 – 1.12, integration §2.1 – 2.20)
```

TP1 test steps are defined inline in the dispatcher for backward compatibility.

## TP2 Test Coverage

### Unit Tests (04b — no Docker)

| § | Module | What's tested |
|---|--------|---------------|
| 1.1 | Auth password | bcrypt hash/verify, salts, edge cases |
| 1.2 | Auth JWT | token issue/verify, expiry, alg-confusion |
| 1.3 | Middleware | CORS, RequestID, Authenticate, RequireAuth, RequirePermission, ActorMiddleware |
| 1.4 | Mail | NoopMailer, SMTP builder, HTML/multipart |
| 1.5 | Secrets | AES-256-GCM encrypt/decrypt, tamper detection |
| 1.6 | Metadata | allow-list validation |
| 1.7 | Search | allow-list, empty-query short-circuit |
| 1.8 | Auth service | CheckPermission (admin bypass, role resolution) |
| 1.9 | Config | ENCRYPTION_KEY, env defaults |
| 1.10 | Types | Decimal, Duration |
| 1.11 | Errors | PG error → HTTP status mapping |
| 1.12 | OpenAPI | spec validation + route-parity |

### Integration Tests (05b — needs Docker / testcontainers)

| § | Module | What's tested |
|---|--------|---------------|
| 2.1 | dbexec | Actor tx wrapper (audit, soft-delete guard) |
| 2.2 | Auth repo users | CRUD, soft-delete, audit attribution |
| 2.3 | Auth repo roles | role/user_role CRUD, idempotent assign |
| 2.4 | Auth repo perms | permissions, resolution, admin=47 |
| 2.5 | Auth handler roles | role-assignment HTTP endpoints |
| 2.6 | Metadata | EAV get/set/delete/all via testcontainers |
| 2.7 | Search | German full-text search, soft-delete exclusion |
| 2.8 | Login | valid/invalid creds, token claims verification |
| 2.9 | Users CRUD | register, list/get/update/delete, search |
| 2.10 | 2FA | setup/confirm/verify/recovery/disable flow |
| 2.11 | E2E | register → login → 2FA → profile (full HTTP) |
| 2.12-16 | CRM | address, preferences, contacts (repo + HTTP) |
| 2.17-20 | Groups | groups + members (repo + HTTP) |

### Global Steps

| Step | Description |
|------|-------------|
| 01   | Prerequisites (Go, sqlc, golangci-lint, Docker) |
| 02   | Build (`go build ./...`) |
| 03   | Lint (`golangci-lint run ./...`) |
| 06   | Full test suite + coverage report (≥75% threshold) |
| 07   | Docker Compose (up → table count → MinIO health → down) |
| 08   | Docker image build + health check via prod compose |
| 09   | Makefile targets (tidy, build, clean) |
| 10   | Full check (build + lint + test + coverage) |

## Running Individual Tests

```bash
# Run a specific test function
go test -count=1 -run TestLoginEndpoint ./internal/auth/...

# Run all tests for a package
go test -count=1 ./internal/auth/...

# Run with coverage
go test -count=1 -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Skip integration tests
SKIP_INTEGRATION=1 go test -count=1 ./...
```

## CI Pipeline

The CI workflow (`.github/workflows/ci.yml`) runs:
1. **lint** — `golangci-lint` (separate job)
2. **test** — `./scripts/test.sh --ci 01 04 05 06` (prerequisites + unit + integration + coverage)

Integration tests use a GitHub Actions `postgres` service container + `testcontainers-go` for the jobs tests (River worker).
