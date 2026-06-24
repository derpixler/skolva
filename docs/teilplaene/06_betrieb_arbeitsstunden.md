# Teilplan 6: Betrieb + Arbeitsstunden

## Voraussetzungen
Teilplan 5 (Dokumente + Verleih) abgeschlossen.

## Ziel
Inventar, Kautionen, Arbeitsstunden-Tracking, Spesen und der
Arbeitseinsatzplaner mit Soll/Ist-Berechnung und Gruppen-Einladung.

## Methodik: TDD
Jede Funktion wird test-first entwickelt. Kein Code ohne vorherigen Test.

## Scope

### Tabellen
- inventory_items, deposits, work_logs, expense_claims
- work_hour_requirements, unit_work_hour_obligations, user_work_hour_adjustments
- work_task_catalog, work_events, work_event_tasks, work_event_participants

## Aufgaben

### 1. Operations - Repository
- TEST: inventory_repo_test.go - CRUD, FindByUnit (testcontainers)
- TEST: deposit_repo_test.go - CRUD, FindByLease, Status (testcontainers)
- TEST: worklog_repo_test.go - CRUD, FindByUser, FindByYear (testcontainers)
- TEST: expense_repo_test.go - CRUD, FindByStatus (testcontainers)
- IMPL: model.go, repository.go, repository_pg.go

### 2. Operations - Service + Handler
- TEST: service_test.go - WorkLog Verify: is_verified=true, verified_by gesetzt
- TEST: service_test.go - ExpenseClaim Workflow: submitted->approved->paid
- TEST: service_test.go - ExpenseClaim ungueltige Uebergaenge -> Fehler
- TEST: service_test.go - Deposit Workflow: held->returned/offset/cancelled
- TEST: service_test.go - Deposit + Journal Verknuepfung
- TEST: handler_test.go - Alle CRUD Endpunkte + Status-Aenderungen
- IMPL: dto.go, service.go, handler.go, routes.go

### 3. Workhours - Pflichtstunden Repository
- TEST: requirement_repo_test.go - CRUD pro Jahr, Unique Year (testcontainers)
- TEST: obligation_repo_test.go - CRUD pro Unit+Jahr, Unique (testcontainers)
- TEST: adjustment_repo_test.go - CRUD pro User+Jahr, Unique (testcontainers)
- IMPL: model.go, repository.go, repository_pg.go

### 4. Workhours - Balance (Soll/Ist)
- TEST: balance_test.go - Nur Basis: 6h Global, 0 gearbeitet -> 6h fehlend
- TEST: balance_test.go - Basis + Unit: 6h + 4h Vorgarten -> 10h Pflicht
- TEST: balance_test.go - Basis + Unit + Adjustment: 6+4-2 (Alter >70) -> 8h
- TEST: balance_test.go - 8h Pflicht, 5h gearbeitet -> 3h fehlend, 45 EUR Ersatzgeld
- TEST: balance_test.go - Nur verifizierte work_logs zaehlen
- TEST: balance_test.go - Unit-Obligation wandert mit Lease zum neuen Paechter
- IMPL: balance.go (view_work_hour_balance Query)

### 5. Workhours - Pflichtstunden Handler
- TEST: handler_test.go - Requirements PUT/GET pro Jahr
- TEST: handler_test.go - Unit Obligations PUT/GET
- TEST: handler_test.go - User Adjustments PUT/GET
- TEST: handler_test.go - GET /balance?user_id=&year= -> korrektes Soll/Ist
- IMPL: dto.go, handler.go, routes.go

### 6. Workhours - Aufgabenkatalog
- TEST: catalog_repo_test.go - CRUD, FindBySeason, Seeds vorhanden (testcontainers)
- TEST: catalog_test.go - 12 Seed-Aufgaben korrekt angelegt
- TEST: handler_test.go - CRUD + Volltextsuche
- IMPL: repository, service, handler fuer work_task_catalog

### 7. Workhours - Events + Tasks
- TEST: event_repo_test.go - Create, FindByDate, FindByStatus (testcontainers)
- TEST: task_repo_test.go - Create mit catalog_task_id FK
- TEST: service_test.go - Event Status-Workflow: draft->planned->invitation_sent->completed
- TEST: service_test.go - Ungueltige Uebergaenge -> Fehler
- IMPL: Work Events + Tasks CRUD

### 8. Workhours - Teilnehmer + Gruppen-Einladung
- TEST: participant_test.go - Einzel-Einladung: User hinzufuegen, Status invited
- TEST: participant_test.go - Gruppen-Einladung: group_id -> alle Members als Teilnehmer
- TEST: participant_test.go - Status accepted -> attended -> work_log automatisch erzeugt
- TEST: participant_test.go - Status no_show -> Hook gefeuert
- TEST: participant_test.go - hours_credited korrekt in work_log
- IMPL: service.go (Einladung, Status-Handling, auto work_log)

### 9. Workhours - Planner + Vorschlaege
- TEST: planner_test.go - Teilnehmer-Vorschlag sortiert nach Reststunden (meiste zuerst)
- TEST: planner_test.go - Befreite Mitglieder werden nicht vorgeschlagen
- IMPL: planner.go
- KI-HOOK: workhours.event.ai_suggest mit Noop registrieren (KI spaeter in TP9)
- TEST: hook_test.go - ai_suggest Hook gefeuert, Noop -> suggestion_reason bleibt nil

### 10. Workhours - Deadline Job
- TEST: deadline_job_test.go - Jahresende: fehlende Stunden -> billing_item Ersatzgeld
- TEST: deadline_job_test.go - Bereits abgerechnete Jahre -> kein Duplikat
- IMPL: deadline_job.go

### 11. Workhours - Events Handler
- TEST: handler_test.go - Event CRUD, Invite (Einzel + Gruppe), Complete
- TEST: handler_test.go - Participant Status aendern, AI-Suggest Endpunkt
- IMPL: handler.go, routes.go

### 12. Integration
- TEST: E2E - Requirement -> Obligation -> Event -> Einladung -> Attended -> work_log -> Balance

## Geschaetzte Sessions: 8-10
