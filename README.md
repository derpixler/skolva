# Skolva

> Community management, rooted in trust.

---

## The Story

It's 11 PM on a Tuesday. Markus, board member of a 200-plot garden
association, is on his third coffee, staring at a spreadsheet.

Lease agreements live in binders. Meter readings are on sticky notes.
Invoices are written in Word, one by one. A payment is overdue — was it
March or May? The reminder letter should have gone out two weeks ago.

There's software for this, of course. One vendor charges €99/month per
board member. Another runs on a Windows server from 2008 in someone's
basement. A third is SaaS-only: all member data stored somewhere in a
cloud no one can audit, no consent asked.

Markus is a volunteer. He does this in his spare time, after his day job.
The tools should make his life easier, not harder.

**There has to be a better way.**

---

Skolva was built from exactly that experience.

Not by a corporation. Not as a SaaS funnel. But by people who manage
communities (garden associations, garage cooperatives, sports clubs,
funding societies) and who believe that your data belongs to you.

## What Skolva Is

A **self-hosted, open-source community management platform.** Modular, so you
only activate what you need. Built on PostgreSQL. Designed for
volunteers.

- **Property & Leasing:** Units, plots, garages, lease contracts, status workflows
- **Billing:** Annual invoices with pro-rata calculations, PDF generation
- **Accounting:** Double-entry bookkeeping on SKR49 chart of accounts
- **Documents:** Upload, categorize, relate to any entity. Full-text search.
- **Metering:** Meter readings with consumption calculation
- **Lending:** Trackable items and bulk goods, group checkout
- **Work Hours:** Three-tier obligation system, event planner, balance tracking
- **Compliance:** Incidents, dunning, warnings, terminations. Auto-draft, human approval.
- **Banking:** CSV import with regex-based categorization rules
- **AI Assistance:** Severity assessment, draft generation, anomaly detection. Pseudonymized before any data leaves your server. GDPR-first.

All backed by a **plugin system**: Go modules for deep integration,
webhooks for external services.

## Principles

| Principle | What it means |
|-----------|---------------|
| **Self-hosted** | Your server, your database, your rules. No vendor lock-in. |
| **Open Source** | AGPL-3.0. Code stays in the community where it belongs. |
| **Modular** | Activate only the modules you need. Gardening club ≠ sports club. |
| **GDPR-first** | Pseudonymization for AI. Audit trail on every change. Anonymization support. |
| **No unnecessary infra** | Just PostgreSQL. No Redis, no Elasticsearch, no message broker. |

## *Skolva* [ˈskɔl.va]

**Skol** is an Old Norse word that survives across Scandinavia:
in Swedish and Danish as *skål*, in Norwegian and Icelandic as *skál*.
Originally it meant *bowl* — a shared vessel passed among people gathered
at a table. Over centuries it evolved into the familiar toast: a gesture
of community, trust, and belonging.

**-va** is a nominal suffix found across Germanic languages. It turns
an action into a place or state of being — much like the *-ing* in
*meeting* or *gathering*.

Together, **Skolva** means roughly: *the place where community happens.*

A fitting name for software that helps people manage the land they share,
the work they do together, and the trust that holds their community in place.

## Quick Start

```bash
git clone https://github.com/derpixler/skolva
cd skolva

# Start the database
docker compose up -d

# Run the server
make run

# Health check
curl http://localhost:8080/api/health
# {"status":"healthy"}
```

**Requirements:** Go 1.25+, Docker, PostgreSQL 16.

## Project Status

**Phase 1:** Walking Skeleton: build, test, lint, CI/CD. Health endpoint
running. Foundation for all modules. See `scripts/test.sh` for the full test suite.

## Architecture

```
Skolva/                          REST API
+-- cmd/api/main.go              Entrypoint (Gin router)
+-- internal/
|   +-- app/router.go            Route setup + health
|   +-- core/
|   |   +-- config/              Environment configuration
|   |   +-- database/            Dual pgx connection pools
|   |   +-- errors/              PG error → HTTP status mapping
|   |   +-- types/               Decimal + Duration types
|   |   +-- hooks/               HookManager, CRUDHooks, Plugin system
|   |   +-- middleware/           CORS, RequestID, Auth, Actor
|   |   +-- jobs/                river job queue (PostgreSQL-backed)
|   |   +-- ai/                  Provider interface + Noop implementation
|   +-- [auth, crm, property, ...]  Module packages (Phase 2+)
+-- plugins/registry.go          Plugin loader
+-- sqlc/                        SQL code generation
+-- schema.sql                   Database schema (52+ tables)
+-- scripts/test.sh               Full test suite runner
```

## Modules (planned)

| Phase | Module | Status |
|-------|--------|--------|
| 1 | Walking Skeleton + CI/CD | ✅ Done |
| 2 | Identity (Auth, CRM, Groups) | — |
| 3 | Property + Leasing | — |
| 4 | Billing + PDF + Admin | — |
| 5 | Documents + Metering + Lending | — |
| 6 | Operations + Work Hours | — |
| 7 | Compliance | — |
| 8 | Accounting + Banking | — |
| 9 | AI Activation + Webhooks | — |
| 10 | Quality + Deployment | — |


## License

GNU Affero General Public License v3.0 (AGPL-3.0).
See [LICENSE](LICENSE).
