# Vereinsverwaltung - Entwicklungsplan

## Positionierung

Open-Source Vereinssoftware fuer die Verwaltung von Pachteinheiten (Parzellen, Garagen, Stellplaetze).
Feature-Paritaet zu Gartenbund Pro. Self-hosted, Go + PostgreSQL, Plugin-basiert.
Generisch einsetzbar: Kleingartenverein, Garagengemeinschaft, Sportverein, Foerderverein.

## Technische Entscheidungen

| Bereich           | Entscheidung                              |
|-------------------|-------------------------------------------|
| Sprache           | Go                                        |
| HTTP Framework    | Gin                                       |
| Datenbank         | PostgreSQL 16                             |
| DB Driver         | pgx/v5                                    |
| SQL Codegen       | sqlc                                      |
| Auth              | JWT (golang-jwt/jwt/v5)                   |
| ZFA               | TOTP (pquerna/otp) + E-Mail OTP Fallback  |
| Passwort-Hashing  | bcrypt (golang.org/x/crypto)              |
| Validierung       | go-playground/validator/v10               |
| Config            | Env-Variablen + .env (godotenv)           |
| Logging           | uber-go/zap                               |
| Dezimalzahlen     | shopspring/decimal                        |
| QR-Codes          | yeqown/go-qrcode/v2                      |
| E-Mail            | gomail.v2                                 |
| PDF-Generierung   | HTML-Templates -> PDF (chromedp oder wkhtmltopdf) |
| File Storage      | S3-Interface (lokal + MinIO/AWS)          |
| Testing           | TDD (Red-Green-Refactor), testcontainers-go |
| Schema-Management | schema.sql als Source of Truth             |
| Migrationen       | Erst bei Schema-Aenderungen (golang-migrate) |
| Audit             | PostgreSQL-Trigger                        |
| Job Queue         | river (PostgreSQL-basiert, kein Redis)    |
| Connection Pools  | Dual Pools: Web (20) + Worker (5), isoliert |
| Circuit Breaker   | gobreaker fuer externe APIs (AI, Webhooks) |
| KI-Pseudonymisierung | Regex + lokale NER vor jedem KI-Aufruf |
| Volltextsuche     | PostgreSQL tsvector/tsquery + GIN-Index   |
| Event Sourcing    | Nein (audit_logs + Hook-System reichen)   |
| Message Broker    | Nicht noetig (Hook-System + river)        |
| Plugin-System     | 2-Ebenen: Go-Module + Webhooks            |
| KI-Integration    | OpenAI-kompatible API (OpenAI, Ollama, konfigurierbar per API-Key) |

## Infrastruktur-Entscheidungen

### Job Queue: river (PostgreSQL)

Keine zusaetzliche Infrastruktur (kein Redis). river nutzt PostgreSQL als
Backend. Jobs in derselben DB-Transaktion wie Geschaeftslogik einfuegbar.

Einsatz: Webhook-Delivery, E-Mail, PDF-Generierung, Bank-Import,
Billing-Berechnung, SEPA-XML, Dokumentverarbeitung.

Scheduled Jobs:
- OverdueLendingCheck: taeglich, ueberfaellige Ausleihen
- DunningEscalation: woechentlich, Mahnstufen hochsetzen
- WorkHourDeadlineCheck: jaehrlich, fehlende Arbeitsstunden -> Ersatzgeld
- WorkEventSuggestion: monatlich, KI-Terminvorschlaege fuer Einsaetze
- ExpiredLinksCleanup: taeglich, abgelaufene shared_links
- PaymentReminder: monatlich, offene Rechnungen
- WarningExpiry: monatlich, abgelaufene Abmahnungen
- MeterAnomalyCheck: bei neuem Zaehlerstand, KI-Anomalie-Erkennung

### Dual Connection Pools

Zwei getrennte pgx-Pools auf dieselbe PostgreSQL-Instanz:
- Web-Pool (max_conns=20): HTTP Requests, Login, CRUD, API-Abfragen
- Worker-Pool (max_conns=5): river Jobs, PDF, Mail, Webhooks, KI

Wenn Worker erschoepft sind (z.B. chromedp haengt), stauen sich Jobs,
aber Web-Requests bleiben responsive. Outbox-Pattern bleibt intakt
(eine Datenbank, atomare Transaktionen).

### Circuit Breaker (gobreaker)

