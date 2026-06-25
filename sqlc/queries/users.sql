-- name: CreateUser :one
INSERT INTO users (email, password_hash, first_name, last_name, created_by, updated_by)
VALUES (sqlc.arg(email), sqlc.arg(password_hash), sqlc.arg(first_name), sqlc.arg(last_name), sqlc.arg(actor), sqlc.arg(actor))
RETURNING id, email, first_name, last_name, is_active, is_protected, created_at, updated_at, deleted_at;

-- name: GetUserByID :one
SELECT id, email, first_name, last_name, is_active, is_protected,
       created_at, updated_at, deleted_at
FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, first_name, last_name, is_active, is_protected,
       created_at, updated_at
FROM users
WHERE lower(email) = lower($1) AND deleted_at IS NULL AND anonymized_at IS NULL;

-- name: ListUsers :many
SELECT id, email, first_name, last_name, is_active, is_protected,
       created_at, updated_at
FROM users
WHERE deleted_at IS NULL
ORDER BY last_name, first_name, id
LIMIT $1 OFFSET $2;

-- name: CountActiveUsers :one
SELECT count(*) FROM users WHERE deleted_at IS NULL;

-- name: UpdateUser :one
UPDATE users
SET first_name = sqlc.arg(first_name),
    last_name = sqlc.arg(last_name),
    is_active = sqlc.arg(is_active),
    updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING id, email, first_name, last_name, is_active, is_protected,
          created_at, updated_at, deleted_at;

-- name: SoftDeleteUser :exec
UPDATE users
SET deleted_at = NOW(), updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: UserExistsByEmail :one
SELECT EXISTS (
  SELECT 1 FROM users
  WHERE lower(email) = lower($1) AND deleted_at IS NULL AND anonymized_at IS NULL
);
