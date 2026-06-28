# RBAC — Rollen & Permissions in Skolva

## Schema: 3 Tabellen, reine Join-Logik

```
┌─────────────┐     ┌─────────────────────┐     ┌──────────────────────┐
│    roles    │     │    role_permissions  │     │     permissions      │
├─────────────┤     ├─────────────────────┤     ├──────────────────────┤
│ slug     PK │◄────│ role_slug     PK  FK │────►│ slug              PK │
│ display_name│     │ permission_slug PK FK│     │ description          │
│ description │     └─────────────────────┘     │ is_protected = TRUE  │
│ is_protected│                                 └──────────────────────┘
│ deleted_at  │
└─────────────┘                ▲
                               │  „admin hat alle"
                               │  (INSERT ... SELECT 'admin', slug FROM permissions)

     ┌──────────────────┐
     │    user_roles    │
     ├──────────────────┤
     │ user_id    PK FK │  M:N-Tabelle — nicht auditiert, kein Soft-Delete
     │ role_slug  PK FK │  DELETE = hart, ON CONFLICT DO NOTHING (idempotent)
     │ assigned_by   FK │  assigned_by → users(id) ON DELETE SET NULL
     └──────────────────┘
```

## Die 5 Rollen (Seed)

| Slug | Display | is_protected | Berechtigungsumfang |
|---|---|---|---|
| `admin` | Administrator | TRUE | Vollzugriff (Wildcard, alle 47 Permissions) |
| `vorstand` | Vorstand | TRUE | 35 Permissions: users.write, units.write, leases.write, billing.approve, … |
| `kassierer` | Kassierer | TRUE | 20 Permissions: accounting.*, billing.*, banking.*, … |
| `mitglied` | Mitglied | FALSE | 7 Permissions: nur *.read für units, leases, docs, metering, lending, workhours, groups |
| `pruefer` | Prüfer | FALSE | 9 Permissions: nur *.read für accounting, billing, audit, banking, docs, operations, workhours, compliance, groups |

`is_protected = TRUE` bedeutet: die Rolle soll zur Laufzeit nicht gelöscht werden (Konvention, kein technischer Enforcement).

## Die 47 Permissions (Seed)

Jede Permission ist ein atomarer Slug nach dem Schema `<resource>.<action>`:

| Kategorie | Permissions |
|---|---|
| **users** | `users.read`, `users.write`, `users.delete` |
| **units** | `units.read`, `units.write`, `units.delete` |
| **leases** | `leases.read`, `leases.write`, `leases.delete` |
| **ownership** | `ownership.read`, `ownership.write` |
| **accounting** | `accounting.read`, `accounting.write`, `accounting.lock` |
| **billing** | `billing.read`, `billing.write`, `billing.approve` |
| **banking** | `banking.import`, `banking.rules` |
| **documents** | `documents.read`, `documents.write`, `documents.delete` |
| **audit** | `audit.read` |
| **applicants** | `applicants.read`, `applicants.write`, `applicants.assign` |
| **groups** | `groups.read`, `groups.write` |
| **sharing** | `sharing.read`, `sharing.write` |
| **metering** | `metering.read`, `metering.write` |
| **operations** | `operations.read`, `operations.write` |
| **lending** | `lending.read`, `lending.write` |
| **workhours** | `workhours.read`, `workhours.write`, `workhours.plan` |
| **compliance** | `compliance.read`, `compliance.write`, `compliance.approve` |
| **webhooks** | `webhooks.read`, `webhooks.write` |
| **admin** | `admin.jobs` |
| **meta** | `meta.read`, `meta.write` |

Alle 47 haben `is_protected = TRUE` — der Katalog ist fix.

## Permission-Prüfung — Flow beim Login