Fuer alle externen API-Aufrufe (KI, Webhooks, SMTP):
- Closed: Normalbetrieb
- 3 Fehler in Folge -> Open: Jobs scheitern sofort (<1ms), DB-Connection frei
- Nach Cooldown -> Half-Open: 1 Test-Request -> Erfolg: Closed / Fehler: Open

Fallback bei offenem Circuit: Datensatz wird als draft ohne KI-Anreicherung
gespeichert, markiert fuer manuelle Pruefung.

### KI-Pseudonymisierung (DSGVO)

Zwingende Middleware vor jedem externen KI-Aufruf (egal ob Cloud oder lokal):

Strukturierte Daten (Bank-Import):
- Nur Verwendungszweck an KI senden, Name/IBAN weglassen
- Verbleibende IBANs per Regex maskieren

Unstrukturierte Texte (Verstoesse, Freitext):
- Lokale Named Entity Recognition (prose/v2, pure Go, <10 MB RAM, CPU-only)
- Erkannte Namen/Orte durch Tokens ersetzen ("Hans Mueller" -> "Mitglied_A")
- Token-Mapping in-memory speichern
- Pseudonymisierten Text an KI senden
- Antwort zurueck-mappen

Echte personenbezogene Daten verlassen den Server nie im Klartext.

### Volltextsuche: PostgreSQL tsvector

Kein Elasticsearch. tsvector GENERATED ALWAYS Spalten auf users, units,
documents, bank_transactions, lendable_items, incidents, work_task_catalog.

### Event Sourcing: Nein

audit_logs + jsonb_diff() + accounting_journal (immutable ledger) reichen.
Hook-System ist de facto Event-basierte Kommunikation.

## KI-Integration

### Provider-Konfiguration

Ein API-Key + URL + Modellname bestimmt den Provider.
OpenAI-kompatible API (funktioniert mit OpenAI, Anthropic via Proxy, Ollama).

```
AI_PROVIDER_URL=https://api.openai.com/v1   # oder http://localhost:11434/v1
AI_API_KEY=sk-...
AI_MODEL=gpt-4o                              # oder llama3.1
AI_GDPR_MODE=strict                          # strict = nur lokal, cloud = erlaubt
```

### KI-Einsatzpunkte

| Bereich | Funktion |
|---------|----------|
| Verstoesse | Schweregrad + Rechtsgrundlage vorschlagen |
| Korrespondenz | Mahnschreiben, Abmahnungen, Kuendigungen generieren |
| Bank-Import | Transaktionen kategorisieren die keiner Regel entsprechen |
| Dokumente | OCR + Datenextraktion aus Rechnungen/Vertraegen |
| Zaehlerstand | Anomalie-Erkennung (ungewoehnlicher/negativer Verbrauch) |
| Einsatzplaner | Terminvorschlaege, Teilnehmer-Priorisierung nach Reststunden |

### DSGVO-Modi

| Modus | Verhalten |
|-------|-----------|
| disabled | Keine KI. Alle ai_* Felder bleiben NULL. |
| strict | Nur lokale Modelle (Ollama). Keine Daten verlassen den Server. |
| cloud | Cloud-Provider erlaubt (OpenAI, Anthropic). AVV noetig. |

## Plugin-System (2 Ebenen)

### Ebene 1: Go-Module

Go-Pakete die das Plugin Interface implementieren. Kompiliert in die Binary.
Verteilung als Go-Module via go get. Kein eigener Paketmanager - Go's
Modulsystem (go mod) ist der Paketmanager.

Quellen: Git Repos (GitHub, GitLab, Gitea), lokale Verzeichnisse.
Konvention: github.com/{user}/vv-plugin-{name}

Installation:
  go get github.com/someone/vv-plugin-datev
  -> Import in plugins/registry.go
  -> make build

Docker (fuer Nicht-Entwickler):
  FROM vereinsverwaltung:latest AS builder
  RUN go get github.com/someone/vv-plugin-datev && make build

### Ebene 2: Webhooks

Externe Integrationen. URL + Secret per API registrieren.
System ruft bei Events die URL async auf (via river Job, HMAC-signiert).
Einsatz: DATEV-Export, Slack/Telegram, Website-CMS, Verband-API.

## Automatisierung + Freigabe Pattern

Mahnungen, Arbeitseinsatz-Einladungen, Abrechnungen werden automatisch
als Entwurf erzeugt (status='draft'). Vorstand prueft und gibt frei
(status='approved'). Erst dann PDF-Generierung und Versand.

