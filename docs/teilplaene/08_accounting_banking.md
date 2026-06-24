# Teilplan 8: Accounting + Banking

## Voraussetzungen
Teilplan 7 (Compliance) abgeschlossen.
Billing (Teilplan 4) laeuft bereits - Rechnungen werden erzeugt.

## Ziel
Doppelte Buchfuehrung (SKR49) und Bankdatenimport. Accounting
registriert sich auf den Hook billing.period.after_approve und
erzeugt ab sofort automatisch Journal-Eintraege fuer neue Rechnungen.
Historische Rechnungen werden per Migration nachverbucht.

## Methodik: TDD
Jede Funktion wird test-first entwickelt. Kein Code ohne vorherigen Test.

## Scope

### Tabellen
- accounting_accounts, accounting_journal, accounting_entries
- bank_accounts, bank_transactions, bank_categorization_rules, bank_import_logs

### Module
```
internal/accounting/    Kontenplan, Journal, Entries, Lock
internal/banking/       CSV-Import, Parser, Kategorisierung
```

## Aufgaben

### 1. Accounting - Repository
- TEST: account_repo_test.go - Kontenplan Seed vorhanden (24 Konten, testcontainers)
- TEST: journal_repo_test.go - Create, FindByDate, Lock-Status (testcontainers)
- TEST: entry_repo_test.go - Create, Debit/Credit Constraint (testcontainers)
- IMPL: model.go, repository.go, repository_pg.go, sqlc Queries

### 2. Accounting - Atomare Buchung
- TEST: service_test.go - Journal + 2 Entries atomar (Debit=Credit) -> Erfolg
- TEST: service_test.go - Debit!=Credit -> DB CHECK Fehler
- TEST: service_test.go - Debit XOR Credit (nie beides, nie 0/0)
- TEST: service_test.go - Decimal Praezision: 0.1 + 0.2 = 0.3
- IMPL: service.go

### 3. Accounting - Lock
- TEST: service_test.go - Lock bei Balance -> Erfolg
- TEST: service_test.go - Lock bei Imbalance -> Trigger-Fehler
- TEST: service_test.go - Entry nach Lock aendern/einfuegen/loeschen -> Fehler
- IMPL: service.go Lock-Methode

### 4. Accounting - Billing-Hook Integration
- TEST: hooks_test.go - billing.period.after_approve -> Journal + Entries erzeugt
- TEST: hooks_test.go - Pro billing_item ein Entry-Paar (Debit + Credit)
- TEST: hooks_test.go - Journal Balance stimmt nach allen Items
- IMPL: hooks.go (registriert sich auf billing.period.after_approve)

### 5. Accounting - Dry-Run + Migration historischer Rechnungen
- TEST: dryrun_test.go - Berechne Journal-Entries fuer historische Periode
- TEST: dryrun_test.go - Vergleiche Debit/Credit mit billing_snapshot -> 100% Match
- TEST: dryrun_test.go - Rundungsdifferenz -> Fehler + manuelles Review
- TEST: dryrun_test.go - Stornierte Rechnung in Snapshot -> korrekt behandelt
- TEST: migration_test.go - Dry-Run OK -> produktive Migration ausfuehren
- TEST: migration_test.go - Dry-Run FAIL -> Migration gestoppt, keine Daten geaendert
- TEST: migration_test.go - Bereits verbuchte Perioden -> kein Duplikat
- IMPL: dryrun.go (validiert gegen billing_snapshots)
- IMPL: migration.go (Replay nach erfolgreicher Validierung)

### 6. Accounting - Handler
- TEST: handler_test.go - GET /accounts -> Kontenplan
- TEST: handler_test.go - POST /journal mit Entries -> 201
- TEST: handler_test.go - POST /journal unbalanciert -> 422
- TEST: handler_test.go - POST /journal/:id/lock -> Erfolg/Fehler
- TEST: handler_test.go - Permission-Check
- IMPL: dto.go, handler.go, routes.go

### 7. Banking - Parser
- TEST: parser_test.go - detect_format: DKB/Sparkasse/Generic erkannt
- TEST: parser_dkb_test.go - Spalten korrekt gemappt, German Float, Datumsformate
- TEST: parser_dkb_test.go - IBAN Extraktion + Maskierung
- TEST: parser_sparkasse_test.go - Sparkasse-CSV korrekt
- TEST: parser_generic_test.go - Semikolon-CSV
- IMPL: parser.go, parser_dkb.go, parser_sparkasse.go, parser_generic.go

### 8. Banking - Kategorisierung (mit KI-Hook)
- TEST: categorizer_test.go - Regex Rule matched: "rewe" -> Lebensmittel
- TEST: categorizer_test.go - Prioritaet: hoehere gewinnt
- TEST: categorizer_test.go - Kein Match -> Hook banking.import.categorize gefeuert
- TEST: categorizer_test.go - KI Noop -> category bleibt nil
- TEST: categorizer_test.go - KI aktiv -> category vorgeschlagen (Mock Provider)
- IMPL: categorizer.go + KI-Hook Anbindung

### 9. Banking - Import Service
- TEST: service_test.go - Preview: parst, erkennt Duplikate, KEIN DB-Write
- TEST: service_test.go - Execute: speichert neue, ueberspringt Duplikate
- TEST: service_test.go - SHA256 Checksum, Account Auto-Erstellung
- TEST: service_test.go - Import-Log geschrieben
- IMPL: service.go

### 10. Banking - Handler
- TEST: handler_test.go - Preview + Execute Endpunkte
- TEST: handler_test.go - Rules CRUD, Transactions Suche
- IMPL: dto.go, handler.go, routes.go

### 11. Integration
- TEST: E2E - Billing Approve -> Accounting Journal automatisch -> Lock -> Bank-Import -> Kategorisierung

## API-Endpunkte nach Abschluss
```
GET    /api/accounts
POST   /api/accounts
GET    /api/journal
POST   /api/journal
GET    /api/journal/:id
PATCH  /api/journal/:id
POST   /api/journal/:id/lock
GET    /api/journal/:id/entries

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

## Geschaetzte Sessions: 6-8
