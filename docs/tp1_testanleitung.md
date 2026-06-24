# TP1 Testanleitung — Walking Skeleton + CI/CD

## Voraussetzungen

- Docker Desktop (für Integrationstests und docker-compose)
- Go 1.25+ (`brew install go@1.23`)
- sqlc (`brew install sqlc`)
- golangci-lint (`brew install golangci-lint`)
- PATH gesetzt:

```bash
export PATH="/opt/homebrew/opt/go@1.23/bin:$HOME/go/bin:$PATH"
```

---

## 1. Projekt-Setup + CI/CD

### 1.1 Build — Kompiliert das gesamte Projekt

```bash
go build ./...
```

**Erwartet:** Kein Output (keine Fehler).

**Prüfung:**

```bash
go build ./... && echo "BUILD OK"
# Ausgabe: BUILD OK
```

**Fehlerbilder:**
- `command not found: go` → Go nicht installiert oder PATH falsch
- `no required module` → `go mod tidy` ausführen

---

### 1.2 Lint — Statische Codeanalyse

```bash
golangci-lint run --no-config --timeout 5m ./...
```

**Erwartet:** `0 issues.`

**Fehlerbilder:**
- `can't load config` → golangci-lint Version ≥2.x, `--no-config` nutzen
- `Error return value of X is not checked` → errcheck-Fundstelle, Code fixen

---

### 1.3 Unit-Tests (ohne Docker)

```bash
SKIP_INTEGRATION=1 go test -v -count=1 ./internal/core/config/ \
  ./internal/core/types/ \
  ./internal/core/errors/ \
  ./internal/core/hooks/ \
  ./internal/core/middleware/ \
  ./internal/core/ai/ \
  ./internal/core/jobs/ \
  ./plugins/ 2>&1
```

**Erwartet:** Alle Tests `PASS`. Integrationstests werden geskippt.

**Typische Ausgabe pro Paket:**

```
=== RUN   TestLoadDefaults
--- PASS: TestLoadDefaults (0.00s)
...
ok    github.com/vereinsverwaltung/vereinsverwaltung/internal/core/config   0.385s
```

---

### 1.4 Integrationstests (benötigt Docker)

```bash
go test -v -count=1 ./internal/core/database/ ./internal/core/jobs/ ./internal/app/ 2>&1
```

**Erwartet:** testcontainers-go startet PostgreSQL-Container, führt schema.sql aus, testet Dual Pools und River Worker.

```bash
# Typische Ausgabe:
=== RUN   TestNewPools
2026/06/24 ... 🐳 Creating container for image postgres:16-alpine
...
--- PASS: TestNewPools (5.44s)
=== RUN   TestSchemaExecution
...
--- PASS: TestSchemaExecution (1.93s)
```

**Fehlerbilder:**
- `Cannot connect to the Docker daemon` → Docker Desktop starten
- `failed to execute schema: ERROR: ...` → schema.sql hat Syntaxfehler (gemeldet als SQLSTATE)

---

### 1.5 Vollständiger Testlauf + Coverage

```bash
go test -count=1 -coverprofile=coverage.out ./... 2>&1
```

**Erwartet:** Alle 11 Pakete `ok`.

```bash
go tool cover -func=coverage.out | grep total
# total:  (statements)  80.3%
```

**Paket-Coverage im Detail:**

| Paket | Coverage |
|-------|----------|
| `cmd/api` | 0.0% (Entrypoint, nicht testbar) |
| `internal/app` | 100.0% |
| `internal/core/ai` | 100.0% |
| `internal/core/middleware` | 96.2% |
| `internal/core/config` | 93.9% |
| `internal/core/types` | 93.0% |
| `internal/core/errors` | 88.9% |
| `internal/core/jobs` | 88.2% |
| `internal/core/hooks` | 86.0% |
| `internal/core/database` | 66.7% |
| `plugins` | 100.0% |