## PDF-Generierung

Von Anfang an dabei. HTML-Templates -> PDF.

Vordefinierte Templates:
- Mahnung (Stufe 1, 2, 3)
- Abmahnung
- Kuendigung
- Jahresabrechnung / Pachtrechnung
- Arbeitseinsatz-Einladung
- Willkommensschreiben

## Dependencies (go.mod)

```
github.com/gin-gonic/gin
github.com/jackc/pgx/v5
github.com/sqlc-dev/sqlc              (CLI)
github.com/golang-jwt/jwt/v5
github.com/google/uuid
github.com/shopspring/decimal
github.com/go-playground/validator/v10
github.com/joho/godotenv
github.com/yeqown/go-qrcode/v2
github.com/riverqueue/river
github.com/riverqueue/river/riverdriver/riverpgxv5
github.com/pquerna/otp                     ZFA (TOTP)
github.com/chromedp/chromedp           (PDF-Generierung)
github.com/sashabaranov/go-openai      (KI - OpenAI-kompatible API)
github.com/sony/gobreaker              Circuit Breaker
github.com/jdkato/prose/v2             Lokale NER (Pseudonymisierung)
go.uber.org/zap
golang.org/x/crypto
gopkg.in/gomail.v2
```

## Module (21)

### Kern (immer aktiv)
- auth: users, roles, permissions, user_roles, role_permissions, user_totp_secrets
- crm: user_address, user_contact_points, user_preferences
- groups: groups, group_members
- documents: documents, document_categories, document_relations
- audit: audit_logs, view_revision_report
- sharing: shared_links, QR-Codes

### Optionale Module (keine Abhaengigkeiten untereinander)
- accounting: accounting_accounts, accounting_journal, accounting_entries
- operations: inventory_items, deposits, work_logs, expense_claims
- lending: lendable_items, lending_records (Geraeteverleih, Bulk + Gruppen-Ausleihe, optional: groups)

### Optionale Module (mit Abhaengigkeiten)
- property: units, garages, parcels
- metering: meter_readings (braucht: property)
- ownership: unit_ownerships (braucht: property)
- leasing: leases (braucht: property)
- applicants: applicants (braucht: property)
- billing: billing_periods, billing_items (braucht: leasing + accounting)
- banking: bank_accounts, bank_transactions, rules, import_logs (braucht: accounting)
- workhours: requirements, obligations, adjustments, events, tasks, participants, catalog (braucht: auth)
  - optional: property (Unit-Verpflichtungen), billing (Ersatzgeld), groups (Gruppen einladen), ai
- compliance: legal_provisions, incidents, witnesses, consequences, dunning, warnings, terminations (braucht: auth, documents)
  - optional: billing (Mahnwesen), leasing (Kuendigungen), workhours (No-Show), ai
- webhooks: webhook_subscriptions

### Vereinstyp-Konfigurationen

| Vereinstyp          | Module                                                               |
|---------------------|----------------------------------------------------------------------|
| Kleingartenverein   | Kern + property + leasing + ownership + metering + billing +         |
|                     | accounting + applicants + banking + lending + workhours + compliance  |
| Sportverein         | Kern + accounting + operations + lending + workhours                  |
| Foerderverein       | Kern + accounting + banking                                          |
| Garagengemeinschaft | Kern + property + leasing + ownership + metering + accounting +      |
|                     | billing + compliance                                                 |

Aktivierung per Config: MODULES_ENABLED=auth,crm,accounting,...

## Modulares Schema

```
schema.sql                   Gesamtschema (Referenz, 2092 Zeilen, 50 Tabellen + 10 Meta + 2 Views)
schema_core.sql              Immer: users, roles, permissions, CRM, documents, audit, shared_links, webhooks
schema_property.sql          Optional: units, garages, parcels
schema_leasing.sql           Optional: leases, unit_ownerships
schema_billing.sql           Optional: billing_periods, billing_items
schema_metering.sql          Optional: meter_readings
schema_accounting.sql        Optional: kontenplan, journal, entries
schema_banking.sql           Optional: bank import
schema_operations.sql        Optional: inventory, deposits, work_logs, expenses
schema_applicants.sql        Optional: applicants
schema_lending.sql           Optional: lendable_items, lending_records
schema_workhours.sql         Optional: requirements, obligations, adjustments, events, tasks, participants, catalog
schema_compliance.sql        Optional: legal_provisions, incidents, witnesses, dunning, warnings, terminations
schema_metadata.sql          Optional: *_meta Tabellen
schema_seeds.sql             Seed-Daten
```

