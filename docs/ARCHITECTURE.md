# Vereinsverwaltung - Architektur

## Systemuebersicht

```
+-------------------------------------------------+
|  Beliebiges Frontend (React, Mobile, CLI)       |
+-----------------------+-------------------------+
                        | REST/JSON
+-----------------------v-------------------------+
|  Go Backend (Gin)                               |
|  +--------+------+----------+-----------+-----+ |
|  | auth   | crm  | property | workhours | ... | |
|  +--------+------+----------+-----------+-----+ |
|  | Hook-System (Actions + Filters)              | |
|  | Plugin Runtime (Go-Module + Webhooks)        | |
|  +----------------------------------------------+ |
|  | Job Queue (river) | AI (OpenAI-kompatibel)   | |
|  | PDF Engine | Storage (S3) | Mail (SMTP)      | |
+-----------------------+-------------------------+
                        | pgx / sqlc
+-----------------------v-------------------------+
|  PostgreSQL 16                                  |
|  50 Tabellen + 11 Meta + 2 Views + Trigger     |
|  river Jobs + tsvector FTS                      |
+-------------------------------------------------+
```

## Projektstruktur

```
vereinsverwaltung/
+-- cmd/
|   +-- api/
|       +-- main.go                     Entrypoint, DI, Modul-Registrierung
+-- internal/
|   +-- core/
|   |   +-- hooks/
|   |   |   +-- hooks.go               HookManager: Actions + Filters
|   |   |   +-- crud_hooks.go          CRUDHooks[T] Wrapper
|   |   |   +-- plugin.go              Plugin Interface + Registry
|   |   |   +-- webhook_delivery.go    Webhook Sender (async via river)
|   |   +-- ai/
|   |   |   +-- provider.go            AIProvider Interface
|   |   |   +-- openai.go              OpenAI-kompatible Implementierung
|   |   |   +-- noop.go                Deaktiviert (gibt nil zurueck)
|   |   +-- pdf/
|   |   |   +-- engine.go              HTML-Template -> PDF
|   |   |   +-- templates/             Mahnung, Abmahnung, Kuendigung, etc.
|   |   +-- admin/
|   |   |   +-- handler.go             Job-Dashboard API (river-Tabellen)
|   |   |   +-- translator.go          PG-Fehler -> menschenlesbare Meldungen
|   |   |   +-- routes.go
|   |   +-- metadata/
|   |   |   +-- metadata.go            EAV Meta-Tabellen (aus Ticketmanager)
|   |   +-- database/
|   |   |   +-- postgres.go            pgx Pool Setup
|   |   +-- jobs/
|   |   |   +-- worker.go              river Worker Setup
|   |   |   +-- scheduled.go           Cron-artige Jobs
|   |   +-- middleware/
|   |   |   +-- auth.go                JWT Middleware
|   |   |   +-- actor.go               User-ID -> Context
|   |   |   +-- cors.go                CORS
|   |   |   +-- requestid.go           Request-ID
|   |   +-- storage/
|   |   |   +-- interface.go           FileStorage Interface
|   |   |   +-- local.go               Disk Storage
|   |   |   +-- s3.go                  S3/MinIO
|   |   +-- search/
|   |   |   +-- search.go              tsvector Query Builder
|   |   +-- errors/
|   |   |   +-- apperror.go            PG Error -> HTTP Mapping
|   |   +-- mail/
|   |   |   +-- smtp.go                SMTP Service
|   |   +-- config/
|   |   |   +-- config.go              Env-Konfiguration
|   |   +-- types/
|   |       +-- decimal.go             shopspring/decimal Wrapper
|   |       +-- duration.go            Erweiterte Duration
|   +-- auth/
|   |   +-- model.go dto.go repository.go repository_pg.go
|   |   +-- service.go handler.go routes.go
|   +-- crm/
|   +-- groups/
|   |   +-- model.go dto.go repository.go repository_pg.go
|   |   +-- service.go handler.go routes.go
|   +-- property/
|   +-- ownership/
|   +-- leasing/
|   +-- applicants/
|   +-- documents/
|   +-- metering/
|   |   +-- ... + anomaly.go           KI-Anomalie-Erkennung
|   +-- accounting/
|   +-- billing/
|   |   +-- ... + calculator.go + pdf_job.go
|   +-- banking/
|   |   +-- ... + parser.go + parser_dkb.go + parser_sparkasse.go
|   |   +-- parser_generic.go + categorizer.go
|   +-- lending/
|   |   +-- ... + overdue_job.go
|   +-- operations/
|   +-- workhours/
|   |   +-- model.go dto.go repository.go repository_pg.go
|   |   +-- service.go handler.go routes.go
|   |   +-- planner.go                Einsatzplanung + KI-Vorschlaege
|   |   +-- balance.go                Soll/Ist Berechnung
|   |   +-- deadline_job.go           Jahresende: Ersatzgeld -> billing
|   |   +-- suggestion_job.go         Monatlich: KI-Terminvorschlaege
|   +-- compliance/
|   |   +-- model.go dto.go repository.go repository_pg.go
|   |   +-- service.go handler.go routes.go
|   |   +-- dunning_service.go        Mahnwesen (auto-draft + Eskalation)
|   |   +-- warning_service.go        Abmahnungen
|   |   +-- termination_service.go    Kuendigungen
|   |   +-- escalation_job.go         Scheduled: Mahnstufen hochsetzen
|   |   +-- expiry_job.go             Scheduled: Abmahnungen ablaufen
|   |   +-- ai_assessor.go            KI: Schweregrad + Rechtsgrundlage
|   |   +-- ai_drafter.go             KI: Korrespondenz generieren
|   |   +-- hooks.go                  Registriert sich auf Events anderer Module
|   +-- webhooks/
|   |   +-- model.go dto.go repository.go repository_pg.go
|   |   +-- service.go handler.go routes.go
|   +-- audit/
|       +-- model.go repository.go repository_pg.go
|       +-- handler.go routes.go
+-- plugins/
|   +-- registry.go                    Plugin-Imports
|   +-- example-welcome-mail/
|       +-- plugin.go
+-- schema.sql
+-- sqlc/
|   +-- sqlc.yaml
|   +-- queries/
|       +-- auth.sql crm.sql property.sql ownership.sql leasing.sql
|       +-- applicants.sql documents.sql metering.sql accounting.sql
|       +-- billing.sql banking.sql lending.sql sharing.sql
|       +-- operations.sql workhours.sql compliance.sql
|       +-- webhooks.sql audit.sql metadata.sql
+-- docker-compose.yml
+-- Makefile
+-- go.mod
+-- docs/
```

