# Wiederverwendungs-Analyse

## Quelle 1: Ticketmanager (Go/Gin/GORM/SQLite)

Pfad: ~/Development/ticketmanager/ticketmanager-app/

### Direkt wiederverwendbar (Copy + Adapt)

| Komponente | Quelldatei | Aufwand | Anpassung |
|-----------|-----------|---------|-----------|
| EAV Metadata | utils/metadata/metadata.go (252 Z.) | Gering | GORM -> pgx/sqlc. Generisches Key-Value fuer dynamische Eigenschaften auf beliebigen Entities. Wird Plugin-Erweiterungsmechanismus. |
| Env-Config | utils/envutil/env.go (47 Z.) | Copy as-is | .env Loading, Type-Conversion, Default-Werte. |
| Duration Parser | utils/timeutil/duration.go (33 Z.) | Copy as-is | Erweiterte time.ParseDuration mit Tagen (d) und Monaten (M). Nuetzlich fuer Lease-Laufzeiten. |
| Pagination | controller/common.go (55 Z.) | Copy as-is | Generische Paginierung (page, per_page). |
| User Service | services/user_service.go (86 Z.) | Gering | Passwort-Hashing (bcrypt cost 14), User-Erstellung, Validierung. |
| Email Service | services/email_service.go (185 Z.) | Mittel | SMTP-Versand. Templates anpassen fuer Vereinskommunikation. ICS-Kalender-Funktion evtl. nuetzlich fuer Vereinstermine. |
| QR-Code Lib | Dependency: yeqown/go-qrcode/v2 | Gering | Bibliothek direkt uebernehmen. QR-Generierung fuer Sharing-Modul. |
| Cleanup Jobs Pattern | jobs/cleanup_jobs.go (74 Z.) | Adapt | Scheduled Job Pattern (periodische Aufgaben). Konzept uebernehmen fuer river Scheduled Jobs: OverdueLendingCheck, ExpiredLinksCleanup, WebhookRetry. |

### Muster uebernehmen (Pattern, nicht Code)

| Muster | Quelldatei | Was uebernehmen |
|--------|-----------|-----------------|
| Capability-based RBAC | middleware/auth_middleware.go (238 Z.) | JWT-Validierung + Capability-Check Konzept. Code komplett refactoren: GORM->pgx, dgrijalva/jwt->golang-jwt/v5, Authorize() Pattern behalten. |
| User/Role/Capability Model | models/user.go (280 Z.) | Rollen/Capability-Konzept passt 1:1 auf unser roles/permissions Schema. GORM Tags -> sqlc Queries. InitRoles/InitCapabilities Pattern fuer Seed-Daten uebernehmen. |
| SQL Views | migrations/create_views.go (426 Z.) | Pattern: Views als Go-Funktionen definieren. 9 Views als Vorlage. Fuer Vereinsverwaltung: view_revision_report, Dashboard-Views. |
| Config-as-JSON | config/ticket_types.json, roles_and_capabilities.json | Rollen/Permissions und Seed-Daten als JSON-Konfiguration laden. |
| Demo Generator | models/demo_generator.go (1254 Z.) | Konzept: Demo-Modus mit Seed-Daten + periodischem Reset. Inhalt komplett neu schreiben fuer Vereins-Domain. |
| Multi-Domain Config | config/frontend_config_loader.go (43 Z.) | APP_DOMAIN basierte Konfiguration. Nuetzlich wenn ein Backend mehrere Vereine bedient. |

### Nicht uebernehmen

| Komponente | Grund |
|-----------|-------|
| Ticket-Booking/Invitation Logik | Domain-spezifisch fuer Event-Ticketing |
| Coin-System (ticket_booking_coins) | Nicht relevant |
| HundepartnerEventsController | Code-Duplikat, Anti-Pattern |
| GORM ORM | Wird durch pgx/sqlc ersetzt |
| SQLite | Wird durch PostgreSQL ersetzt |
| main.go God-File (816 Z.) | Anti-Pattern, wird in saubere Struktur aufgeteilt |
| modules/auth/service/auth_service.go | Leerer Stub |
| models/user copy.go | Dateiname mit Leerzeichen, Stub |

### Refactoring-Aufwand

