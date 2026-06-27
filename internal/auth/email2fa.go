package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"time"

	apperrors "github.com/derpixler/skolva/internal/core/errors"
	"github.com/derpixler/skolva/internal/core/mail"
	"github.com/derpixler/skolva/internal/core/metadata"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Email-2FA is a standalone second factor that delivers a one-time numeric
// code by email. Its transient state and the enabled flag live in the
// users_meta EAV store (the same mechanism the password reset uses); no
// dedicated table is required. Brute-force protection reuses the TOTP
// thresholds (maxFailedAttempts / lockoutDuration, defined in totp.go).
const (
	email2faOTPExpiry = 10 * time.Minute

	email2faKeyEnabled  = "auth.email2fa.enabled"
	email2faKeyOTPHash  = "auth.email2fa.otp_hash"
	email2faKeyExpires  = "auth.email2fa.expires_at"
	email2faKeyAttempts = "auth.email2fa.attempts"
	email2faKeyLocked   = "auth.email2fa.locked_until"
)

func (s *Service) email2faStore() (*metadata.Store, error) {
	return metadata.NewStore("users_meta")
}

// isEmail2FAEnabled reports whether the user has activated email-2FA.
func (s *Service) isEmail2FAEnabled(ctx context.Context, userID uuid.UUID) (bool, error) {
	store, err := s.email2faStore()
	if err != nil {
		return false, err
	}
	v, found, err := store.Get(ctx, s.repo.pool, userID, email2faKeyEnabled)
	if err != nil {
		return false, err
	}
	return found && v == "true", nil
}

// SetupEmail2FA starts activation by emailing a confirmation code. The factor
// is not active until ConfirmEmail2FA succeeds.
func (s *Service) SetupEmail2FA(ctx context.Context, userID uuid.UUID) error {
	enabled, err := s.isEmail2FAEnabled(ctx, userID)
	if err != nil {
		return err
	}
	if enabled {
		return apperrors.NewConflict("email 2FA is already active")
	}
	u, err := s.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	return s.issueEmailOTP(ctx, userID, u.Email,
		"Email-2FA bestätigen — Skolva",
		"Bestätige die Aktivierung der Zwei-Faktor-Authentifizierung per E-Mail.")
}

// ConfirmEmail2FA validates the emailed setup code and activates email-2FA.
func (s *Service) ConfirmEmail2FA(ctx context.Context, userID uuid.UUID, code string) error {
	enabled, err := s.isEmail2FAEnabled(ctx, userID)
	if err != nil {
		return err
	}
	if enabled {
		return apperrors.NewConflict("email 2FA is already active")
	}
	if err := s.verifyEmailOTP(ctx, userID, code); err != nil {
		return err
	}
	store, err := s.email2faStore()
	if err != nil {
		return err
	}
	return store.Set(ctx, s.repo.pool, userID, email2faKeyEnabled, "true")
}

// SendEmail2FALoginOTP emails a fresh login code. It is a no-op when the user
// has not enabled email-2FA. Called automatically at login and by resend.
func (s *Service) SendEmail2FALoginOTP(ctx context.Context, userID uuid.UUID) error {
	enabled, err := s.isEmail2FAEnabled(ctx, userID)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	u, err := s.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	return s.issueEmailOTP(ctx, userID, u.Email,
		"Dein Anmelde-Code — Skolva",
		"Verwende diesen Code, um deine Anmeldung abzuschließen.")
}

// VerifyEmail2FALogin validates an emailed login code for an active email-2FA
// session. The caller issues the access token on success.
func (s *Service) VerifyEmail2FALogin(ctx context.Context, userID uuid.UUID, code string) error {
	enabled, err := s.isEmail2FAEnabled(ctx, userID)
	if err != nil {
		return err
	}
	if !enabled {
		return apperrors.NewUnauthorized("email 2FA is not active for this account")
	}
	return s.verifyEmailOTP(ctx, userID, code)
}

