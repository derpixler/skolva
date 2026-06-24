# Teilplan 9: KI-Aktivierung + Pseudonymizer + Webhooks

## Voraussetzungen
Teilplan 8 (Accounting + Banking) abgeschlossen.
Alle Module haben ihre KI-Hooks bereits mit Noop registriert.

## Ziel
Core-Infrastruktur fuer KI und Webhooks fertigstellen (aus TP1 hierher
verschoben). KI-Provider aktivieren, Pseudonymizer End-to-End,
Webhooks-Modul. Kein Fach-Modul muss geoeffnet werden.

## Methodik: TDD
Jede Funktion wird test-first entwickelt. Kein Code ohne vorherigen Test.

## Scope

### Tabellen
- webhook_subscriptions

### Neue Core-Packages (aus TP1 hierher verschoben)
```
internal/core/ai/openai.go             OpenAI-kompatible Implementierung
internal/core/ai/pseudonymizer.go      Regex + Token-Mapping Engine
internal/core/ai/pseudonymizer_ner.go  Lokale NER (prose/v2, CPU-only)
internal/core/ai/middleware.go         PseudonymizingProvider + Circuit Breaker
internal/core/hooks/webhook_delivery.go  Webhook-Delivery Job (river + HMAC)
```

## Aufgaben

### 1. Core: AI OpenAI Provider
- TEST: openai_test.go - Request Format korrekt (Mock HTTP Server)
- TEST: openai_test.go - Response Parsing (Complete, Classify, Extract)
- TEST: openai_test.go - Fehler-Handling (Timeout, 500, Rate Limit)
- IMPL: core/ai/openai.go

### 2. Core: Pseudonymizer
- TEST: pseudonymizer_test.go - IBAN maskiert: DE89370400440532013000 -> [IBAN]
- TEST: pseudonymizer_test.go - Kontonummer, Telefonnummer, E-Mail maskiert
- TEST: pseudonymizer_test.go - Unmask: Tokens zurueck zu Originaldaten
- IMPL: core/ai/pseudonymizer.go

### 3. Core: NER (Named Entity Recognition)
- TEST: ner_test.go - Name erkannt: "Hans Mueller" -> "Mitglied_A"
- TEST: ner_test.go - Ort erkannt: "Parzelle 12" -> "Objekt_B"
- TEST: ner_test.go - Kein Match -> Text unveraendert
- TEST: ner_test.go - Mapping korrekt gespeichert + zurueck-gemappt
- IMPL: core/ai/pseudonymizer_ner.go (prose/v2)

### 4. Core: AI Middleware (Pseudonymizer + Circuit Breaker)
- TEST: middleware_test.go - Complete: Klartext -> maskiert -> API -> demaskiert
- TEST: middleware_test.go - Kein Klarname/IBAN im ausgehenden Request
- TEST: middleware_test.go - Circuit Breaker: 3 Fehler -> Open -> sofort nil
- TEST: middleware_test.go - Circuit Breaker: Half-Open -> Test -> Closed
- TEST: middleware_test.go - Fallback: draft ohne KI, needs_review=true
- TEST: middleware_test.go - DSGVO strict: Cloud-URL -> Fehler
- IMPL: core/ai/middleware.go (gobreaker)

### 5. KI-Hooks End-to-End (alle Module, kein Modul-Code aendern)
- TEST: banking_ai_e2e_test.go - Import -> Kategorisierung (pseudonymisiert)
- TEST: incident_ai_e2e_test.go - Verstoß -> Schweregrad (pseudonymisiert)
- TEST: compliance_ai_e2e_test.go - Mahnung -> Korrespondenz generiert
- TEST: meter_ai_e2e_test.go - Zaehlerstand -> Anomalie
- TEST: doc_ai_e2e_test.go - Upload -> OCR Extraktion
- TEST: workhours_ai_e2e_test.go - Event -> Terminvorschlag
- IMPL: Nur Provider-Konfiguration aendern (Noop -> OpenAI)

### 6. Core: Webhook-Delivery
- TEST: webhook_delivery_test.go - HTTP POST + HMAC-SHA256 Signatur
- TEST: webhook_delivery_test.go - Retry bei Fehler (max retry_count)
- TEST: webhook_delivery_test.go - Circuit Breaker: 3 Fehler -> Open
- IMPL: core/hooks/webhook_delivery.go

### 7. Webhooks Modul
- TEST: webhook_repo_test.go - CRUD, FindByHook, nur aktive (testcontainers)
- TEST: webhook_service_test.go - DoAction -> Subscriptions -> river Job
- TEST: webhook_service_test.go - failure_count + last_status_code aktualisiert
- IMPL: model.go, dto.go, repository.go, repository_pg.go, service.go

### 8. Webhooks Handler
- TEST: handler_test.go - CRUD, Test-Delivery, Hook-Namen auflisten
- TEST: handler_test.go - Permission-Check (webhooks.read/write)
- IMPL: handler.go, routes.go

### 9. Integration
- TEST: E2E - Webhook registrieren -> Lease erstellen -> Webhook empfaengt (HMAC)
- TEST: E2E - KI off -> ai_* NULL -> KI on -> ai_* befuellt (kein Modul-Code geaendert)

## API-Endpunkte nach Abschluss
```
POST   /api/compliance/ai-assess
POST   /api/compliance/ai-draft
POST   /api/workhours/events/:id/ai-suggest

GET    /api/webhooks
POST   /api/webhooks
GET    /api/webhooks/:id
PATCH  /api/webhooks/:id
DELETE /api/webhooks/:id
POST   /api/webhooks/:id/test
GET    /api/webhooks/hooks
```

## Geschaetzte Sessions: 4-6
