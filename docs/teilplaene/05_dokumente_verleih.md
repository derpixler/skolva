# Teilplan 5: Dokumente + Zaehler + QR + Verleih

## Voraussetzungen
Teilplan 4 (Finanzen) abgeschlossen.

## Ziel
Dokumentenmanagement, Zaehlerstaende, QR-Code-Aushaenge und
Geraeteverleih (inkl. Bulk-Artikel und Gruppen-Ausleihe).

## Methodik: TDD
Jede Funktion wird test-first entwickelt. Kein Code ohne vorherigen Test.

## Scope

### Tabellen
- documents, document_categories, document_relations
- meter_readings
- shared_links
- lendable_items, lending_records

### Neue Core-Packages
```
internal/core/storage/   FileStorage Interface + Local + S3 (erstmals hier: Dokument-Upload)
```

## Aufgaben

### 0. Core: Storage (erstmals hier gebraucht)
- TEST: local_test.go - Upload, Download, Delete, URL Generierung
- TEST: s3_test.go - S3 Interface gegen MinIO (testcontainers)
- IMPL: core/storage/interface.go, local.go, s3.go

### 1. Documents - Repository
- TEST: doc_repo_test.go - Create, FindByID, SoftDelete, Volltextsuche (testcontainers)
- TEST: relation_repo_test.go - Create Relation, FindByTarget, FindByDocument
- TEST: category_repo_test.go - Seeds vorhanden (8 Kategorien)
- IMPL: model.go, repository.go, repository_pg.go

### 2. Documents - Service (Upload)
- TEST: service_test.go - Upload berechnet SHA256, speichert via Storage Interface
- TEST: service_test.go - Download gibt korrekten Reader zurueck
- TEST: service_test.go - Relate erstellt document_relation korrekt
- IMPL: service.go

### 3. Documents - Handler
- TEST: handler_test.go - POST /documents Multipart Upload -> 201 + SHA256
- TEST: handler_test.go - GET /documents/:id/download -> Dateiinhalt
- TEST: handler_test.go - POST /documents/:id/relate -> Relation erstellt
- TEST: handler_test.go - GET /document-categories -> 8 Seeds
- TEST: handler_test.go - GET /search/documents?q= -> Volltextsuche
- IMPL: dto.go, handler.go, routes.go
- KI-HOOK: document.after_upload mit Noop registrieren (OCR spaeter in TP9)
- TEST: hook_test.go - document.after_upload Hook wird gefeuert, Noop -> kein Effekt

### 4. Metering - Repository + Service
- TEST: reading_repo_test.go - Create, FindByUnit, Unique pro Tag+Unit (testcontainers)
- TEST: reading_repo_test.go - Duplikat am selben Tag -> Constraint Error
- TEST: handler_test.go - CRUD, Permission-Check
- IMPL: model.go, dto.go, repository.go, repository_pg.go, service.go, handler.go, routes.go
- KI-HOOK: meter_reading.after_create mit Noop registrieren (Anomalie spaeter in TP9)
- TEST: hook_test.go - meter_reading.after_create Hook wird gefeuert, Noop -> kein Effekt

### 5. Sharing - Service
- TEST: service_test.go - Slug generiert (eindeutig, URL-safe)
- TEST: service_test.go - Visit-Tracking: visit_count incrementiert
- TEST: service_test.go - Abgelaufen: expires_at < now -> Fehler
- TEST: service_test.go - Max Visits erreicht -> Fehler
- TEST: service_test.go - PIN korrekt -> Zugang, PIN falsch -> Fehler
- IMPL: model.go, dto.go, repository.go, repository_pg.go, service.go

### 6. Sharing - QR + Handler
- TEST: qr_test.go - QR generiert als PNG, enthaelt korrekte URL
- TEST: handler_test.go - POST /shared-links -> Link + Slug erstellt
- TEST: handler_test.go - GET /shared-links/:id/qr -> PNG Bytes
- TEST: handler_test.go - GET /p/:slug (public=true) -> 200 ohne Auth
- TEST: handler_test.go - GET /p/:slug (public=false) -> 401 ohne Auth
- TEST: handler_test.go - GET /p/:slug (expired) -> 410 Gone
- IMPL: qr.go, handler.go, routes.go

### 7. Lending - Repository
- TEST: item_repo_test.go - Create trackable + bulk, FindByCategory (testcontainers)
- TEST: item_repo_test.go - Volltextsuche auf name/description
- TEST: record_repo_test.go - Create mit user_id, Create mit group_id (testcontainers)
- TEST: record_repo_test.go - Polymorphe Constraint: user XOR group, nie beides
- IMPL: model.go, repository.go, repository_pg.go

### 8. Lending - Service (Bulk + Gruppen)
- TEST: service_test.go - Trackable: 1:1 Ausleihe, Quantity=1
- TEST: service_test.go - Bulk: 40 von 100 ausleihen -> stock_available=60
- TEST: service_test.go - Bulk: Rueckgabe -> stock_available erhoeht
- TEST: service_test.go - Bulk: Mehr ausleihen als verfuegbar -> Fehler
- TEST: service_test.go - Gruppen-Ausleihe: borrower_group_id statt user_id
- TEST: service_test.go - Status-Workflow: reserved -> checked_out -> returned
- TEST: service_test.go - Hooks: after_checkout, after_return gefeuert
- IMPL: service.go

### 9. Lending - Overdue Job + Handler
- TEST: overdue_job_test.go - reserved_until < today -> Status 'overdue' (testcontainers)
- TEST: overdue_job_test.go - Sammel-Benachrichtigung pro Team (nicht 50 Einzel-Mails)
- TEST: handler_test.go - CRUD Items, CRUD Records, Checkout, Return, Cancel
- IMPL: overdue_job.go, dto.go, handler.go, routes.go

### 10. Integration
- TEST: E2E - Dokument hochladen -> an Unit relaten -> QR erstellen -> oeffentlich abrufen
- TEST: E2E - Bulk-Item anlegen -> Team-Ausleihe -> Rueckgabe -> Stock korrekt

## Geschaetzte Sessions: 6-7
