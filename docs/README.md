# Skolva — Documentation

## API

The machine-readable contract is `api/openapi.yaml` (OpenAPI 3.1).
It is served at runtime via `GET /api/openapi.yaml`.

Human-readable overview per module:

| Module | Document | Endpoints |
|--------|----------|-----------|
| Auth & Users | [docs/api/auth.md](api/auth.md) | login, register, user CRUD, role assignment, 2FA, search |
| CRM | [docs/api/crm.md](api/crm.md) | user address, contacts, preferences |
| Groups | [docs/api/groups.md](api/groups.md) | groups CRUD, members |

## Architecture

| Document | Topic |
|----------|-------|
| [docs/rbac.md](rbac.md) | Roles, permissions, how auth decisions work |
| [docs/password-reset.md](password-reset.md) | Password reset flow (forgot + reset via email) |

## Testing

See [docs/testing.md](testing.md) for the full test suite (TP1 + TP2).

Quick run: `./scripts/test.sh` (interactive menu) or `./scripts/test.sh --ci 04 06` (CI mode).
