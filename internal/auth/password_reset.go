package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	apperrors "github.com/derpixler/skolva-core/errors"
	"github.com/derpixler/skolva-core/mail"
	"github.com/derpixler/skolva-core/metadata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const resetTokenExpiry = 1 * time.Hour

// ForgotPassword initiates a password reset. The user receives a reset link
// via email if an account with the given address exists. The response is
// always 200 to prevent user enumeration.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // no enumeration
		}
		return err
	}

	token := uuid.NewString()
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	meta, err := metadata.NewStore("users_meta")
	if err != nil {
		return err
	}
	expires := time.Now().Add(resetTokenExpiry).UTC().Format(time.RFC3339)
	if err := meta.Set(ctx, s.repo.pool, u.ID, "auth.reset.token_hash", string(hash)); err != nil {
		return err
	}
	if err := meta.Set(ctx, s.repo.pool, u.ID, "auth.reset.expires_at", expires); err != nil {
		return err
	}
	// clear any previous "used" flag
	_ = meta.Delete(ctx, s.repo.pool, u.ID, "auth.reset.used")

	_ = s.mailer.Send(ctx, mail.Message{
		To:      []string{u.Email},
		Subject: "Passwort zurücksetzen — Skolva",
		Body: fmt.Sprintf(
			"Klicke zum Zurücksetzen deines Passworts:\n\n"+
				"  %s/reset?user=%s&token=%s\n\n"+
				"Der Link ist %d Stunde(n) gültig.",
			"https://skolva.example.com", u.ID.String(), token, int(resetTokenExpiry.Hours()),
		),
	})

	return nil
}

// ResetPassword validates a reset token and updates the user's password.
func (s *Service) ResetPassword(ctx context.Context, userID uuid.UUID, token, newPassword string) error {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NewNotFound("user")
		}
		return err
	}
	_ = u

	meta, err := metadata.NewStore("users_meta")
	if err != nil {
		return err
	}

	// check if already used
	if used, _, _ := meta.Get(ctx, s.repo.pool, userID, "auth.reset.used"); used == "true" {
		return apperrors.NewValidation("reset link has already been used")
	}

	// check expiry
	expiresStr, found, err := meta.Get(ctx, s.repo.pool, userID, "auth.reset.expires_at")
	if err != nil || !found {
		return apperrors.NewValidation("no active reset request")
	}
	expires, err := time.Parse(time.RFC3339, expiresStr)
	if err != nil || time.Now().After(expires) {
		return apperrors.NewValidation("reset link has expired")
	}

	// verify token
	tokenHash, found, err := meta.Get(ctx, s.repo.pool, userID, "auth.reset.token_hash")
	if err != nil || !found {
		return apperrors.NewValidation("no active reset request")
	}
	if bcrypt.CompareHashAndPassword([]byte(tokenHash), []byte(token)) != nil {
		return apperrors.NewValidation("invalid reset link")
	}

	// update password
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}

	// mark token as used
	if err := meta.Set(ctx, s.repo.pool, userID, "auth.reset.used", "true"); err != nil {
		return err
	}

	return nil
}