```
POST /api/auth/login  {"email":"...","password":"..."}
         │
         ▼
  service.Login(ctx, email, password)
    │
    ├─ repo.GetUserByEmail → User (id, email, password_hash)
    ├─ VerifyPassword(hash, password)
    │
    ├─ IF 2FA aktiv → return temp_token (2fa-pending JWT, nur userID)
    │
    └─ ELSE:
         │
         ├─ repo.ListUserRoles(userID) → ["mitglied"]
         ├─ repo.GetPermissionsForUser(userID)
         │      SELECT DISTINCT p.slug
         │      FROM user_roles ur
         │      JOIN roles r ON r.slug = ur.role_slug AND r.deleted_at IS NULL
         │      JOIN role_permissions rp ON rp.role_slug = ur.role_slug
         │      JOIN permissions p ON p.slug = rp.permission_slug
         │      WHERE ur.user_id = $1
         │   → ["units.read","leases.read","docs.read",…]
         │
         └─ tm.IssueAccess(userID, email, roles, permissions)
              → JWT mit Claims {sub, email, roles, perms, kind=access}
```

## Permission-Prüfung — Flow bei jedem API-Request

```
GET /api/users  (Authorization: Bearer <JWT>)
    │
    ├─ Authenticate(verifier):
    │     tm.Verify(token) → Claims → Actor{UserID, Email, Roles, Permissions}
    │     → c.Set("actor", actor)
    │
    ├─ ActorMiddleware:
    │     → c.Request = c.Request.WithContext(ctx with actor)
    │
    └─ RequirePermission("users.read"):
         │
         ├─ actor.HasPermission("users.read")
         │    ├─ IF actor.Roles enthält "admin" → TRUE (Wildcard, kein DB-Hit)
         │    ├─ ELSE: checkt ob "users.read" in actor.Permissions
         │    └─ Result → c.Next() oder c.AbortWithStatusJSON(403)
         │
         └─ Handler: c.JSON(200, users)
```

## Admin-Wildcard

```go
func (a *Actor) HasPermission(permission string) bool {
    if a.HasRole("admin") {
        return true  // ← KEIN DB-Zugriff, reiner In-Memory-Check
    }
    for _, p := range a.Permissions {
        if p == permission { return true }
    }
    return false
}
```

Zwei Ebenen:
1. **Im JWT** (Middleware): Actor mit `Roles=["admin"]` → `HasPermission(anything)` → true — seit #128.
2. **In der DB** (`service.CheckPermission`, seit #18): User mit Admin-Rolle → sofort true, ohne den role_permissions-Join. Admin hat aber auch ALLE 47 Permissions explizit in role_permissions — der Bypass ist Performance-Optimierung.

## Rollen-Verwaltung (API)

| Endpoint | Permission | Beschreibung |
|---|---|---|
| `GET /api/users/:id/roles` | `users.read` | Rollen eines Users listen |
| `POST /api/users/:id/roles {"role_slug":"kassierer"}` | `users.write` | Rolle zuweisen (idempotent) |
| `DELETE /api/users/:id/roles/:slug` | `users.write` | Rolle entfernen |

Rollen selbst sind read-only per API (`GET /api/permissions`, `GET /api/roles`). Die 5 Rollen + 47 Permissions + Mappings sind **geseeded** (#12).

## Datenfluss

```
                    ┌──────────────────────────┐
                    │      role_permissions     │
                    │  (Seed: wer kann was?)    │
                    └──────────┬───────────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
         admin(47)      vorstand(35)      kassierer(20)  ...
              │                │                │
              └────────────────┼────────────────┘
                               │
                    ┌──────────▼───────────┐
                    │      user_roles      │
                    │  (User ← Rolle)      │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │  GetPermissionsFor   │
                    │  User (SQL JOIN)     │  ← DB (1× pro Login)
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │   JWT Claims         │
                    │   {roles, perms}     │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │  Middleware          │
                    │  RequirePermission   │  ← In-Memory (kein DB-Hit pro Request)
                    └──────────────────────┘
```
