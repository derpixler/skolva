-- name: AddMember :exec
INSERT INTO group_members (group_id, user_id, role_in_group, created_by)
VALUES (sqlc.arg(group_id), sqlc.arg(user_id), sqlc.arg(role_in_group), sqlc.arg(created_by))
ON CONFLICT (group_id, user_id) DO UPDATE SET role_in_group = EXCLUDED.role_in_group;

-- name: RemoveMember :exec
DELETE FROM group_members
WHERE group_id = $1 AND user_id = $2;

-- name: ListMembers :many
SELECT gm.user_id, u.first_name, u.last_name, u.email, gm.role_in_group, gm.joined_at
FROM group_members gm
JOIN users u ON u.id = gm.user_id AND u.deleted_at IS NULL
WHERE gm.group_id = $1
ORDER BY u.last_name, u.first_name, gm.user_id;

-- name: ListUserGroups :many
SELECT g.id, g.name, g.group_type, gm.role_in_group, gm.joined_at
FROM group_members gm
JOIN groups g ON g.id = gm.group_id AND g.deleted_at IS NULL
WHERE gm.user_id = $1
ORDER BY g.name, g.id;