## Implementierungsphasen

### Phase 1: Walking Skeleton + CI/CD (3-4 Sessions)
- Projekt-Setup, Docker, Makefile, .golangci.yml, GitHub Actions
- Multi-Stage Dockerfile, testcontainers-go Setup
- Core: database (Dual Pools), config, errors, types, middleware
- Core: hooks (HookManager, CRUDHooks, Plugin Interface)
- Core: jobs (river Grundgeruest), ai (Noop Interface nur)
- Periphere Infrastruktur kommt spaeter wo erstmals gebraucht

### Phase 2: Identity + CRM + Groups (5-7 Sessions)
- Auth: User CRUD, JWT, ZFA (TOTP), Rollen, Permissions
- CRM: Adresse, Kontaktpunkte, Praeferenzen
- Groups: Gruppen + Mitglieder
- Neu aus TP1: core/mail, core/metadata, core/search

### Phase 3: Property + Pacht (5-6 Sessions)
- Units, Garages, Parcels, Ownership, Leasing, Applicants
- Status-Workflows, Overlap-Handling

### Phase 4: Billing + PDF + Admin + Snapshots (5-6 Sessions)
- Rechnungsstellung (Pacht + Strom + Wasser + Umlagen)
- Neu aus TP1: core/pdf, core/admin (Job-Dashboard)
- billing_snapshots: JSON-Abbild bei Freigabe (Absicherung fuer TP8)
- Hook billing.period.after_approve vorbereitet (leer)

### Phase 5: Dokumente + Zaehler + QR + Verleih (6-7 Sessions)
- Neu aus TP1: core/storage (S3 Interface)
- Documents, Metering, Sharing (QR), Lending (Bulk + Gruppen)
- KI-Hooks mit Noop: document.after_upload, meter_reading.after_create

### Phase 6: Betrieb + Arbeitsstunden (8-10 Sessions)
- Operations: Inventar, Kautionen, Arbeitsprotokolle, Spesen
- Workhours: Pflicht (3 Ebenen), Einsatzplaner, Events, Teilnehmer
- KI-Hook mit Noop: workhours.event.ai_suggest

### Phase 7: Compliance (8-10 Sessions)
- Verstoesse, Mahnungen, Abmahnungen, Kuendigungen
- Rechtsgrundlagen-Katalog, PDF-Generierung, Eskalationskette
- KI-Hooks mit Noop: incident.create.validate, compliance.ai_draft

### Phase 8: Accounting + Banking + Dry-Run (6-8 Sessions)
- Doppelte Buchfuehrung (SKR49, Journal, Entries, Lock)
- Dry-Run: Validierung gegen billing_snapshots vor Migration
- Billing-Hook Integration (after_approve -> Journal)
- Bank-Import (CSV Parser, Kategorisierung, Duplikate)
- KI-Hook mit Noop: banking.import.categorize

### Phase 9: KI + Pseudonymizer + Webhooks (4-6 Sessions)
- Neu aus TP1: core/ai (OpenAI, Pseudonymizer, NER, Circuit Breaker)
- Neu aus TP1: webhook_delivery
- Noop durch echten Provider ersetzen (kein Modul-Code aendern)
- Webhooks-Modul (Subscriptions, Delivery, HMAC)

### Phase 10: Qualitaet + Go-Fixtures + Deployment (4-6 Sessions)
- Go-Fixtures Factory (isolierte State-Tests statt Domino-E2E)
- Audit, DSGVO, Metadata API, Rate Limiting
- Coverage-Luecken, Performance-Tests, Smoke Test
- Plugin-System finalisieren
- Gesamt: ~54-70 Sessions

## REST API

### Auth
```
POST   /api/auth/login
POST   /api/auth/2fa/verify              (TOTP Code pruefen, gibt JWT zurueck)
POST   /api/auth/2fa/setup               (TOTP einrichten, gibt QR-Code + Secret)
POST   /api/auth/2fa/confirm             (Erster TOTP Code zur Aktivierung)
POST   /api/auth/2fa/disable             (ZFA deaktivieren)
POST   /api/auth/2fa/recovery            (Recovery Code nutzen)
POST   /api/auth/register
GET    /api/users
GET    /api/users/:id
PATCH  /api/users/:id
DELETE /api/users/:id
POST   /api/users/:id/roles
DELETE /api/users/:id/roles/:slug
GET    /api/search/users?q=
```

