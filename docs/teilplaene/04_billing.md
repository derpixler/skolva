# Teilplan 4: Billing + PDF + Admin + Snapshots

## Voraussetzungen
Teilplan 3 (Property + Pacht) abgeschlossen.

## Ziel
Jahresabrechnungen berechnen, als PDF versenden. Admin-Dashboard fuer
Job-Ueberwachung. Billing-Snapshots als Absicherung fuer spaetere
Accounting-Migration. Buchhaltung und Banking folgen in Teilplan 8.

## Methodik: TDD
Jede Funktion wird test-first entwickelt. Kein Code ohne vorherigen Test.

## Scope

### Tabellen
- billing_periods, billing_items, billing_snapshots

### Module + neue Core-Packages
```
internal/billing/           Rechnungsstellung
internal/core/pdf/          HTML -> PDF Engine (erstmals hier gebraucht)
internal/core/admin/        Job-Dashboard API (erstmals echte Jobs)
```

## Aufgaben

### 1. Core: PDF Engine
- TEST: engine_test.go - HTML Template -> PDF Bytes (chromedp)
- TEST: engine_test.go - Template mit Variablen (Name, Betraege, Datum)
- IMPL: core/pdf/engine.go

### 2. Core: Admin Dashboard
- TEST: handler_test.go - GET /admin/jobs -> Liste (testcontainers + river)
- TEST: handler_test.go - GET /admin/jobs/:id -> Details + Fehler uebersetzt
- TEST: handler_test.go - POST /admin/jobs/:id/retry -> Job wiederholt
- TEST: handler_test.go - POST /admin/jobs/:id/cancel -> Job abgebrochen
- TEST: handler_test.go - GET /admin/jobs/stats -> Counts
- TEST: handler_test.go - GET /admin/health -> DB+Storage Status
- TEST: translator_test.go - PG Fehler -> Klartext (23P01 -> "Ueberschneidung")
- IMPL: core/admin/handler.go, translator.go, routes.go

### 3. Billing - Repository
- TEST: period_repo_test.go - Create, FindByYear, FindByStatus (testcontainers)
- TEST: item_repo_test.go - Create, FindByPeriod, FindByLease (testcontainers)
- TEST: snapshot_repo_test.go - Create, FindByPeriod (testcontainers)
- IMPL: model.go, repository.go, repository_pg.go, sqlc Queries

### 4. Billing - Calculator
- TEST: calculator_test.go - Pacht aus Lease-Daten
- TEST: calculator_test.go - Strom: (aktueller - vorheriger Zaehlerstand) * Preis
- TEST: calculator_test.go - Wasser: analog
- TEST: calculator_test.go - Umlagen pro-rata nach Flaeche
- TEST: calculator_test.go - Arbeitsstunden-Ersatzgeld (view_work_hour_balance)
- TEST: calculator_test.go - Leihgebuehren (lending_records.fee_charged)
- TEST: calculator_test.go - Keine Lease -> keine Items
- TEST: calculator_test.go - Hook billing.line_item.calculate Filter aendert Betrag
- IMPL: calculator.go

### 5. Billing - Service (Workflow + Snapshot)
- TEST: service_test.go - Workflow: draft -> calculated -> approved -> sent
- TEST: service_test.go - Calculate erzeugt Items fuer alle aktiven Leases
- TEST: service_test.go - Approve feuert Hook billing.period.after_approve
- TEST: service_test.go - Approve schreibt billing_snapshot (JSON mit Posten+Betraegen)
- TEST: service_test.go - Snapshot ist unveraenderlich (kein Update/Delete)
- TEST: service_test.go - Ungueltige Uebergaenge -> Fehler
- TEST: service_test.go - Doppelte Berechnung -> alte Items ersetzt
- IMPL: service.go

### 6. Billing - PDF
- TEST: pdf_job_test.go - PDF enthaelt Paechter, Parzelle, Posten, Summe
- TEST: pdf_job_test.go - Async via river Job
- TEST: pdf_job_test.go - PDF als Document gespeichert + document_relation
- IMPL: pdf_job.go

### 7. Billing - Handler
- TEST: handler_test.go - GET /billing/periods, POST, Calculate, Approve
- TEST: handler_test.go - GET /periods/:id/items, /periods/:id/pdf
- TEST: handler_test.go - Permission-Check (billing.read/write/approve)
- IMPL: dto.go, handler.go, routes.go

### 8. Integration
- TEST: E2E - Lease -> Zaehlerstand -> Periode -> Calculate -> Approve -> Snapshot + PDF
- TEST: Snapshot enthaelt korrekte Daten (Betraege, Posten, Kontierungshinweise)

## API-Endpunkte nach Abschluss
```
GET    /api/admin/jobs
GET    /api/admin/jobs/:id
POST   /api/admin/jobs/:id/retry
POST   /api/admin/jobs/:id/cancel
GET    /api/admin/jobs/stats
GET    /api/admin/health

GET    /api/billing/periods
POST   /api/billing/periods
GET    /api/billing/periods/:id
POST   /api/billing/periods/:id/calculate
POST   /api/billing/periods/:id/approve
GET    /api/billing/periods/:id/items
GET    /api/billing/periods/:id/pdf
```

## Geschaetzte Sessions: 5-6