---

### 1.6 CI/CD Pipeline (GitHub Actions)

Datei: `.github/workflows/ci.yml`

**Stufen:**
1. `lint` — golangci-lint auf ubuntu-latest
2. `test` — Go-Tests gegen PostgreSQL-16-Service, Coverage ≥80%
3. `build` — `go build` + `docker build`

**Lokal simulieren (nur Lint+Test):**

```bash
make lint && go test -count=1 -coverprofile=coverage.out ./...
```

---

### 1.7 Docker Compose — Infrastruktur hochfahren

```bash
docker compose up -d
```

**Erwartet:** Zwei Container laufen:

```bash
docker compose ps
# NAME                           STATUS                   PORTS
# vereinsverwaltung-postgres-1   Up (healthy)             0.0.0.0:5432->5432/tcp
# vereinsverwaltung-minio-1      Up (healthy)             0.0.0.0:9000-9001->9000-9001/tcp
```

**Prüfung PostgreSQL:**

```bash
docker compose exec postgres psql -U vv -d vv -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public';"
#  count
# -------
#     52
```

**Prüfung MinIO:**

```bash
curl -s http://localhost:9000/minio/health/live
# (HTTP 200, MinIO-Response)
```

**Herunterfahren:**

```bash
docker compose down -v
```

---

### 1.8 Docker Image bauen

```bash
docker build -t vereinsverwaltung:latest .
```

**Erwartet:** Multi-Stage Build erfolgreich.

```bash
docker images vereinsverwaltung
# REPOSITORY           TAG       IMAGE ID       CREATED         SIZE
# vereinsverwaltung    latest    abc123def456   5 seconds ago   ~150MB
```

**Health-Check nach Start (mit docker-compose.prod.yml):**

```bash
docker compose -f docker-compose.prod.yml up -d
sleep 5
curl -s http://localhost:8080/api/health
# {"status":"healthy"}
docker compose -f docker-compose.prod.yml down -v
```

---

### 1.9 Makefile-Kommandos

```bash
make help  # (falls implementiert) oder direkter Aufruf:

make build      # go build -o bin/vereinsverwaltung ./cmd/api
make test       # go test -v -count=1 -coverprofile=coverage.out ./...
make lint       # golangci-lint run ./...
make run        # go run ./cmd/api
make docker-up  # docker compose up -d
make docker-down # docker compose down -v
make tidy       # go mod tidy
make clean      # entfernt bin/, coverage.out
```

---

## 2. Config + Types + Errors

### 2.1 Config — Env-Variablen laden

```bash
go test -v -run "TestLoad" ./internal/core/config/
```

**Einzeltests:**

| Test | Was er prüft |
|------|-------------|
| `TestLoadDefaults` | Default-Werte: Port=8080, Env=development, JWTExpiry=24 |
| `TestLoadOverrides` | Überschreibung via Env: Port=9090, Env=production, JWTExpiry=48 |
| `TestLoadMissingRequired` | Fehlendes DATABASE_URL → panic |
| `TestModulesEnabled` | MODULES_ENABLED Parsing + IsModuleEnabled() |
| `TestAIGDPRModeDisabled` | AI_GDPR_MODE Default = "disabled" |
| `TestStorageBackend` | STORAGE_BACKEND + S3_ENDPOINT |

**Manueller Test:**

```bash
DATABASE_URL="postgres://test:test@localhost/test" \
JWT_SECRET="secret" \
APP_PORT=9999 \
  go run ./cmd/api 2>&1 | grep "starting"
# server starting on :9999
```

---

### 2.2 Types — Decimal (shopspring/decimal)

```bash
go test -v -run "Decimal" ./internal/core/types/
```

