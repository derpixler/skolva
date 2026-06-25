-- name: GetUserByID :one
SELECT id, email, first_name, last_name, is_active, is_protected,
       created_at, updated_at, deleted_at
FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: CountActiveUsers :one
SELECT count(*) FROM users WHERE deleted_at IS NULL;
