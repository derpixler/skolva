-- name: ListRoles :many
SELECT slug, display_name, description, is_protected, created_at, updated_at
FROM roles
WHERE deleted_at IS NULL
ORDER BY slug;

-- name: GetRole :one
SELECT slug, display_name, description, is_protected, created_at, updated_at
FROM roles
WHERE slug = $1 AND deleted_at IS NULL;

-- name: AssignRole :exec
INSERT INTO user_roles (user_id, role_slug, assigned_by)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, role_slug) DO NOTHING;

-- name: RemoveRole :exec
DELETE FROM user_roles
WHERE user_id = $1 AND role_slug = $2;

-- name: ListUserRoles :many
SELECT r.slug, r.display_name, r.description, r.is_protected
FROM user_roles ur
JOIN roles r ON r.slug = ur.role_slug
WHERE ur.user_id = $1 AND r.deleted_at IS NULL
ORDER BY r.slug;

-- name: UserHasRole :one
SELECT EXISTS (
  SELECT 1 FROM user_roles WHERE user_id = $1 AND role_slug = $2
);