| Test | Was er prüft |
|------|-------------|
| `TestNewDecimal` | String → Decimal ("123.45") |
| `TestMustDecimal` | MustDecimal("67.89") → kein panic |
| `TestMustDecimalPanics` | "not-a-decimal" → panic |
| `TestDecimalJSONRoundTrip` | {"amount":"99.99"} → Marshal → Unmarshal |
| `TestDecimalPrecision` | 0.1 + 0.2 == 0.3 (kein Floating-Point-Fehler) |
| `TestZeroDecimal` | Zero.IsZero() == true |

---

### 2.3 Types — Duration (erweiterter Parser)

```bash
go test -v -run "Duration" ./internal/core/types/
```

| Test | Eingabe | Erwartet |
|------|---------|----------|
| `TestParseDurationStandard` | `"1h30m"` | 90m |
| `TestParseDurationDays` | `"3d"` | 72h |
| `TestParseDurationDaysAndHours` | `"2d5h"` | 53h |
| `TestParseDurationMonths` | `"1M"` | 720h (30d) |
| `TestParseDurationComplex` | `"1d3M2h30m"` | 24h + 2160h + 2h + 30m |
| `TestParseDurationEmpty` | `""` | Error |
| `TestParseDurationInvalid` | `"xyz"` | Error |
| `TestFormatDuration` | 25h | `"1d1h0m"` |

---

### 2.4 Errors — PG Error Mapping

```bash
go test -v -run "TestFromPGError\|TestNew\|TestAppError" ./internal/core/errors/
```

**Mapping:**

| PG Code | HTTP-Status | Bedeutung |
|---------|-------------|-----------|
| 23505 | 409 Conflict | Unique-Verletzung |
| 23503 | 400 Bad Request | FK-Verletzung |
| 23514 | 422 Unprocessable | Check-Constraint |
| 23P01 | 409 Conflict | Exclusion/Overlap |
| P0001 | 422 Unprocessable | Trigger RAISE |
| XX999 | 500 Internal | Unbekannt |

**Test: Fehlerdetail wird übernommen:**

```go
// pgErr.Detail = "Pachtvertrag ueberschneidet sich zeitlich."
// → AppError.Message = "Pachtvertrag ueberschneidet sich zeitlich."
```

---

## 3. Database (Dual Pools + testcontainers)

```bash
go test -v -run "TestNewPools\|TestPools\|TestDual\|TestSchema" ./internal/core/database/
```

### 3.1 Pool-Erstellung

**TestNewPools:** Startet PostgreSQL-Container, erstellt Web-Pool (max 20) + Worker-Pool (max 5), beide antworten auf Ping.

**TestPoolsHealth:** Beide Pools liefern `Ping()` ohne Fehler.

### 3.2 Dual Pool Isolation

**TestDualPoolIsolation:** Getrennte `pgxpool.Pool`-Instanzen mit eigenen `MaxConns`-Werten. Web blockiert Worker nicht und umgekehrt.

### 3.3 Schema-Ausführung

**TestSchemaExecution:** Liest `schema.sql`, führt es aus, zählt Tabellen (≥10).

### 3.4 Fehlerpfade

**TestNewPoolsInvalidURL:** `"invalid://url"` → Fehler.

**TestPoolsClose:** `Close()` ohne Panic.

**TestPoolsHealthError:** `Health()` nach `Close()` → Fehler.

---

## 4. Hooks

```bash
go test -v -run "TestHookManager\|TestCRUDHooks\|TestPlugin" ./internal/core/hooks/
```

### 4.1 HookManager — Actions

| Test | Was er prüft |
|------|-------------|
| `TestHookManagerAddAction` | Einfache Action wird aufgerufen |
| `TestHookManagerActionPriority` | 3 Actions → Reihenfolge: [10, 20, 30] |
| `TestHookManagerMultipleHandlers` | 5 Handler → alle 5 aufgerufen |
| `TestHookManagerNoHandler` | Nicht-existenten Hook aufrufen → kein Fehler |
| `TestHookManagerActionError` | Action gibt Error zurück → Fehler propagiert |