| Problem | Beschreibung |
|---------|-------------|
| main.go (816 Z.) | Aufteilen in cmd/api/main.go, core/server, core/database |
| Business-Logik in Controllern | Extrahieren in Service-Layer |
| Kein Repository-Pattern | Repository-Interfaces einfuehren |
| Kein DI | Constructor Injection in main.go |
| GORM -> pgx/sqlc | Kompletter Umbau der DB-Schicht |
| SQLite -> PostgreSQL | SQLite-spezifische Funktionen ersetzen |
| Kein Hook/Plugin System | Komplett neu implementieren |
| Minimale Tests (2 Dateien) | Test-Coverage massiv erhoehen |

## Quelle 2: Haushaltsbuch (Python/FastAPI/SQLAlchemy/SQLite)

Pfad: ~/Development/tmp/Haushaltsbuch/

### Konzepte uebernehmen (portiert nach Go)

| Konzept | Quelldatei | Go-Adaption |
|---------|-----------|-------------|
| CSV Parser Registry | services/csv_parser.py (172 Z.) | map[string]ParserFunc Interface. detect_format() + bank-spezifische Parser. |
| Format-Erkennung | csv_parser.py:39-52 | Keyword-Scan in ersten 2000 Bytes. "buchungsdatum"->DKB, "buchungstag"+"auftraggeber"->Sparkasse. |
| IBAN-Extraktion | csv_parser.py:55-72 | Regex auf erste 15 Zeilen: DE + 20 Ziffern. |
| IBAN-Maskierung | csv_parser.py:68-72 | DE54...655101 Format. |
| Spalten-Mapping | csv_parser.py:101-117 | Keyword-basiert: "betrag"->Amount, "empfaenger"->Payee. Flexibel, nicht hartcodiert. |
| German Float Parsing | csv_parser.py:23-27 | "1.234,56" -> 1234.56 via strings.Replace + strconv.ParseFloat |
| Datumsformate | csv_parser.py:29-36 | Fallback-Kette: dd.mm.yyyy, dd.mm.yy, yyyy-mm-dd |
| Duplikat-Erkennung | routers/imports.py:29-39 | Exact-Match auf datum+betrag+empfaenger+account_id |
| Auto-Kategorisierung | services/categorizer.py (146 Z.) | Regex-Rules mit Prioritaet. Hoehere Prio gewinnt. Fallback: Substring-Match. |
| Zwei-Stufen-Import | routers/imports.py | Preview (kein DB-Write) -> Execute (Persist). |
| Account Auto-Erstellung | routers/imports.py:18-26 | IBAN nicht vorhanden -> neues Konto anlegen. |

### Verbesserungen fuer Go-Version

| Bereich | Original (Python) | Verbesserung (Go) |
|---------|-------------------|-------------------|
| Insert-Performance | Einzelinserts pro Zeile | Batch-Insert (Multi-Row INSERT oder COPY) |
| Duplikat-Erkennung | Nur Exact-Match auf 4 Felder | + SHA256 Hash pro Transaktion als zusaetzlicher Check |
| Import-History | Nicht vorhanden | bank_import_logs Tabelle (wer, wann, welche Datei, wie viele Zeilen) |
| Preview | db.rollback() nach Test-Insert | Kein DB-Schreiben beim Preview, nur in-memory Verarbeitung |
| Betraege | Python float | shopspring/decimal (exakte Dezimalrechnung) |
| Bankformate | Nur CSV (DKB, Sparkasse, Generisch) | CSV + spaeter CAMT.053/MT940 (Backlog) |
| Accounting-Verknuepfung | Nicht vorhanden | Importierte Transaktionen -> automatisch accounting_journal Entries |
| Kategorisierung | Tier 1: Rules, Tier 2: Payee-Frequenz | Beides uebernehmen + Hook banking.import.categorize fuer Plugin-Erweiterung |

### Datenmodell-Mapping (Haushaltsbuch -> Vereinsverwaltung)

| Haushaltsbuch Tabelle | Vereinsverwaltung Tabelle |
|----------------------|--------------------------|
| accounts (iban, name) | bank_accounts (iban_masked, name, note) |
| transactions | bank_transactions (+ checksum_sha256, + journal_id FK) |
| categories | Nutzt bestehende accounting_accounts oder eigene bank_categories |
| categorization_rules | bank_categorization_rules (pattern, field, category_id, priority) |
| (nicht vorhanden) | bank_import_logs (filename, format, iban, count, created_by) |
