# Teilplan 7: Compliance

## Voraussetzungen
Teilplan 6 (Betrieb + Arbeitsstunden) abgeschlossen.

## Ziel
Verstoesse dokumentieren, Mahnwesen (automatisch + Freigabe),
Abmahnungen, Kuendigungen. PDF-Generierung. Eskalationskette.

## Methodik: TDD
Jede Funktion wird test-first entwickelt. Kein Code ohne vorherigen Test.

## Scope

### Tabellen
- legal_provisions
- incidents, incident_witnesses, incident_consequences
- dunning_notices, warnings, terminations

## Aufgaben

### 1. Legal Provisions
- TEST: provision_repo_test.go - CRUD, FindBySource, Seeds vorhanden (testcontainers)
- TEST: provision_test.go - 12 Seed-Eintraege korrekt
- TEST: handler_test.go - CRUD Endpunkte
- IMPL: model.go, repository, service, handler, routes

### 2. Incidents - Repository
- TEST: incident_repo_test.go - Create, FindByUser, FindByUnit, FindByStatus (testcontainers)
- TEST: witness_repo_test.go - Create intern (user_id) + extern (name+kontakt)
- TEST: consequence_repo_test.go - Verknuepfung Incident -> Warning/Dunning
- IMPL: model.go, repository.go, repository_pg.go

### 3. Incidents - Service
- TEST: service_test.go - Incident anlegen mit Zeugen + Rechtsgrundlage
- TEST: service_test.go - Beweisdokumente via document_relations verknuepft
- TEST: service_test.go - Status-Workflow: reported->investigating->confirmed->resolved
- TEST: service_test.go - Ungueltige Uebergaenge -> Fehler
- TEST: service_test.go - Hooks: incident.after_create, status.after_change
- IMPL: service.go
- KI-HOOK: incident.create.validate mit Noop registrieren (Bewertung spaeter in TP9)
- TEST: hook_test.go - incident.create.validate gefeuert, Noop -> ai_* Felder bleiben nil

### 4. Incidents - Handler
- TEST: handler_test.go - POST /incidents mit Zeugen -> 201
- TEST: handler_test.go - POST /incidents/:id/witnesses -> Zeuge hinzugefuegt
- TEST: handler_test.go - POST /incidents/:id/consequences -> Verknuepfung erstellt
- TEST: handler_test.go - GET /users/:id/incidents, GET /units/:id/incidents
- IMPL: dto.go, handler.go, routes.go

### 5. Dunning - Service (Auto-Draft + Freigabe)
- TEST: dunning_service_test.go - Auto-Draft aus offenem Posten: Status 'draft'
- TEST: dunning_service_test.go - Mahngebuehr korrekt pro Stufe
- TEST: dunning_service_test.go - Freigabe: draft -> approved -> issued
- TEST: dunning_service_test.go - Ablehnung: draft -> cancelled
- TEST: dunning_service_test.go - Eskalation: Stufe 1 -> 2 -> 3
- TEST: dunning_service_test.go - Stufe 3 abgelaufen -> Status 'escalated'
- TEST: dunning_service_test.go - Mahngebuehr in accounting_journal gebucht
- IMPL: dunning_service.go

### 6. Dunning - PDF + Handler
- TEST: dunning_pdf_test.go - PDF generiert mit korrekten Daten (Stufe, Betrag, Frist)
- TEST: handler_test.go - CRUD, Approve, PDF-Download
- TEST: handler_test.go - Nur Vorstand kann approven (Permission)
- IMPL: handler.go (Dunning-Endpunkte)
- KI-HOOK: compliance.dunning.ai_draft mit Noop (Korrespondenz spaeter in TP9)
- TEST: hook_test.go - ai_draft gefeuert, Noop -> kein generierter Text

### 7. Dunning - Escalation Job
- TEST: escalation_job_test.go - Frist abgelaufen + nicht bezahlt -> Stufe hoch
- TEST: escalation_job_test.go - Bereits bezahlt -> keine Eskalation
- TEST: escalation_job_test.go - Stufe 3 abgelaufen -> 'escalated'
- IMPL: escalation_job.go

### 8. Warnings - Service + Handler
- TEST: warning_service_test.go - Warning aus Incident erstellen
- TEST: warning_service_test.go - Freigabe: draft->approved->issued->acknowledged
- TEST: warning_service_test.go - Stellungnahme: response_received_at gesetzt
- TEST: warning_service_test.go - Gueltigkeit: valid_until abgelaufen -> 'expired'
- TEST: warning_service_test.go - Rechtsgrundlage verknuepft
- TEST: warning_pdf_test.go - PDF mit korrekten Daten
- TEST: handler_test.go - CRUD, Approve, PDF
- IMPL: warning_service.go, handler.go

### 9. Warnings - Expiry Job
- TEST: expiry_job_test.go - valid_until < today -> Status 'expired'
- TEST: expiry_job_test.go - Noch gueltige -> unveraendert
- IMPL: expiry_job.go

### 10. Terminations - Service + Handler
- TEST: termination_service_test.go - Typen: ordentlich, ausserordentlich, fristlos
- TEST: termination_service_test.go - Fristen korrekt (notice, effective, clearance, objection)
- TEST: termination_service_test.go - Vorstandsbeschluss erforderlich
- TEST: termination_service_test.go - Widerspruch: objection_received_at gesetzt
- TEST: termination_service_test.go - Freigabe: draft->approved->issued->enforced
- TEST: termination_service_test.go - Lease verknuepft -> Lease beenden bei 'enforced'
- TEST: termination_pdf_test.go - PDF mit korrekten Daten
- TEST: handler_test.go - CRUD, Approve, PDF
- IMPL: termination_service.go, handler.go

### 11. Cross-Module Hooks
- TEST: hooks_test.go - billing.after_approve -> offene Posten -> Mahnung draft
- TEST: hooks_test.go - work_event_participant.no_show -> Incident vorgeschlagen
- TEST: hooks_test.go - workhours.deadline_passed -> Ersatzgeld + Mahnung
- TEST: hooks_test.go - lease.ended -> Raeumungsfrist geprueft
- IMPL: hooks.go

### 12. Integration
- TEST: E2E - Verstoß -> Abmahnung -> 2. Verstoß -> Kuendigung -> Lease beendet

## Geschaetzte Sessions: 8-10