### 4.2 HookManager — Filters

| Test | Was er prüft |
|------|-------------|
| `TestHookManagerFilter` | Filter transformiert HookContext |
| `TestHookManagerFilterChain` | Zwei Filter in Kette → beide angewandt |
| `TestHookManagerNoFilter` | Nicht-existenten Filter → unveränderter Context |
| `TestHookManagerFilterError` | Filter Error → Fehler propagiert |

### 4.3 HookManager — Plugin-Entfernung

| Test | Was er prüft |
|------|-------------|
| `TestHookManagerRemovePlugin` | Plugin-Action nach RemovePlugin nicht mehr vorhanden |
| `TestHookManagerRemovePluginFilters` | Plugin-Filter nach RemovePlugin nicht mehr vorhanden |

### 4.4 CRUDHooks

| Test | Was er prüft |
|------|-------------|
| `TestCRUDHooksCreateLifecycle` | BeforeCreate + AfterCreate in richtiger Reihenfolge |
| `TestCRUDHooksUpdateLifecycle` | BeforeUpdate + AfterUpdate |
| `TestCRUDHooksDeleteLifecycle` | BeforeDelete + AfterDelete |
| `TestCRUDHooksWithFilters` | create.validate-Filter ändert Entity vor BeforeCreate |
| `TestCRUDHooksWithoutCallbacks` | Kein Callback registriert → kein Fehler |

### 4.5 Plugin Registry

| Test | Was er prüft |
|------|-------------|
| `TestPluginRegistryRegisterAll` | Register() aufgerufen |
| `TestPluginRegistryActivateAll` | Activate() aufgerufen |
| `TestPluginRegistryDeactivateAll` | Deactivate() aufgerufen |
| `TestPluginRegistryAll` | All() gibt registrierte Plugins zurück |
| `TestPluginLifecycle` | Vollständiger Zyklus: Register → Activate → Deactivate |

---

## 5. Middleware

```bash
go test -v ./internal/core/middleware/
```

### 5.1 CORS

| Test | Was er prüft |
|------|-------------|
| `TestCORS` | GET → `Access-Control-Allow-Origin: *` gesetzt |
| `TestCORSPreflight` | OPTIONS → HTTP 204 |

### 5.2 RequestID

| Test | Was er prüft |
|------|-------------|
| `TestRequestID` | Generiert UUID als X-Request-ID |
| `TestRequestIDWithHeader` | Übernimmt existierenden X-Request-ID-Header |

### 5.3 Auth (Skeleton)

| Test | Was er prüft |
|------|-------------|
| `TestAuthSkeletonNoToken` | Kein Token → kein Actor im Context |
| `TestAuthSkeletonTestToken` | `Bearer test-token` → Admin-Actor gesetzt |
| `TestRequireAuthNoActor` | Kein Actor → 401 |
| `TestRequireAuthWithActor` | Actor vorhanden → 200 |
| `TestRequirePermissionDenied` | "mitglied" ohne admin.jobs → 403 |
| `TestRequirePermissionAdmin` | "admin"-Rolle → alle Permissions erlaubt |

### 5.4 Actor

| Test | Was er prüft |
|------|-------------|
| `TestActorMiddleware` | Akzeptiert gesetzten Actor im Context |

---

## 6. Jobs (river)

```bash
# Unit-Tests (kein Docker):
SKIP_INTEGRATION=1 go test -v -run "TestDefault\|TestScheduled\|TestRegister" ./internal/core/jobs/

# Integrationstest (benötigt Docker):
go test -v -run "TestNewWorker" ./internal/core/jobs/
```

### 6.1 Scheduled Jobs

| Test | Was er prüft |
|------|-------------|
| `TestDefaultScheduledJobs` | Rückgabe ist nicht nil |
| `TestScheduledJobStruct` | Struct-Felder Name, Schedule |
| `TestRegisterScheduledJobs` | Registrierung ohne Fehler |
| `TestSchedulerContextCancellation` | Start + Cancel → stoppt sauber |