## Design Patterns

### Entwicklungsmethodik: TDD (Test-Driven Development)

Jede Funktion wird nach dem Red-Green-Refactor Zyklus entwickelt:
1. Test schreiben (Red: Test schlaegt fehl)
2. Minimale Implementierung (Green: Test besteht)
3. Refactoring (Clean: Code verbessern, Tests bleiben gruen)

Es gibt keine separate Test-Phase. Tests sind integraler Bestandteil
jeder Aufgabe. Kein Code ohne vorherigen Test. Kein Merge ohne gruene Tests.

Test-Ebenen:
- Unit Tests: Service-Logik mit gemocktem Repository
- Integration Tests: Repository gegen echtes PostgreSQL (testcontainers)
- HTTP Tests: Handler mit Gin Test-Mode
- E2E: Kompletter Request-Lifecycle

### Request-Lifecycle

```
HTTP Request
  -> Gin Middleware (CORS, RequestID, JWT Auth, Actor)
  -> Handler (parst Request, validiert DTO)
    -> Hook: {entity}.create.validate (Filter)
    -> Hook: {entity}.before_create (Action)
    -> Service (Business-Logik)
      -> Repository Interface -> sqlc/pgx
      -> river Job einfuegen (gleiche Transaktion)
    -> Hook: {entity}.after_create (Action)
      -> Webhook-Delivery Jobs (async via river)
    -> Hook: {entity}.create.response (Filter)
  -> Handler (JSON Response)
  -> PostgreSQL Trigger: audit_logs INSERT
```

### Modul-Interface

```go
type Module interface {
    RegisterRoutes(router *gin.RouterGroup)
}
```

### Repository, DTO, Service Patterns

Jedes Modul: model.go, dto.go, repository.go (Interface), repository_pg.go,
service.go (Business-Logik + CRUDHooks), handler.go (HTTP), routes.go.

