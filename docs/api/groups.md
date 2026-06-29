# Groups API

> Authoritative contract: `api/openapi.yaml`.

Generic groups with typed categories (`mannschaft`, `abteilung`,
`arbeitsgruppe`, `vorstand`, `ausschuss`, `sonstige`). Soft-deletable.
All writes run inside actor transactions for audit logging.

## Groups

### GET /groups
Requires `groups.read`. Lists non-deleted groups. Optional query filter:

```
GET /api/groups?group_type=mannschaft  → only that type
GET /api/groups?limit=20&offset=0      → pagination (default 50/0, max 200)
```

### POST /groups
Requires `groups.write`. Creates a group. `name` is required.
`group_type` defaults to `sonstige`. Valid types: `mannschaft`,
`abteilung`, `arbeitsgruppe`, `vorstand`, `ausschuss`, `sonstige`.

```
POST /api/groups  {"name":"Vorstand 2025","group_type":"vorstand"}
→ 201 {"id":"...","name":"Vorstand 2025","group_type":"vorstand","is_active":true,...}
```

`description` is optional.

### GET /groups/:id
Requires `groups.read`. Returns 404 if soft-deleted.

### PATCH /groups/:id
Requires `groups.write`. Updates a group. `name` and `group_type` are
required. `is_active` is optional (keeps current if omitted).
Soft-deleted groups return 404.

### DELETE /groups/:id
Requires `groups.write`. Soft-deletes (sets `deleted_at`). 204. 404 if
already deleted.

## Group Members

Members have a `role_in_group`: `leiter`, `stellvertreter`, `trainer`,
`mitglied` (default). Adding a user who is already a member updates their
role (upsert on the `group_id, user_id` PK).

### GET /groups/:id/members
Requires `groups.read`. Lists members (with user names + email), excludes
soft-deleted users.

```
→ 200 [{"user_id":"...","first_name":"Anna","last_name":"Schmidt","email":"anna@example.com","role_in_group":"leiter","joined_at":"2025-..."}, ...]
```

### POST /groups/:id/members
Requires `groups.write`. Adds (or re-roles) a member.
`user_id` is required; `role_in_group` defaults to `mitglied`.

```
POST /api/groups/:id/members  {"user_id":"...","role_in_group":"leiter"}
→ 200 [...]  (returns updated member list)
```

### DELETE /groups/:id/members/:user_id
Requires `groups.write`. Removes a member. 204.

### GET /users/:id/groups
Requires `groups.read`. Lists all groups the user belongs to (excludes
soft-deleted groups).

```
→ 200 [{"group_id":"...","name":"Vorstand 2025","group_type":"vorstand","role_in_group":"leiter","joined_at":"2025-..."}, ...]
```
