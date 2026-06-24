# Teilplan 10: Qualitaet + Deployment

## Voraussetzungen
Alle Teilplaene 1-9 abgeschlossen.
CI/CD Pipeline laeuft seit Teilplan 1.

## Ziel
Produktionsreife: Coverage-Luecken schliessen, DSGVO finalisieren,
Metadata API, Performance-Tests, Plugin-System. Seed-basierte isolierte
Tests statt monolithischer E2E-Ketten.

## Methodik: TDD
Tests zuerst. Go-Fixtures mit Factory-Pattern fuer isolierte State-Tests.

## Test-Strategie: Go-Fixtures statt Domino-E2E

Kein monolithischer 15-Schritte-Test. Stattdessen:

```go
// testutil/fixtures/factory.go
type TestState struct {
    DB     *pgxpool.Pool
    Admin  *User
    Member *User
    Unit   *Unit
    Lease  *Lease
    Period *BillingPeriod
}

func NewBaseState(t *testing.T, db *pgxpool.Pool) *TestState { ... }
func (s *TestState) WithProperty(t *testing.T) *TestState { ... }
func (s *TestState) WithBilling(t *testing.T) *TestState { ... }
func (s *TestState) WithCompliance(t *testing.T) *TestState { ... }
func (s *TestState) WithFullState(t *testing.T) *TestState { ... }
```

Jeder Test laedt nur den State den er braucht:
```go
func TestDunningEscalation(t *testing.T) {
    state := fixtures.NewBaseState(t, db).
        WithProperty(t).
        WithBilling(t).
        WithCompliance(t)  // Incident + 1. Abmahnung + offene Mahnung
    // Teste NUR Eskalation, nicht den ganzen Weg dahin
}
```

## Aufgaben

### 1. Go-Fixtures Factory
- TEST: factory_test.go - NewBaseState erzeugt Users + Rollen (testcontainers)
- TEST: factory_test.go - WithProperty erzeugt Units + Leases
- TEST: factory_test.go - WithBilling erzeugt Perioden + Items + Snapshots
- TEST: factory_test.go - WithCompliance erzeugt Incidents + Warnings + Dunning
- TEST: factory_test.go - WithFullState erzeugt alles zusammen
- TEST: factory_test.go - Jeder State ist isoliert (kein Bleed zwischen Tests)
- IMPL: testutil/fixtures/factory.go

### 2. Audit-Verifizierung
- TEST: audit_trigger_test.go - INSERT/UPDATE auf allen Tabellen -> audit_log
- TEST: audit_trigger_test.go - Soft-Delete -> action='DELETE'
- TEST: audit_trigger_test.go - Status-Change -> action='STATUS_CHANGE'
- TEST: audit_trigger_test.go - Anonymisierung -> action='ANONYMIZE'
- TEST: audit_view_test.go - view_revision_report korrekt
- TEST: handler_test.go - GET /audit mit Filtern
- IMPL: Audit Handler + Routes

### 3. DSGVO
- TEST: anonymize_test.go - POST /users/:id/anonymize -> Felder ueberschrieben
- TEST: anonymize_test.go - anonymized_at gesetzt, Meta bereinigt
- TEST: anonymize_test.go - Audit-Log mit Action='ANONYMIZE'
- TEST: anonymize_test.go - audit_logs: old_data/new_data PII-Felder maskiert ("[ANON]")
- TEST: anonymize_test.go - audit_logs anderer Tabellen mit user-Referenz ebenfalls gescrubbt
- TEST: data_export_test.go - GET /users/:id/data-export -> alle User-Daten
- IMPL: anonymize scrubbt auch audit_logs (JSONB PII-Felder entfernen/maskieren)
- IMPL: data-export Endpunkt

### 4. Metadata API
- TEST: meta_handler_test.go - GET/PUT/DELETE fuer alle 11 Entity-Typen
- IMPL: Generischer Meta-Handler

### 5. Sicherheit
- TEST: ratelimit_test.go - Zu viele Requests -> 429
- TEST: upload_test.go - Datei > Limit -> 413, unerlaubter MIME -> 415
- IMPL: Rate Limiting, Upload-Limits

### 6. Coverage-Luecken (mit Go-Fixtures)
- Edge Cases: nil Inputs, leere Listen, Concurrent Writes
- Fehlende Fehlerpfade: 401, 403, 404, 409, 422
- Hook Chain Tests ueber mehrere Module
- Ziel: 100% Line Coverage

### 7. Performance (mit Go-Fixtures)
- TEST: perf_test.go - 1000 Bank-Importe in <10s
- TEST: perf_test.go - 200 Billing Items in <5s
- TEST: perf_test.go - 50 gleichzeitige Requests -> alle <200ms

### 8. Plugin-System
- TEST: plugin_loader_test.go - Plugin aus registry.go geladen
- TEST: plugin_lifecycle_test.go - Register -> Hook -> Deactivate
- TEST: example_plugin_test.go - Welcome-Mail Plugin E2E
- IMPL: plugins/registry.go, plugins/example-welcome-mail/plugin.go

### 9. Smoke Test (ein Happy Path)
- TEST: smoke_test.go - Register -> Login -> ZFA -> Unit -> Lease ->
  Billing -> Approve -> Snapshot -> Accounting Journal ->
  Bank-Import -> Arbeitsstunden -> Verstoß -> Abmahnung ->
  PDF -> Datenexport -> Anonymisierung
- Dieser Test nutzt WithFullState() als Basis, NICHT als 15-Schritte-Kette
- Dient als Regressionsschutz, nicht als primaere Validierung

## API-Endpunkte nach Abschluss
```
GET    /api/audit
POST   /api/users/:id/anonymize
GET    /api/users/:id/data-export
GET    /api/:entity/:id/meta
PUT    /api/:entity/:id/meta/:key
DELETE /api/:entity/:id/meta/:key
```

## Gesamtuebersicht

| # | Teilplan | Sessions | Kumuliert |
|---|----------|----------|-----------|
| 1 | Walking Skeleton + CI/CD | 3-4 | 3-4 |
| 2 | Identity + CRM + Groups + Mail + Search | 5-7 | 8-11 |
| 3 | Property + Pacht | 5-6 | 13-17 |
| 4 | Billing + PDF + Admin + Snapshots | 5-6 | 18-23 |
| 5 | Dokumente + Storage + Zaehler + QR + Verleih | 6-7 | 24-30 |
| 6 | Betrieb + Arbeitsstunden | 8-10 | 32-40 |
| 7 | Compliance | 8-10 | 40-50 |
| 8 | Accounting + Banking + Dry-Run | 6-8 | 46-58 |
| 9 | KI + Pseudonymizer + Webhooks | 4-6 | 50-64 |
| 10 | Qualitaet + Go-Fixtures + Smoke Test | 4-6 | 54-70 |
| **Gesamt** | | **54-70** | |

## Geschaetzte Sessions: 4-6
