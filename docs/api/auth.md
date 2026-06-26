# Auth API

> The authoritative machine-readable contract is `api/openapi.yaml`.
> This document provides an overview and examples.

All endpoints are mounted under `/api`. Authentication uses Bearer JWTs
(HMAC-SHA256, issuer `skolva`). Permissions are enforced via role-based
checks (47 granular permissions across 5 seeded roles; admin = wildcard).

## Endpoints

### POST /auth/login
Public. Authenticate with email + password.
- If 2FA is **not** active: returns a full access token (`token`).
- If 2FA **is** active: returns `requires_2fa: true` + a `temp_token`
  (exchange at `/auth/2fa/verify`).

```
POST /api/auth/login
{"email":"user@example.com","password":"s3cr3t"}
→ 200 {"token":"eyJ..."}
→ 200 {"requires_2fa":true,"temp_token":"eyJ..."}
```

### POST /auth/register
Requires `users.write`. Admin-gated user account creation. Emails must be
unique (409 on duplicate).

```
POST /api/auth/register  Authorization: Bearer <admin-token>
{"email":"new@example.com","password":"password123","first_name":"New","last_name":"User"}
→ 201 {"id":"...","email":"new@example.com","first_name":"New","last_name":"User","is_active":true,...}
```

### GET /users
Requires `users.read`. Paginated list (query params: `limit`, `offset`;
defaults 50/0, max 200).

```
GET /api/users?limit=10  Authorization: Bearer <token>
→ 200 [{"id":"...","email":"...","first_name":"...","last_name":"...","is_active":true,...}, ...]
```

### GET /users/:id
Requires `users.read`. Returns 404 if the user is soft-deleted.

### PATCH /users/:id
Requires `users.write`. Full update of first_name, last_name, is_active
(optional; defaults to current). Soft-deleted users return 404.

### DELETE /users/:id
Requires `users.write`. Soft-deletes the user (sets `deleted_at`). 204 on
success, 404 if already deleted.

### GET /users/:id/roles
Requires `users.read`. Lists assigned roles.

```
→ 200 [{"slug":"mitglied","display_name":"Mitglied","is_protected":false}, ...]
```

### POST /users/:id/roles
Requires `users.write`. Assign a role (idempotent). Returns updated roles.

```
POST /api/users/:id/roles  {"role_slug":"kassierer"}
→ 200 [{"slug":"kassierer",...}, {"slug":"mitglied",...}]
```

### DELETE /users/:id/roles/:slug
Requires `users.write`. Remove a role. 204.

### GET /search/users?q=
Requires `users.read`. German full-text search over users (names + email).
Returns results ranked by relevance.

```
GET /api/search/users?q=schmidt
→ 200 [{"id":"...","email":"anna@example.com","first_name":"Anna","last_name":"Schmidt",...}, ...]
```

## 2FA Endpoints

### POST /auth/2fa/setup
Requires authentication (any valid access token). Generates a TOTP secret +
10 single-use recovery codes. Returns a provisioning URI (for QR codes) and
the **plaintext** recovery codes (shown once).

```
POST /api/auth/2fa/setup  Authorization: Bearer <token>
→ 200 {"provisioning_uri":"otpauth://totp/Skolva:...","recovery_codes":["...",...]}
```

### POST /auth/2fa/confirm
Requires authentication. Verifies a TOTP code and activates 2FA. 204.

```
POST /api/auth/2fa/confirm  {"code":"123456"}
→ 204
```

### POST /auth/2fa/verify
Public. Exchanges a 2FA pending token + TOTP code for a full access token.
After 5 consecutive failures the account is locked for 15 minutes.

```
POST /api/auth/2fa/verify  {"temp_token":"eyJ...","code":"123456"}
→ 200 {"token":"eyJ..."}
```

### POST /auth/2fa/recovery
Public. Exchanges a pending token + a single-use recovery code for a full
access token. Each code can be used only once.

```
POST /api/auth/2fa/recovery  {"temp_token":"eyJ...","code":"ABCD1234EFGH5678"}
→ 200 {"token":"eyJ..."}
```

### POST /auth/2fa/disable
Requires authentication. Validates a TOTP code and removes 2FA. 204.

## Roles & Permissions

| Role | Seeded permissions | Purpose |
|------|--------------------|---------|
| admin | all 47 (wildcard) | Full access |
| vorstand | 35 | Board: manage users/units/leases/billing/applicants/groups |
| kassierer | 20 | Treasurer: accounting, billing, banking, documents |
| mitglied | 7 | Member: read units, leases, documents, metering, lending, work hours, groups |
| pruefer | 9 | Auditor: read accounting, billing, audit log, banking, documents |

The full permission catalog (47 slugs) is seeded in `schema.sql`.

## Error Responses

All errors follow:
```json
{"code":"UNAUTHORIZED","message":"invalid credentials"}
```

Common error codes: `UNAUTHORIZED` (401), `FORBIDDEN` (403), `NOT_FOUND`
(404), `CONFLICT` (409), `VALIDATION_ERROR` (422), `INTERNAL_ERROR` (500).