### 6.2 Worker (Integration)

**TestNewWorker:** Startet PostgreSQL-Container, führt schema.sql aus, erstellt River-Client mit 4 Queues (default:10, webhooks:5, mail:3, pdf:2), startet und stoppt Worker.

---

## 7. AI Noop Interface

```bash
go test -v ./internal/core/ai/
```

| Test | Was er prüft |
|------|-------------|
| `TestNoopComplete` | Complete() → "", nil |
| `TestNoopClassify` | Classify() → "", 0.0, nil |
| `TestNoopExtract` | Extract() → nil, nil |
| `TestNoopProviderImplementsInterface` | Compile-Zeit-Check: NoopProvider implementiert Provider |

---

## 8. Main + E2E

### 8.1 Unit — Router + Health

```bash
go test -v -run "Health" ./cmd/api/
```

| Test | Was er prüft |
|------|-------------|
| `TestHealthEndpoint` | GET /api/health → 200 |
| `TestHealthEndpointCORSHeaders` | Response enthält CORS + X-Request-ID Header |

### 8.2 Integration — App Router mit echter DB

```bash
go test -v -run "TestNewRouter" ./internal/app/
```

| Test | Was er prüft |
|------|-------------|
| `TestNewRouterHealth` | Echter DB-Pool → GET /api/health → 200 |
| `TestNewRouterUnhealthy` | Pool geschlossen → GET /api/health → 503 |

---

## 9. Plugin-Leer-Registry

```bash
go test -v ./plugins/
```

| Test | Was er prüft |
|------|-------------|
| `TestAll` | All() → leere Liste (0 Plugins registriert) |

---

## 10. Gesamtprüfung — alle Tests auf einmal

```bash
# Kompletter Durchlauf:
go build ./... && \
golangci-lint run --no-config --timeout 5m ./... && \
go test -count=1 -coverprofile=coverage.out ./... && \
go tool cover -func=coverage.out | grep total
```

**Erwartetes Endergebnis:**

```
0 issues.                                     # lint
ok  .../cmd/api                               # 0.0%
ok  .../internal/app                          # 100.0%
ok  .../internal/core/ai                      # 100.0%
ok  .../internal/core/config                  # 93.9%
ok  .../internal/core/database                # 66.7%
ok  .../internal/core/errors                  # 88.9%
ok  .../internal/core/hooks                   # 86.0%
ok  .../internal/core/jobs                    # 88.2%
ok  .../internal/core/middleware              # 96.2%
ok  .../internal/core/types                   # 93.0%
ok  .../plugins                               # 100.0%
total:  (statements)  80.3%
```

---

## 11. Fehlerbehebung (Troubleshooting)

### Docker nicht erreichbar

```bash
docker info 2>&1 | head -1
# Sollte: "Client: Docker Engine - Community" o.ä.
# Sonst: Docker Desktop starten
```

### testcontainers schlägt fehl

```bash
# Integrationstests überspringen:
SKIP_INTEGRATION=1 go test ./...
```

### Go-Toolchain-Wechsel

Go 1.23 installiert, aber go.mod erfordert 1.25 (wg. Gin v1.12):

```bash
# Go 1.25 automatisch herunterladen:
go mod tidy
# Oder ältere Gin-Version:
go get github.com/gin-gonic/gin@v1.10.0
```

### golangci-lint Version 2

```bash
# V2 benötigt --no-config oder überarbeitete YAML:
golangci-lint run --no-config --timeout 5m ./...
```

### schema.sql wird nicht ausgeführt (docker-compose)

Das Schema wird nur beim ERSTEN `docker compose up` ausgeführt (docker-entrypoint-initdb.d). Bei `down -v` werden die Volumes gelöscht — das Schema wird beim nächsten `up` neu ausgeführt.
