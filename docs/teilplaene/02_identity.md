# Teilplan 2: Identity + CRM + Groups

## Voraussetzungen
Teilplan 1 (Foundation + Core) abgeschlossen.

## Ziel
Vollstaendige Benutzerverwaltung mit Rollen, Rechten, ZFA, CRM-Daten
und generischen Gruppen.

## Methodik: TDD
Jede Funktion wird test-first entwickelt. Kein Code ohne vorherigen Test.

## Scope

### Tabellen
- users, roles, permissions, user_roles, role_permissions
- user_totp_secrets
- user_address, user_contact_points, user_preferences
- groups, group_members

### Module + neue Core-Packages
```
internal/auth/        User CRUD, JWT, ZFA, Rollen, Permissions
internal/crm/         Adresse, Kontaktpunkte, Praeferenzen
internal/groups/      Gruppen, Mitgliedschaft
internal/core/mail/   SMTP Service (erstmals hier gebraucht: ZFA E-Mail)
internal/core/metadata/  EAV Meta-Tabellen (erstmals hier: User-Meta)
internal/core/search/    tsvector Helper (erstmals hier: User-Suche)
```

## Aufgaben

### 1. Auth - Repository
- TEST: user_repo_test.go - Create, FindByID, FindByEmail, Update, SoftDelete (testcontainers)
- TEST: user_repo_test.go - Unique Email Constraint, Soft-Delete filtert korrekt
- TEST: role_repo_test.go - CRUD, RolePermissions, UserRoles (testcontainers)
- IMPL: model.go, repository.go, repository_pg.go
- sqlc Queries: auth.sql

### 2. Auth - Service
- TEST: service_test.go - HashPassword + VerifyPassword Round-Trip
- TEST: service_test.go - CreateUser Validierung (leere Felder, doppelte Email)
- TEST: service_test.go - GenerateJWT + ValidateJWT (gueltig, abgelaufen, manipuliert)
- TEST: service_test.go - AssignRole, RemoveRole, CheckPermission
- IMPL: service.go (mit gemocktem Repository)

### 3. Auth - ZFA
- TEST: totp_test.go - Setup generiert Secret + QR URI
- TEST: totp_test.go - Verify akzeptiert gültigen Code, lehnt falschen ab
- TEST: totp_test.go - Recovery Codes: generiert, gehasht, einloesbar, nur einmal
- TEST: totp_test.go - Brute-Force: 5 Fehlversuche -> locked_until gesetzt
- TEST: totp_test.go - Erzwingung: User mit Rolle 'admin' ohne ZFA -> Redirect
- IMPL: service.go (ZFA Methoden), repository_pg.go (user_totp_secrets)

### 4. Auth - Handler + Middleware
- TEST: handler_test.go - POST /login Happy Path -> JWT
- TEST: handler_test.go - POST /login falsche Credentials -> 401
- TEST: handler_test.go - POST /login mit ZFA -> requires_2fa + temp_token
- TEST: handler_test.go - POST /2fa/verify -> JWT
- TEST: handler_test.go - GET /users ohne Auth -> 401
- TEST: handler_test.go - GET /users ohne Permission -> 403
- TEST: handler_test.go - User CRUD (Create, Read, Update, SoftDelete)
- TEST: handler_test.go - Role Assignment (Add, Remove)
- TEST: handler_test.go - Volltextsuche GET /search/users?q=Mueller
- IMPL: handler.go, routes.go, dto.go
- IMPL: middleware/auth.go fertigstellen

### 5. Auth - Seeds
- TEST: seeds_test.go - Rollen + Permissions + Zuordnungen korrekt (testcontainers)
- IMPL: Seed-Daten aus schema.sql verifizieren

### 6. CRM - Repository + Service
- TEST: address_repo_test.go - Upsert, Get, 1:1 Constraint (testcontainers)
- TEST: contacts_repo_test.go - CRUD, is_primary Constraint (max 1 pro Typ)
- TEST: contacts_service_test.go - SetPrimary entfernt alten Primary
- TEST: preferences_repo_test.go - Upsert, Get
- IMPL: model.go, dto.go, repository.go, repository_pg.go, service.go

### 7. CRM - Handler
- TEST: handler_test.go - Address PUT/GET, 404 wenn nicht vorhanden
- TEST: handler_test.go - Contacts CRUD, Primary-Logik
- TEST: handler_test.go - Preferences PUT/GET
- TEST: handler_test.go - Permission-Check auf allen Endpunkten
- IMPL: handler.go, routes.go

### 8. Groups - Repository + Service
- TEST: group_repo_test.go - CRUD, SoftDelete, FindByType (testcontainers)
- TEST: members_repo_test.go - Add, Remove, List, UserGroups
- TEST: group_service_test.go - Mitglied hinzufuegen, Rolle aendern
- IMPL: model.go, dto.go, repository.go, repository_pg.go, service.go

### 9. Groups - Handler
- TEST: handler_test.go - Group CRUD
- TEST: handler_test.go - Members Add/Remove/List
- TEST: handler_test.go - GET /users/:id/groups
- TEST: handler_test.go - Permission-Check
- IMPL: handler.go, routes.go

### 10. Core: Mail
- TEST: smtp_test.go - Mail zusammengebaut, gesendet (Mock SMTP)
- TEST: smtp_test.go - HTML Template mit Variablen
- IMPL: core/mail/smtp.go

### 11. Core: Metadata (EAV)
- TEST: metadata_test.go - GetMeta, SetMeta, DeleteMeta, ListMeta (testcontainers)
- TEST: metadata_test.go - Unique Constraint (entity_id + meta_key)
- IMPL: core/metadata/metadata.go

### 12. Core: Search
- TEST: search_test.go - tsvector Query Builder, deutsche Stemming
- TEST: search_test.go - User-Suche ueber search_vector
- IMPL: core/search/search.go

### 13. Integration
- TEST: E2E - Register -> Login -> ZFA Setup -> ZFA Verify -> Geschuetzter Endpunkt
- TEST: E2E - User anlegen -> Adresse -> Kontakte -> Gruppe -> Meta setzen -> Suche

## API-Endpunkte nach Abschluss

```
POST   /api/auth/login
POST   /api/auth/2fa/verify
POST   /api/auth/2fa/setup
POST   /api/auth/2fa/confirm
POST   /api/auth/2fa/disable
POST   /api/auth/2fa/recovery
POST   /api/auth/register
GET    /api/users
GET    /api/users/:id
PATCH  /api/users/:id
DELETE /api/users/:id
POST   /api/users/:id/roles
DELETE /api/users/:id/roles/:slug
GET    /api/search/users?q=
GET    /api/users/:id/address
PUT    /api/users/:id/address
GET    /api/users/:id/contacts
POST   /api/users/:id/contacts
PATCH  /api/users/:id/contacts/:cid
DELETE /api/users/:id/contacts/:cid
GET    /api/users/:id/preferences
PUT    /api/users/:id/preferences
GET    /api/groups
POST   /api/groups
GET    /api/groups/:id
PATCH  /api/groups/:id
DELETE /api/groups/:id
GET    /api/groups/:id/members
POST   /api/groups/:id/members
DELETE /api/groups/:id/members/:user_id
GET    /api/users/:id/groups
```

## Geschaetzte Sessions: 5-7