// DisableEmail2FA deactivates email-2FA and clears all transient state.
func (s *Service) DisableEmail2FA(ctx context.Context, userID uuid.UUID) error {
	store, err := s.email2faStore()
	if err != nil {
		return err
	}
	for _, k := range []string{
		email2faKeyEnabled, email2faKeyOTPHash, email2faKeyExpires,
		email2faKeyAttempts, email2faKeyLocked,
	} {
		if err := store.Delete(ctx, s.repo.pool, userID, k); err != nil {
			return err
		}
	}
	return nil
}

// issueEmailOTP generates a 6-digit code, stores its bcrypt hash with an
// expiry, resets the failure counter and emails the code.
func (s *Service) issueEmailOTP(ctx context.Context, userID uuid.UUID, email, subject, intro string) error {
	store, err := s.email2faStore()
	if err != nil {
		return err
	}
	otp, err := generateNumericOTP()
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	pool := s.repo.pool
	if err := store.Set(ctx, pool, userID, email2faKeyOTPHash, string(hash)); err != nil {
		return err
	}
	expires := time.Now().Add(email2faOTPExpiry).UTC().Format(time.RFC3339)
	if err := store.Set(ctx, pool, userID, email2faKeyExpires, expires); err != nil {
		return err
	}
	_ = store.Delete(ctx, pool, userID, email2faKeyAttempts)
	_ = store.Delete(ctx, pool, userID, email2faKeyLocked)

	_ = s.mailer.Send(ctx, mail.Message{
		To:      []string{email},
		Subject: subject,
		Body: fmt.Sprintf("%s\n\nDein Code: %s\n\nDer Code ist %d Minuten gültig.",
			intro, otp, int(email2faOTPExpiry.Minutes())),
	})
	return nil
}

// verifyEmailOTP checks a code against the stored hash, enforcing expiry and
// brute-force lockout. On success the transient state is cleared.
func (s *Service) verifyEmailOTP(ctx context.Context, userID uuid.UUID, code string) error {
	store, err := s.email2faStore()
	if err != nil {
		return err
	}
	pool := s.repo.pool

	if lockedStr, found, _ := store.Get(ctx, pool, userID, email2faKeyLocked); found {
		if until, perr := time.Parse(time.RFC3339, lockedStr); perr == nil && time.Now().Before(until) {
			return apperrors.NewForbidden("account is temporarily locked due to too many failed attempts")
		}
	}

	expiresStr, found, err := store.Get(ctx, pool, userID, email2faKeyExpires)
	if err != nil {
		return err
	}
	if !found {
		return apperrors.NewValidation("no active code; request a new one")
	}
	if expires, perr := time.Parse(time.RFC3339, expiresStr); perr != nil || time.Now().After(expires) {
		return apperrors.NewValidation("code has expired; request a new one")
	}

	hash, found, err := store.Get(ctx, pool, userID, email2faKeyOTPHash)
	if err != nil {
		return err
	}
	if !found {
		return apperrors.NewValidation("no active code; request a new one")
	}

	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) != nil {
		attempts := 0
		if a, ok, _ := store.Get(ctx, pool, userID, email2faKeyAttempts); ok {
			attempts, _ = strconv.Atoi(a)
		}
		attempts++
		_ = store.Set(ctx, pool, userID, email2faKeyAttempts, strconv.Itoa(attempts))
		if attempts >= maxFailedAttempts {
			until := time.Now().Add(lockoutDuration).UTC().Format(time.RFC3339)
			_ = store.Set(ctx, pool, userID, email2faKeyLocked, until)
		}
		return apperrors.NewUnauthorized("invalid code")
	}

	_ = store.Delete(ctx, pool, userID, email2faKeyOTPHash)
	_ = store.Delete(ctx, pool, userID, email2faKeyExpires)
	_ = store.Delete(ctx, pool, userID, email2faKeyAttempts)
	_ = store.Delete(ctx, pool, userID, email2faKeyLocked)
	return nil
}

func generateNumericOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
