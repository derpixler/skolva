-- name: GetTOTPSecret :one
SELECT user_id, secret_encrypted, is_enabled, verified_at, recovery_codes_hash,
       last_used_at, failed_attempts, locked_until, created_at, updated_at
FROM user_totp_secrets
WHERE user_id = $1;

-- name: UpsertTOTPSecret :one
INSERT INTO user_totp_secrets (
  user_id, secret_encrypted, is_enabled, verified_at, recovery_codes_hash,
  last_used_at, failed_attempts, locked_until
) VALUES (
  sqlc.arg(user_id), sqlc.arg(secret_encrypted), sqlc.arg(is_enabled), sqlc.arg(verified_at),
  sqlc.arg(recovery_codes_hash), sqlc.arg(last_used_at), sqlc.arg(failed_attempts), sqlc.arg(locked_until)
)
ON CONFLICT (user_id) DO UPDATE SET
  secret_encrypted = EXCLUDED.secret_encrypted,
  is_enabled = EXCLUDED.is_enabled,
  verified_at = EXCLUDED.verified_at,
  recovery_codes_hash = EXCLUDED.recovery_codes_hash,
  last_used_at = EXCLUDED.last_used_at,
  failed_attempts = EXCLUDED.failed_attempts,
  locked_until = EXCLUDED.locked_until
RETURNING user_id, secret_encrypted, is_enabled, verified_at, recovery_codes_hash,
          last_used_at, failed_attempts, locked_until, created_at, updated_at;

-- name: DeleteTOTPSecret :exec
DELETE FROM user_totp_secrets WHERE user_id = $1;