### CRM
```
GET    /api/users/:id/address
PUT    /api/users/:id/address
GET    /api/users/:id/contacts
POST   /api/users/:id/contacts
PATCH  /api/users/:id/contacts/:cid
DELETE /api/users/:id/contacts/:cid
GET    /api/users/:id/preferences
PUT    /api/users/:id/preferences
```

### Groups
```
GET    /api/groups
POST   /api/groups
GET    /api/groups/:id
PATCH  /api/groups/:id
DELETE /api/groups/:id
GET    /api/groups/:id/members
POST   /api/groups/:id/members
DELETE /api/groups/:id/members/:user_id
GET    /api/users/:id/groups              (Gruppen eines Users)
```

### Property
```
GET    /api/units
POST   /api/units
GET    /api/units/:id
PATCH  /api/units/:id
DELETE /api/units/:id
GET    /api/units/:id/details
PUT    /api/units/:id/details
```

### Ownership + Leasing
```
GET    /api/units/:id/ownerships
POST   /api/units/:id/ownerships
PATCH  /api/ownerships/:id
GET    /api/units/:id/leases
POST   /api/units/:id/leases
GET    /api/leases/:id
PATCH  /api/leases/:id
POST   /api/leases/:id/status
```

### Applicants
```
GET    /api/applicants
POST   /api/applicants
GET    /api/applicants/:id
PATCH  /api/applicants/:id
POST   /api/applicants/:id/assign
```

### Accounting
```
GET    /api/accounts
POST   /api/accounts
GET    /api/journal
POST   /api/journal
GET    /api/journal/:id
PATCH  /api/journal/:id
POST   /api/journal/:id/lock
GET    /api/journal/:id/entries
```

### Billing
```
GET    /api/billing/periods
POST   /api/billing/periods
GET    /api/billing/periods/:id
POST   /api/billing/periods/:id/calculate
POST   /api/billing/periods/:id/approve
GET    /api/billing/periods/:id/items
GET    /api/billing/periods/:id/pdf
```

### Banking
```
POST   /api/banking/import/preview
POST   /api/banking/import/execute
GET    /api/banking/accounts
GET    /api/banking/transactions
GET    /api/banking/transactions?q=
GET    /api/banking/rules
POST   /api/banking/rules
PATCH  /api/banking/rules/:id
DELETE /api/banking/rules/:id
GET    /api/banking/import-logs
```

### Documents
```
GET    /api/documents
POST   /api/documents
GET    /api/documents/:id
GET    /api/documents/:id/download
DELETE /api/documents/:id
POST   /api/documents/:id/relate
GET    /api/document-categories
GET    /api/search/documents?q=
```

### Metering
```
GET    /api/units/:id/meter-readings
POST   /api/units/:id/meter-readings
PATCH  /api/meter-readings/:id
```

### Operations
```
GET    /api/units/:id/inventory
POST   /api/units/:id/inventory
PATCH  /api/inventory/:id
GET    /api/leases/:id/deposits
POST   /api/leases/:id/deposits
PATCH  /api/deposits/:id
GET    /api/work-logs
POST   /api/work-logs
PATCH  /api/work-logs/:id
POST   /api/work-logs/:id/verify
GET    /api/expense-claims
POST   /api/expense-claims
PATCH  /api/expense-claims/:id
POST   /api/expense-claims/:id/status
```

### Lending
```
GET    /api/lending/items
POST   /api/lending/items
GET    /api/lending/items/:id
PATCH  /api/lending/items/:id
GET    /api/lending/records
POST   /api/lending/records
GET    /api/lending/records/:id
POST   /api/lending/records/:id/checkout
POST   /api/lending/records/:id/return
POST   /api/lending/records/:id/cancel
```

### Workhours
```
GET    /api/workhours/requirements
PUT    /api/workhours/requirements/:year
GET    /api/workhours/balance
GET    /api/workhours/balance?user_id=&year=
GET    /api/units/:id/workhour-obligations
PUT    /api/units/:id/workhour-obligations/:year
GET    /api/users/:id/workhour-adjustments
PUT    /api/users/:id/workhour-adjustments/:year
GET    /api/workhours/task-catalog
POST   /api/workhours/task-catalog
PATCH  /api/workhours/task-catalog/:id
GET    /api/workhours/events
POST   /api/workhours/events
GET    /api/workhours/events/:id
PATCH  /api/workhours/events/:id
POST   /api/workhours/events/:id/invite
POST   /api/workhours/events/:id/complete
GET    /api/workhours/events/:id/participants
PATCH  /api/workhours/participants/:id/status
POST   /api/workhours/events/:id/ai-suggest     (KI-Teilnehmervorschlag)
```

