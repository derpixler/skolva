# Teilplan 1: Walking Skeleton + CI/CD

## Voraussetzungen
Keine. Dies ist der erste Teilplan.

## Ziel
Minimales lauffaehiges System das exakt das liefert was Teilplan 2
(Auth + CRM) braucht. Kompiliert, Datenbank faehrt hoch, CI ist gruen.
Keine peripheren Features die erst spaeter gebraucht werden.

## Methodik: TDD
Jede Komponente wird test-first entwickelt. testcontainers-go ab dem
ersten Datenbankzugriff.

## Scope

### Was DRIN ist (Walking Skeleton fuer TP2)
```
cmd/api/main.go
internal/core/
  config/config.go              Env-Variablen
  database/postgres.go          pgx Dual Pools (Web + Worker)
  errors/apperror.go            PG Error -> HTTP Mapping
  types/decimal.go              shopspring/decimal Wrapper
  types/duration.go             Erweiterte Duration
  middleware/cors.go            CORS
  middleware/requestid.go       X-Request-ID
  middleware/auth.go            JWT Skeleton (voll in TP2)
  middleware/actor.go           User-ID -> Context
  hooks/hooks.go                HookManager: Actions + Filters
  hooks/crud_hooks.go           CRUDHooks[T] Wrapper
  hooks/plugin.go               Plugin Interface + Registry
  jobs/worker.go                river Client + Dual Pool Worker
  jobs/scheduled.go             Periodic Job Skeleton
  ai/provider.go                AIProvider Interface (nur Interface)
  ai/noop.go                    Noop Provider (gibt nil zurueck)
```

### Was NICHT DRIN ist (kommt spaeter)
```
core/mail         -> TP2 (erster Einsatz: ZFA E-Mail)
core/metadata     -> TP2 (erster Einsatz: User-Meta)
core/search       -> TP2 (erster Einsatz: User-Suche)
core/pdf          -> TP4 (erster Einsatz: Billing PDF)
core/admin        -> TP4 (erster Einsatz: echte Jobs)
core/storage      -> TP5 (erster Einsatz: Dokument-Upload)
core/ai/openai    -> TP9 (erster Einsatz: KI-Aktivierung)
core/ai/pseudo    -> TP9 (erster Einsatz: KI-Aktivierung)
webhook_delivery  -> TP9 (erster Einsatz: Integrationen)
```

### Infrastruktur (Shift-Left)
```
docker-compose.yml          PostgreSQL 16 + MinIO
docker-compose.prod.yml     Production (Volumes, Restart, Health)
Dockerfile                  Multi-Stage: Go Builder + Alpine Runtime
Makefile                    run, test, lint, build, docker-up, schema-apply, sqlc-generate
.golangci.yml               golangci-lint (go vet, staticcheck, errcheck, gosec)
.github/workflows/ci.yml    Push -> Lint -> Test -> Build
go.mod / .env.example / sqlc/sqlc.yaml
```

## Aufgaben

### 1. Projekt-Setup + CI/CD (Tag 1)
- go mod init, alle Dependencies
- Makefile: run, test, lint, build, docker-up, docker-down, schema-apply
- IMPL: .golangci.yml (go vet, staticcheck, errcheck, gosec, gocritic)
- IMPL: .github/workflows/ci.yml (Push -> Lint -> Test -> Build)
- IMPL: Coverage-Minimum in CI (go test -coverprofile, Schwelle 90%, Pipeline bricht ab)
- TEST: make lint laeuft fehlerfrei
- TEST: Push -> GitHub Actions gruen
- IMPL: docker-compose.yml (PostgreSQL 16 + MinIO)
- IMPL: Dockerfile (Multi-Stage: golang:1.23 -> alpine)
- TEST: docker build -> Image laeuft, Health OK
- IMPL: docker-compose.prod.yml

### 2. Config + Types + Errors
- TEST: config_test.go - Env laden, Defaults, Typkonvertierung
- IMPL: config/config.go
- TEST: decimal_test.go - JSON Round-Trip, Praezision
- TEST: duration_test.go - ParseDuration mit d/M
- IMPL: types/decimal.go, types/duration.go
- TEST: apperror_test.go - PG Codes -> HTTP Status (23505->409, 23P01->409, etc.)
- IMPL: errors/apperror.go

### 3. Database (Dual Pools + testcontainers)
- TEST: postgres_test.go - Pool erstellt, Health Check (testcontainers-go)
- TEST: postgres_test.go - schema.sql ausgefuehrt, Tabellen existieren
- TEST: postgres_test.go - Dual Pool Isolation: Worker voll, Web antwortet
- IMPL: database/postgres.go (Web max=20, Worker max=5)
- IMPL: sqlc/sqlc.yaml Grundkonfiguration

### 4. Hooks
- TEST: hooks_test.go - Action Reihenfolge, Prioritaeten, mehrere Handler
- TEST: hooks_test.go - Filter Chain, Transformation, Abbruch bei Fehler
- TEST: crud_hooks_test.go - WrapCreate/Update/Delete Lifecycle
- TEST: plugin_test.go - Register, Activate, Deactivate
- IMPL: hooks/hooks.go, crud_hooks.go, plugin.go

### 5. Middleware
- TEST: cors_test.go, requestid_test.go, auth_test.go (Skeleton), actor_test.go
- IMPL: middleware/cors.go, requestid.go, auth.go, actor.go

### 6. Jobs (river)
- TEST: worker_test.go - Worker startet mit Worker-Pool (testcontainers)
- TEST: scheduled_test.go - Periodic Jobs registriert
- IMPL: jobs/worker.go, scheduled.go

### 7. AI Noop Interface
- TEST: noop_test.go - Complete/Classify/Extract geben nil zurueck
- IMPL: ai/provider.go (Interface), ai/noop.go

### 8. Main + E2E
- TEST: GET /api/health -> 200
- IMPL: cmd/api/main.go (Gin Router, Dual Pools, Middleware, Health)
- TEST: Docker Image bauen + starten + Health -> 200
- TEST: CI Pipeline komplett gruen

## API-Endpunkte nach Abschluss
```
GET    /api/health
```

## Geschaetzte Sessions: 3-4