## Zwei-Faktor-Authentifizierung (ZFA)

### Login-Flow mit ZFA

```
POST /api/auth/login (email + password)
  -> Passwort korrekt + ZFA NICHT aktiv:
     -> JWT zurueck (wie bisher)
  -> Passwort korrekt + ZFA aktiv:
     -> { "requires_2fa": true, "temp_token": "..." }
     -> POST /api/auth/2fa/verify (temp_token + totp_code)
        -> JWT zurueck
```

### ZFA-Setup

```
POST /api/auth/2fa/setup (authentifiziert)
  -> Generiert TOTP Secret + QR-Code (otpauth:// URI)
  -> Speichert Secret verschluesselt in user_totp_secrets
  -> Gibt QR-Code (Base64 PNG) + Recovery Codes zurueck

POST /api/auth/2fa/confirm (erster TOTP Code)
  -> Verifiziert dass User die App korrekt eingerichtet hat
  -> Setzt is_enabled=TRUE, verified_at=NOW()
```

### Erzwingung pro Rolle

Rollen admin, vorstand, kassierer koennen ZFA-Pflicht haben.
Konfigurierbar per Env: ZFA_REQUIRED_ROLES=admin,vorstand,kassierer
User mit diesen Rollen werden nach Login zum ZFA-Setup weitergeleitet
wenn noch nicht eingerichtet.

### Brute-Force-Schutz

- failed_attempts zaehlt fehlgeschlagene TOTP-Versuche
- Nach 5 Fehlversuchen: locked_until = NOW() + 15 Minuten
- Recovery Codes: 10 Einmal-Codes, bcrypt-gehasht gespeichert

### CRUDHooks Wrapper

Automatische Hook-Platzierung: {entity}.create.validate, before_create,
after_create, create.response. Analog fuer update, delete, read, list.

### Prozess-spezifische Hooks

Alle CRUD-Hooks plus:
- lease/dunning/warning/termination/work_event: status.validate/before_change/after_change
- accounting: entry.validate, journal.before_lock/after_lock
- billing: line_item.calculate, period.before_calculate/after_calculate/before_approve/after_approve
- banking: import.detect_format/parse_row/categorize/duplicate_check/before_persist/after_persist
- compliance: incident.ai_assess, dunning.auto_draft, warning.auto_draft
- workhours: event.ai_suggest_date, event.ai_suggest_participants
- document: before_upload/after_upload/before_serve
- meter_reading: after_create (Anomalie)
- lending: record.before_checkout/after_checkout/before_return/after_return/overdue
- sharing: link.before_visit/after_visit
- auth: before_login/after_login/authorize/before_register/after_register
- request: before/after

## Erweiterungs-Modell (4 Stufen)

```
Stufe 1: EAV Metadata        Dynamische Felder via REST API (kein Code)
Stufe 2: Business Rules       Wenn-Dann via UI (kein Code, spaeter)
Stufe 3: Go-Plugins           Volle Module (Kompilierung noetig)
Stufe 4: Webhooks             Externe HTTP-Integrationen
```

### Stufe 3: Go-Plugins

```go
type Plugin interface {
    Name() string
    Version() string
    Description() string
    Register(hooks *HookManager) error
    Activate(db *pgxpool.Pool) error
    Deactivate() error
}
```

Plugin-Registry (plugins/registry.go):

```go
func All() []core.Plugin {
    return []core.Plugin{
        &welcomemail.Plugin{},
        &datev.Plugin{},
    }
}
```

### Stufe 4: Webhooks

Async via river. HMAC-SHA256 signiert. Retry mit Backoff.

## Automatisierung + Freigabe

```
Scheduled Job erzeugt Entwurf (status='draft')
  -> Vorstand sieht Entwuerfe
    -> Freigabe -> 'approved' -> PDF generiert -> 'issued' -> Versand
    -> Ablehnung -> 'cancelled'
```

Gilt fuer: Mahnungen, Abmahnungen, Kuendigungen, Abrechnungen.

## Admin Dashboard (Job-Queue Verwaltung)

REST API fuer die Verwaltung der river Job-Queue.
Liest river-interne Tabellen (river_job, river_leader) aus.
Uebersetzt kryptische PG-Fehler in menschenlesbare Meldungen.

