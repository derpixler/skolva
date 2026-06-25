-- name: ListPermissions :many
SELECT slug, description, is_protected
FROM permissions
ORDER BY slug;

-- name: GetPermissionsForUser :many
SELECT DISTINCT p.slug
FROM user_roles ur
JOIN roles r ON r.slug = ur.role_slug AND r.deleted_at IS NULL
JOIN role_permissions rp ON rp.role_slug = ur.role_slug
JOIN permissions p ON p.slug = rp.permission_slug
WHERE ur.user_id = $1
ORDER BY p.slug;

-- name: GetPermissionsForRole :many
SELECT p.slug, p.description, p.is_protected
FROM role_permissions rp
JOIN permissions p ON p.slug = rp.permission_slug
WHERE rp.role_slug = $1
ORDER BY p.slug;

-- name: AddRolePermission :exec
INSERT INTO role_permissions (role_slug, permission_slug)
VALUES ($1, $2)
ON CONFLICT (role_slug, permission_slug) DO NOTHING;

-- name: RemoveRolePermission :exec
DELETE FROM role_permissions
WHERE role_slug = $1 AND permission_slug = $2;
