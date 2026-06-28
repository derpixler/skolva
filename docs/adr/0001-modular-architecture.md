# ADR 0001 — Modular, open-core, polyrepo architecture

- **Status:** Accepted
- **Date:** 2026-06
- **Context branch:** `feature/modularization` (PR #136)

## Context

Skolva began as a single Go module (`github.com/derpixler/skolva`) with one
binary, one monolithic `schema.sql`, and the router importing each feature
package concretely. The roadmap (TP1–TP10) bundled identity, CRM, groups,
property, billing, documents, metering, lending, work hours, compliance,
accounting and AI into one codebase.

We want Skolva to be **more modular**: a reusable **core** that other
products can build on, with each domain shipped as an independent module that
is later combined into Skolva — or into something new.

## Decision

1. **Polyrepo.** Core and each module are autonomous Git repos with their own
   SemVer, CI and issues:
   - `derpixler/skolva-core` — infra + module SDK + identity (Apache-2.0).
   - `derpixler/skolva-<name>` — one feature module each.
   - `derpixler/skolva` — the product: composition root assembling core +
     chosen modules into a single binary.
2. **Compile-time assembly, one binary.** No microservices, no dynamic plugins.
3. **Module SDK** (`core/module`): a `Module` contract + typed `Deps` bundle +
   a `Registry` that mounts routes, registers hooks/permissions, runs
   migrations and drives lifecycle. The composition root owns the assembly
   (`app.DefaultRegistry`).
4. **Pluggable infra seams — interface + default now, adapters later.** Core
   defines `events.Bus` (in-proc default), `cache.Cache` (in-memory),
   `search.Service` (Postgres FTS). Redis / RabbitMQ / **Elasticsearch**
   adapters are built only when a real need appears.
5. **Per-module schema ownership** via a migration runner
   (`Registry.Migrate` + `schema_migrations`). The full split of `schema.sql`
   happens **at extraction time** for each module (premature earlier because
   future-module tables still live in the shared schema).
6. **Open-core, Apache-2.0** for the core.
7. **Identity provider seam** decomposed into three independent concerns:
   per-request token verification (`middleware.Verifier`), the login provider
   (`auth.Provider`, local default, OIDC-ready), and authorization/RBAC
   (`RequirePermission`) — kept separate.

## Consequences

- **+** Core is a clean, reusable, open foundation; modules evolve and ship
  independently; the product is a thin assembly.
- **+** Seams keep enterprise infra (Redis/RabbitMQ/Elasticsearch) reachable
  without paying for it now.
- **−** Polyrepo adds version-coordination overhead (core change → bump in each
  module → release → bump in product) and weaker local cross-repo ergonomics
  (mitigated by a local `go.work`).
- Modules must never import one another; cross-module interaction goes through
  the event bus + soft references.

The repeatable per-module procedure is captured for agents in the
`skolva-module-charter` skill.
