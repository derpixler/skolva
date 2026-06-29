-- name: CreateGroup :one
INSERT INTO groups (name, description, group_type, created_by, updated_by)
VALUES (sqlc.arg(name), sqlc.arg(description), sqlc.arg(group_type), sqlc.arg(actor), sqlc.arg(actor))
RETURNING id, name, description, group_type, is_active, created_at, updated_at;

-- name: GetGroupByID :one
SELECT id, name, description, group_type, is_active, created_at, updated_at
FROM groups
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListGroups :many
SELECT id, name, description, group_type, is_active, created_at, updated_at
FROM groups
WHERE deleted_at IS NULL
ORDER BY name, id
LIMIT $1 OFFSET $2;

-- name: ListGroupsByType :many
SELECT id, name, description, group_type, is_active, created_at, updated_at
FROM groups
WHERE deleted_at IS NULL AND group_type = $1
ORDER BY name, id
LIMIT $2 OFFSET $3;

-- name: UpdateGroup :one
UPDATE groups
SET name = sqlc.arg(name),
    description = sqlc.arg(description),
    group_type = sqlc.arg(group_type),
    is_active = sqlc.arg(is_active),
    updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING id, name, description, group_type, is_active, created_at, updated_at;

-- name: SoftDeleteGroup :exec
UPDATE groups
SET deleted_at = NOW(), updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;