### Compliance
```
GET    /api/legal-provisions
POST   /api/legal-provisions
PATCH  /api/legal-provisions/:id
GET    /api/incidents
POST   /api/incidents
GET    /api/incidents/:id
PATCH  /api/incidents/:id
POST   /api/incidents/:id/witnesses
POST   /api/incidents/:id/consequences
GET    /api/users/:id/incidents
GET    /api/units/:id/incidents
GET    /api/dunning
POST   /api/dunning
GET    /api/dunning/:id
PATCH  /api/dunning/:id
POST   /api/dunning/:id/approve
GET    /api/dunning/:id/pdf
GET    /api/warnings
POST   /api/warnings
GET    /api/warnings/:id
PATCH  /api/warnings/:id
POST   /api/warnings/:id/approve
GET    /api/warnings/:id/pdf
GET    /api/terminations
POST   /api/terminations
GET    /api/terminations/:id
PATCH  /api/terminations/:id
POST   /api/terminations/:id/approve
GET    /api/terminations/:id/pdf
POST   /api/compliance/ai-assess              (KI-Bewertung eines Verstosses)
POST   /api/compliance/ai-draft               (KI-Korrespondenz generieren)
```

### Sharing
```
POST   /api/shared-links
GET    /api/shared-links
GET    /api/shared-links/:id
DELETE /api/shared-links/:id
GET    /api/shared-links/:id/qr
GET    /p/:slug
```

### Webhooks
```
GET    /api/webhooks
POST   /api/webhooks
GET    /api/webhooks/:id
PATCH  /api/webhooks/:id
DELETE /api/webhooks/:id
POST   /api/webhooks/:id/test
GET    /api/webhooks/hooks
```

### Audit
```
GET    /api/audit
GET    /api/audit?table=&record_pk=
```

### Metadata
```
GET    /api/:entity/:id/meta
PUT    /api/:entity/:id/meta/:key
DELETE /api/:entity/:id/meta/:key
```

### Admin (Job-Dashboard)
```
GET    /api/admin/jobs
GET    /api/admin/jobs/:id
POST   /api/admin/jobs/:id/retry
POST   /api/admin/jobs/:id/cancel
GET    /api/admin/jobs/stats
GET    /api/admin/health
```

## Erweiterungs-Modell (4 Stufen)

```
Stufe 1: EAV Metadata        Dynamische Felder via REST API (kein Code)
Stufe 2: Business Rules       Wenn-Dann via UI (kein Code, spaeter)
Stufe 3: Go-Plugins           Volle Module (Kompilierung noetig)
Stufe 4: Webhooks             Externe HTTP-Integrationen
```

## Seed-Daten

### Rollen
admin, vorstand, kassierer, mitglied, pruefer

### Permissions (47)
users.read/write/delete, units.read/write/delete, leases.read/write/delete,
ownership.read/write, accounting.read/write/lock, billing.read/write/approve,
banking.import/rules, documents.read/write/delete, audit.read,
applicants.read/write/assign, groups.read/write, sharing.read/write, metering.read/write,
operations.read/write, lending.read/write, workhours.read/write/plan,
compliance.read/write/approve, webhooks.read/write,
admin.jobs, meta.read/write

### Kontenplan (24 Konten, vereinfachter SKR49)
1000-1500 Assets, 2000-2900 Liabilities, 3000-3900 Equity,
4000-4900 Income (inkl. 4400 Leihgebuehren, 4500 Arbeitsstunden-Ersatzgeld, 4600 Mahngebuehren),
6000-6900 Expenses

### Dokument-Kategorien (8)
vertrag, beleg, protokoll, rechnung, bescheid, foto, plan, sonstiges

### Rechtsgrundlagen-Katalog (12 Eintraege)
Vereinssatzung (§4, §6, §7), Gartenordnung (§3, §5, §8, §10),
Bundeskleingartengesetz (§1, §3, §9)

### Aufgabenkatalog (12 Aufgaben)
Hecken, Wege, Laub, Vereinshaus, Wasserleitung, Fruehjahrs-/Herbstputz,
Spielplatz, Zaun, Vereinsfest, Muellsammelplatz