```
GET    /api/admin/jobs              Liste (pending/running/failed/completed)
GET    /api/admin/jobs/:id          Details + Fehler (uebersetzt)
POST   /api/admin/jobs/:id/retry    Job erneut ausfuehren
POST   /api/admin/jobs/:id/cancel   Job abbrechen
GET    /api/admin/jobs/stats        {pending: 5, running: 2, failed: 1}
GET    /api/admin/health            DB, Storage, Mail, AI Status
```

Fehler-Uebersetzer (Runbooks):
- 23P01 -> "Pachtvertrag ueberschneidet sich zeitlich."
- 23505 -> "Dieser Eintrag existiert bereits (Duplikat)."
- ECONNREFUSED -> "Externer Dienst nicht erreichbar."
- timeout -> "Zeitueberschreitung."

Permission: admin.jobs (nur admin + vorstand).

## Job Queue (river)

Queues: default (10 Worker), webhooks (5), mail (3), pdf (2).
Jobs transaktional einfuegbar (gleiche DB-Transaktion).

| Job | Queue | Schedule |
|-----|-------|----------|
| WebhookDeliveryJob | webhooks | - |
| SendMailJob | mail | - |
| GeneratePdfJob | pdf | - |
| ProcessBankImportJob | default | - |
| OverdueLendingCheckJob | default | taeglich 08:00 |
| DunningEscalationJob | default | woechentlich |
| WorkHourDeadlineJob | default | jaehrlich 01.12. |
| WorkEventSuggestionJob | default | monatlich |
| WarningExpiryJob | default | monatlich |
| ExpiredLinksCleanupJob | default | taeglich 02:00 |
| PaymentReminderJob | mail | monatlich |
| MeterAnomalyCheckJob | default | bei neuem Zaehlerstand |

## Dual Connection Pools

Zwei getrennte pgx-Pools auf dieselbe Datenbank:

```go
webPool, _ := pgxpool.New(ctx, dbURL)       // max_conns=20
webPool.Config().MaxConns = 20

workerPool, _ := pgxpool.New(ctx, dbURL)    // max_conns=5
workerPool.Config().MaxConns = 5

// Gin Handler nutzen webPool
// River Worker nutzen workerPool
```

Wenn Worker erschoepft: Jobs stauen sich, Web bleibt responsive.
Outbox-Pattern bleibt intakt (eine Datenbank, atomare Transaktionen).

## Circuit Breaker

gobreaker fuer alle externen API-Aufrufe (KI, Webhooks, SMTP):

```go
breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name:        "openai-api",
    MaxRequests: 1,                    // Half-Open: 1 Test-Request
    Interval:    60 * time.Second,
    Timeout:     30 * time.Second,     // Open -> Half-Open nach 30s
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        return counts.ConsecutiveFailures > 2  // 3 Fehler -> Open
    },
})

result, err := breaker.Execute(func() (any, error) {
    return ai.Complete(ctx, prompt, opts)
})
```

State Machine: Closed -> Open (3 Fehler) -> Half-Open (1 Test) -> Closed/Open
Fallback bei Open: draft ohne KI, markiert fuer manuelle Pruefung.

## KI-Integration

```go
type AIProvider interface {
    Complete(ctx context.Context, prompt string, opts CompletionOpts) (string, error)
    Classify(ctx context.Context, text string, categories []string) (string, float64, error)
    Extract(ctx context.Context, text string, schema any) (any, error)
}
```

Eine Implementierung (OpenAI-kompatibel) fuer alle Provider.
API-Key + URL + Model bestimmt den Provider.

### Pseudonymisierungs-Middleware

Jeder KI-Aufruf durchlaeuft eine Pseudonymisierungsschicht:

```go
type PseudonymizingProvider struct {
    inner     AIProvider        // Eigentlicher Provider (OpenAI, Ollama)
    pseudonym *Pseudonymizer    // Regex + NER Engine
    breaker   *gobreaker.CircuitBreaker
}

func (p *PseudonymizingProvider) Complete(ctx, prompt, opts) (string, error) {
    masked, mapping := p.pseudonym.Mask(prompt)   // NER + Regex
    result, err := p.breaker.Execute(func() (any, error) {
        return p.inner.Complete(ctx, masked, opts)
    })
    return p.pseudonym.Unmask(result, mapping), err
}
```

