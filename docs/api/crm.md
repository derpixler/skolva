# CRM API

> Authoritative contract: `api/openapi.yaml`.

Endpoints for a user's address, contact points, and preferences.

## Address

Single address per user (1:1 via upsert on `user_id` PK).

### GET /users/:id/address
Requires `users.read`. Returns the address or 404 if none set.

### PUT /users/:id/address
Requires `users.write`. Creates or replaces the address (upsert).
`street1`, `postal_code`, `city`, `country_code` are required.
`country_code` is uppercased and must be a 2-letter ISO code (e.g. `DE`).

```
PUT /api/users/:id/address
{"street1":"Hauptstr. 1","postal_code":"12345","city":"Berlin","country_code":"DE"}
→ 200 {"user_id":"...","street1":"Hauptstr. 1","postal_code":"12345","city":"Berlin","country_code":"DE",...}
```

Fields: `company`, `care_of`, `street2`, `state`, `note` are optional.

## Contact Points

Multiple contacts per user. Enforces max 1 primary contact per type
(`email`/`phone` etc.) — setting a new primary automatically clears the old
one. Soft-deletable.

### GET /users/:id/contacts
Requires `users.read`. Lists all non-deleted contacts, ordered by type.

### POST /users/:id/contacts
Requires `users.write`. Creates a contact. `contact_type` must be one of
`email`, `phone`, `mobile`, `fax`, `website`, `other`. `value` is required.

```
POST /api/users/:id/contacts
{"contact_type":"email","value":"user@example.com","is_primary":true}
→ 201 {"id":"...","contact_type":"email","value":"user@example.com","is_primary":true,...}
```

If `is_primary` is true and another contact of the same type is already
primary, it is demoted automatically.

### PATCH /users/:id/contacts/:cid
Requires `users.write`. Updates the contact. `value` is required; other
fields are optional. `allow_contact` defaults to the existing value if
omitted.

### DELETE /users/:id/contacts/:cid
Requires `users.write`. Soft-deletes (sets `deleted_at`). 204.

## Preferences

Single preferences object per user (upsert on `user_id` PK).

### GET /users/:id/preferences
Requires `users.read`. Returns preferences or 404.

### PUT /users/:id/preferences
Requires `users.write`. Creates or replaces preferences.
`preferred_contact_type` must be one of `email`, `phone`, `mobile`,
`postal`, `other` (or omitted).

```
PUT /api/users/:id/preferences
{"preferred_contact_type":"email","note":"Reply within 24h"}
→ 200 {"user_id":"...","preferred_contact_type":"email","note":"Reply within 24h",...}
```
