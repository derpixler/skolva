# Teilplan 3: Property + Pacht

## Voraussetzungen
Teilplan 2 (Identity + CRM + Groups) abgeschlossen.

## Ziel
Verwaltung von Pachteinheiten, Eigentum, Mietvertraege und Warteliste.

## Methodik: TDD
Jede Funktion wird test-first entwickelt. Kein Code ohne vorherigen Test.

## Scope

### Tabellen
- units, garages, parcels
- unit_ownerships
- leases
- applicants

### Module
```
internal/property/       Units + Typ-Tabellen (Garages, Parcels)
internal/ownership/      Eigentumsverhaeltnisse
internal/leasing/        Mietvertraege + Status-Workflow
internal/applicants/     Warteliste / Bewerbermanager
```

## Aufgaben

### 1. Property - Repository
- TEST: unit_repo_test.go - Create mit unit_type, FindByID, FindByType, SoftDelete (testcontainers)
- TEST: garage_repo_test.go - Create mit unit_id FK, Unique nr, meter_id (testcontainers)
- TEST: parcel_repo_test.go - Create, total_area_sqm korrekt berechnet (testcontainers)
- IMPL: model.go, repository.go, repository_pg.go, sqlc Queries

### 2. Property - Service + Handler
- TEST: service_test.go - Unit + Garage/Parcel zusammen anlegen (je nach unit_type)
- TEST: handler_test.go - POST /units -> Unit + Typ-Details in einer Transaktion
- TEST: handler_test.go - GET /units/:id/details -> Garage ODER Parcel Details
- TEST: handler_test.go - PUT /units/:id/details -> Update Typ-Details
- TEST: handler_test.go - Volltextsuche, Pagination, SoftDelete
- IMPL: dto.go, service.go, handler.go, routes.go

### 3. Ownership - Repository
- TEST: ownership_repo_test.go - Create, FindByUnit, Update, SoftDelete (testcontainers)
- TEST: ownership_repo_test.go - Overlap -> PG Exclusion Constraint Error 23P01
- TEST: ownership_repo_test.go - end_date >= start_date Constraint
- IMPL: model.go, repository.go, repository_pg.go

### 4. Ownership - Service + Handler
- TEST: service_test.go - Overlap-Fehler -> 409 mit Klartextmeldung
- TEST: handler_test.go - CRUD, Zeitraum-Validierung, Permission-Check
- IMPL: dto.go, service.go, handler.go, routes.go

### 5. Leasing - Repository
- TEST: lease_repo_test.go - Create, FindByUnit, FindByTenant, SoftDelete (testcontainers)
- TEST: lease_repo_test.go - Overlap -> 23P01, end_date Constraint
- IMPL: model.go, repository.go, repository_pg.go

### 6. Leasing - Service (Status-Workflow)
- TEST: service_test.go - Status planned -> active: erlaubt
- TEST: service_test.go - Status active -> ended: erlaubt
- TEST: service_test.go - Status ended -> active: verboten -> Fehler
- TEST: service_test.go - Status planned -> ended: verboten -> Fehler
- TEST: service_test.go - Overlap -> 409 Conflict mit Klartext
- TEST: service_test.go - Hooks: before_change + after_change gefeuert
- IMPL: service.go

### 7. Leasing - Handler
- TEST: handler_test.go - POST /units/:id/leases -> Lease anlegen
- TEST: handler_test.go - POST /leases/:id/status -> Status-Wechsel
- TEST: handler_test.go - Ungueltige Uebergaenge -> 422
- TEST: handler_test.go - Permission-Check
- IMPL: dto.go, handler.go, routes.go

### 8. Applicants - Repository + Service
- TEST: applicant_repo_test.go - CRUD, FindByStatus (testcontainers)
- TEST: service_test.go - Status-Workflow: waiting -> offered -> accepted/rejected/withdrawn
- TEST: service_test.go - Ungueltige Uebergaenge -> Fehler
- TEST: service_test.go - Assign: Lease wird automatisch erstellt
- IMPL: model.go, dto.go, repository.go, repository_pg.go, service.go

### 9. Applicants - Handler
- TEST: handler_test.go - CRUD, Assign-Endpunkt, Status-Aenderung
- TEST: handler_test.go - POST /applicants/:id/assign -> Lease in DB pruefen
- IMPL: handler.go, routes.go

### 10. Integration
- TEST: E2E - Unit anlegen -> Eigentuemer zuweisen -> Lease erstellen -> Bewerber zuweisen

## API-Endpunkte nach Abschluss

```
GET/POST        /api/units
GET/PATCH/DEL   /api/units/:id
GET/PUT         /api/units/:id/details
GET/POST        /api/units/:id/ownerships
PATCH           /api/ownerships/:id
GET/POST        /api/units/:id/leases
GET/PATCH       /api/leases/:id
POST            /api/leases/:id/status
GET/POST        /api/applicants
GET/PATCH       /api/applicants/:id
POST            /api/applicants/:id/assign
```

## Geschaetzte Sessions: 5-6