Strukturierte Daten (Bank-Import):
- Nur Verwendungszweck senden, Name/IBAN weglassen
- Verbleibende IBANs per Regex maskieren

Unstrukturierte Texte (Verstoesse):
- Lokale NER (prose/v2): Erkennt Namen, Orte, Organisationen
- Ersetzt durch Tokens: "Hans Mueller" -> "Mitglied_A"
- In-Memory Mapping, Rueck-Mapping nach KI-Antwort
- Personenbezogene Daten verlassen Server nie im Klartext

```
core/ai/
  provider.go              AIProvider Interface
  openai.go                OpenAI-kompatibel
  noop.go                  Deaktiviert
  pseudonymizer.go         Regex + Token-Mapping Engine
  pseudonymizer_ner.go     Lokale NER (prose/v2, CPU-only, <10MB RAM)
  middleware.go            PseudonymizingProvider (wraps inner + breaker)
```

Integration via Hook-System als Go-Plugin:

```go
func (p *AIPlugin) Register(hooks *HookManager) error {
    hooks.AddFilter("incident.create.validate", 10, "ai", p.suggestSeverity)
    hooks.AddFilter("banking.import.categorize", 20, "ai", p.categorizeTx)
    hooks.AddAction("dunning_notice.after_create", 10, "ai", p.draftLetter)
    hooks.AddAction("meter_reading.after_create", 10, "ai", p.checkAnomaly)
    return nil
}
```

## PDF-Generierung

HTML-Templates -> PDF via chromedp oder wkhtmltopdf.

Templates: Mahnung (Stufe 1-3), Abmahnung, Kuendigung,
Jahresabrechnung, Arbeitseinsatz-Einladung, Willkommensschreiben.

Erzeugung async als river Job (GeneratePdfJob).
Ergebnis wird als Document gespeichert + via document_relations verknuepft.

## Compliance als erweiterndes Modul

Das Compliance-Modul registriert sich auf Events anderer Module:

```
billing.period.after_approve       -> Offene Posten pruefen -> Mahnung draft
work_event_participant.no_show     -> Verstoß vorschlagen
workhours.deadline_passed          -> Ersatzgeld + ggf. Mahnung
lease.status.after_change='ended'  -> Raeumungsfrist pruefen
```

API-Routen unter bestehenden Ressourcen:
  GET /api/users/:id/incidents
  GET /api/users/:id/warnings
  GET /api/units/:id/incidents

## Arbeitsstunden: 3-Ebenen Pflicht

```
Gesamtpflicht = Globale Basis (6h)
              + Unit-Verpflichtung (+4h Vorgarten Parzelle 105)
              + Individuelle Anpassung (-2h Alter >70)
              = 8h
```

Unit-Verpflichtung wandert automatisch mit dem aktiven Lease zum Paechter.
view_work_hour_balance berechnet Soll/Ist pro Mitglied pro Jahr.

## Volltextsuche

tsvector + GIN-Index auf: users, units, documents, bank_transactions,
lendable_items, incidents, work_task_catalog.

## Metadata (EAV)

Kern-Felder = echte Spalten. Plugin-Erweiterungen = *_meta Tabellen.
Meta-Tabellen fuer: users, units, leases, documents, applicants,
accounting_journal, billing_periods, bank_transactions, lendable_items,
work_events, incidents.

## Storage

```go
type FileStorage interface {
    Upload(ctx context.Context, key string, reader io.Reader, contentType string) error
    Download(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    URL(ctx context.Context, key string) (string, error)
}
```

Implementierungen: LocalStorage (Disk), S3Storage (MinIO/AWS).
Env: STORAGE_BACKEND=local|s3

## Error Handling

| PG Error | HTTP Status | Bedeutung |
|----------|-------------|-----------|
| 23505 | 409 | Unique Violation |
| 23503 | 400 | Foreign Key Violation |
| 23514 | 422 | Check Constraint |
| 23P01 | 409 | Exclusion Constraint (Overlap) |
| P0001 | 422 | Raise Exception (Trigger) |
