-- name: ListContacts :many
SELECT id, user_id, contact_type, label, value, is_primary, is_preferred,
       allow_contact, preferred_time_window, verified_at, note, created_at, updated_at
FROM user_contact_points
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY contact_type, created_at, id;

-- name: GetContact :one
SELECT id, user_id, contact_type, label, value, is_primary, is_preferred,
       allow_contact, preferred_time_window, verified_at, note, created_at, updated_at
FROM user_contact_points
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateContact :one
INSERT INTO user_contact_points (
  user_id, contact_type, label, value, is_primary, is_preferred,
  allow_contact, preferred_time_window, note, created_by, updated_by
) VALUES (
  sqlc.arg(user_id), sqlc.arg(contact_type), sqlc.arg(label), sqlc.arg(value),
  sqlc.arg(is_primary), sqlc.arg(is_preferred), sqlc.arg(allow_contact),
  sqlc.arg(preferred_time_window), sqlc.arg(note), sqlc.arg(actor), sqlc.arg(actor)
)
RETURNING id, user_id, contact_type, label, value, is_primary, is_preferred,
          allow_contact, preferred_time_window, verified_at, note, created_at, updated_at;

-- name: UpdateContact :one
UPDATE user_contact_points
SET label = sqlc.arg(label),
    value = sqlc.arg(value),
    is_primary = sqlc.arg(is_primary),
    is_preferred = sqlc.arg(is_preferred),
    allow_contact = sqlc.arg(allow_contact),
    preferred_time_window = sqlc.arg(preferred_time_window),
    note = sqlc.arg(note),
    updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING id, user_id, contact_type, label, value, is_primary, is_preferred,
          allow_contact, preferred_time_window, verified_at, note, created_at, updated_at;

-- name: SoftDeleteContact :exec
UPDATE user_contact_points
SET deleted_at = NOW(), updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ClearPrimaryContacts :exec
UPDATE user_contact_points
SET is_primary = FALSE
WHERE user_id = sqlc.arg(user_id)
  AND contact_type = sqlc.arg(contact_type)
  AND deleted_at IS NULL
  AND is_primary = TRUE
  AND id <> sqlc.arg(exclude_id);
