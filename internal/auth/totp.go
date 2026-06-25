package auth

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"time"

	apperrors "github.com/derpixler/skolva/internal/core/errors"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxFailedAttempts = 5
	lockoutDuration   = 15 * time.Minute
	recoveryCodeCount = 10
	recoveryCodeLen   = 16
)

// Setup2FA generates a TOTP secret and 10 single-use recovery codes, stores
// them (disabled until confirmed), and returns the provisioning URI (to
// display as a QR code) together with the plaintext recovery codes (shown
// once). The user must call Confirm2FA with a valid TOTP code to activate.
func (s *Service) Setup2FA(ctx context.Context, userID uuid.UUID) (provisioningURI string, recoveryCodes []string, err error) {
	existing, _ := s.repo.GetTOTPSecret(ctx, userID)
	if existing.IsEnabled {
		return "", nil, apperrors.NewConflict("2FA is already set up and active")
	}

	u, err := s.GetUser(ctx, userID)
	if err != nil {
		return "", nil, err
	}

	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Skolva", AccountName: u.Email})
	if err != nil {
		return "", nil, err
	}

	encrypted, err := s.cipher.Encrypt(key.Secret())
	if err != nil {
		return "", nil, err
	}

	codes := make([]string, recoveryCodeCount)
	hashes := make([]string, recoveryCodeCount)
	for i := range codes {
		code, hash, err := generateRecoveryCode()
		if err != nil {
			return "", nil, err
		}
		codes[i] = code
		hashes[i] = hash
	}

	if _, err := s.repo.UpsertTOTPSecret(ctx, db.UpsertTOTPSecretParams{
		UserID:            userID,
		SecretEncrypted:   encrypted,
		RecoveryCodesHash: hashes,
	}); err != nil {
		return "", nil, err
	}

	return key.URL(), codes, nil
}

// Confirm2FA validates a TOTP code and activates 2FA for the user.
func (s *Service) Confirm2FA(ctx context.Context, userID uuid.UUID, code string) error {
	row, err := s.repo.GetTOTPSecret(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NewNotFound("2FA not set up")
		}
		return err
	}
	if row.IsEnabled {
		return apperrors.NewConflict("2FA is already active")
	}

	secret, err := s.cipher.Decrypt(row.SecretEncrypted)
	if err != nil {
		return err
	}
	if !totp.Validate(code, secret) {
		return apperrors.NewValidation("invalid TOTP code")
	}

	now := time.Now()
	if _, err := s.repo.UpsertTOTPSecret(ctx, db.UpsertTOTPSecretParams{
		UserID:            userID,
		SecretEncrypted:   row.SecretEncrypted,
		IsEnabled:         true,
		VerifiedAt:        pgtype.Timestamptz{Time: now, Valid: true, InfinityModifier: pgtype.Finite},
		RecoveryCodesHash: row.RecoveryCodesHash,
	}); err != nil {
		return err
	}
	return nil
}

// Verify2FA validates a TOTP code for an active 2FA session. On success
// failed-attempts are reset; on failure the counter is incremented and after
// maxFailedAttempts the account is temporarily locked.
func (s *Service) Verify2FA(ctx context.Context, userID uuid.UUID, code string) error {
	row, err := s.repo.GetTOTPSecret(ctx, userID)
	if err != nil || !row.IsEnabled {
		return apperrors.NewUnauthorized("2FA is not active for this account")
	}
	if row.LockedUntil.Valid && time.Now().Before(row.LockedUntil.Time) {
		return apperrors.NewForbidden("account is temporarily locked due to too many failed attempts")
	}

	secret, err := s.cipher.Decrypt(row.SecretEncrypted)
	if err != nil {
		return err
	}

	if totp.Validate(code, secret) {
		now := time.Now()
		if _, err := s.repo.UpsertTOTPSecret(ctx, db.UpsertTOTPSecretParams{
			UserID: userID, SecretEncrypted: row.SecretEncrypted, IsEnabled: true,
			VerifiedAt: row.VerifiedAt, RecoveryCodesHash: row.RecoveryCodesHash,
			LastUsedAt:     pgtype.Timestamptz{Time: now, Valid: true, InfinityModifier: pgtype.Finite},
			FailedAttempts: 0,
		}); err != nil {
			return err
		}
		return nil
	}

	// invalid code: increment failures, lock if threshold reached
	failed := row.FailedAttempts + 1
	var lock pgtype.Timestamptz
	if failed >= maxFailedAttempts {
		lock = pgtype.Timestamptz{Time: time.Now().Add(lockoutDuration), Valid: true, InfinityModifier: pgtype.Finite}
	}
	if _, err := s.repo.UpsertTOTPSecret(ctx, db.UpsertTOTPSecretParams{
		UserID: userID, SecretEncrypted: row.SecretEncrypted, IsEnabled: true,
		VerifiedAt: row.VerifiedAt, RecoveryCodesHash: row.RecoveryCodesHash,
		FailedAttempts: failed, LockedUntil: lock,
	}); err != nil {
		return err
	}
	return apperrors.NewUnauthorized("invalid TOTP code")
}

// ConsumeRecoveryCode validates a single-use recovery code and removes it
// from the stored list on success. Recovery codes are bcrypt-hashed.
func (s *Service) ConsumeRecoveryCode(ctx context.Context, userID uuid.UUID, code string) error {
	row, err := s.repo.GetTOTPSecret(ctx, userID)
	if err != nil || !row.IsEnabled {
		return apperrors.NewUnauthorized("recovery is not available")
	}

	found := -1
	for i, h := range row.RecoveryCodesHash {
		if bcrypt.CompareHashAndPassword([]byte(h), []byte(code)) == nil {
			found = i
			break
		}
	}
	if found < 0 {
		return apperrors.NewUnauthorized("invalid recovery code")
	}

	if _, err := s.repo.UpsertTOTPSecret(ctx, db.UpsertTOTPSecretParams{
		UserID:            userID,
		SecretEncrypted:   row.SecretEncrypted,
		IsEnabled:         true,
		VerifiedAt:        row.VerifiedAt,
		FailedAttempts:    0,
		RecoveryCodesHash: append(row.RecoveryCodesHash[:found], row.RecoveryCodesHash[found+1:]...),
	}); err != nil {
		return err
	}
	return nil
}

// Disable2FA verifies a TOTP code and deletes the 2FA configuration.
func (s *Service) Disable2FA(ctx context.Context, userID uuid.UUID, code string) error {
	if err := s.Verify2FA(ctx, userID, code); err != nil {
		return apperrors.NewUnauthorized("invalid code; 2FA remains active")
	}
	return s.repo.DeleteTOTPSecret(ctx, userID)
}

// IssueAccessForUser produces a full access token (JWT) for a verified
// user, resolving their roles and permissions from the database.
func (s *Service) IssueAccessForUser(ctx context.Context, userID uuid.UUID) (string, error) {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	roleRows, err := s.repo.ListUserRoles(ctx, userID)
	if err != nil {
		return "", err
	}
	roles := make([]string, len(roleRows))
	for i, r := range roleRows {
		roles[i] = r.Slug
	}
	perms, err := s.repo.GetPermissionsForUser(ctx, userID)
	if err != nil {
		return "", err
	}
	return s.tm.IssueAccess(userID.String(), u.Email, roles, perms)
}

func generateRecoveryCode() (plain, bcryptHash string, err error) {
	buf := make([]byte, recoveryCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	plain = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)[:recoveryCodeLen]
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", "", err
	}
	return plain, string(hash), nil
}
