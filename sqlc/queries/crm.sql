-- name: GetUserAddress :one
SELECT user_id, company, care_of, street1, street2, postal_code, city, state, country_code, note, created_at, updated_at
FROM user_address
WHERE user_id = $1;

-- name: UpsertUserAddress :one
INSERT INTO user_address (
  user_id, company, care_of, street1, street2, postal_code, city, state, country_code, note, created_by, updated_by
) VALUES (
  sqlc.arg(user_id), sqlc.arg(company), sqlc.arg(care_of), sqlc.arg(street1), sqlc.arg(street2),
  sqlc.arg(postal_code), sqlc.arg(city), sqlc.arg(state), sqlc.arg(country_code), sqlc.arg(note),
  sqlc.arg(actor), sqlc.arg(actor)
)
ON CONFLICT (user_id) DO UPDATE SET
  company = EXCLUDED.company,
  care_of = EXCLUDED.care_of,
  street1 = EXCLUDED.street1,
  street2 = EXCLUDED.street2,
  postal_code = EXCLUDED.postal_code,
  city = EXCLUDED.city,
  state = EXCLUDED.state,
  country_code = EXCLUDED.country_code,
  note = EXCLUDED.note,
  updated_by = EXCLUDED.updated_by
RETURNING user_id, company, care_of, street1, street2, postal_code, city, state, country_code, note, created_at, updated_at;

-- name: GetUserPreferences :one
SELECT user_id, preferred_contact_type, note, updated_at
FROM user_preferences
WHERE user_id = $1;

-- name: UpsertUserPreferences :one
INSERT INTO user_preferences (user_id, preferred_contact_type, note, updated_by)
VALUES (sqlc.arg(user_id), sqlc.arg(preferred_contact_type), sqlc.arg(note), sqlc.arg(actor))
ON CONFLICT (user_id) DO UPDATE SET
  preferred_contact_type = EXCLUDED.preferred_contact_type,
  note = EXCLUDED.note,
  updated_by = EXCLUDED.updated_by
RETURNING user_id, preferred_contact_type, note, updated_at;
