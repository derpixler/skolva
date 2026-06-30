package auth_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/derpixler/skolva-core/mail"
	"github.com/derpixler/skolva-core/metadata"
	"github.com/derpixler/skolva-core/secrets"
	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func TestPasswordReset(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()

	// Create a user with a known password
	repo := auth.NewRepository(pool)
	hash, _ := auth.HashPassword("old-password")
	user, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "reset@example.com", PasswordHash: hash, FirstName: "Reset", LastName: "User",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	tm, _ := auth.NewTokenManager("s", 1)
	cipher, _ := secrets.NewCipher("k")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	mailer := mail.NewNoopMailer()
	auth.RegisterRoutes(api, pool, tm, cipher, mailer)

	// forgot -> always 200 (no user enumeration), unknown + existing email
	tstep(t, "forgot password (unknown + existing email, both 200)")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/password/forgot", "", `{"email":"nonexistent@example.com"}`),
		http.StatusOK, "forgot unknown email")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/password/forgot", "", `{"email":"reset@example.com"}`),
		http.StatusOK, "forgot existing email")
	if sent := mailer.Sent(); len(sent) > 0 {
		last := sent[len(sent)-1]
		tlog(t, "[mail] to=%v subject=%q (reset link delivered)", last.To, last.Subject)
	}

	// reset negative cases
	tstep(t, "reset password negative cases")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/password/reset", "", `{"user_id":"`+user.ID.String()+`","token":"wrong-token","password":"newpass123"}`),
		http.StatusUnprocessableEntity, "reset wrong token")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/password/reset", "", `{"user_id":"not-a-uuid","token":"x","password":"newpass123"}`),
		http.StatusUnprocessableEntity, "reset invalid user_id")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/password/forgot", "", `{}`),
		http.StatusUnprocessableEntity, "forgot missing email")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/password/forgot", "", `{"email":"a@b.c"}`),
		http.StatusOK, "forgot any email (no enumeration)")
}

func TestPasswordResetExpiryAndReplay(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()

	repo := auth.NewRepository(pool)
	hash, _ := auth.HashPassword("old-password")
	user, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "reset2@example.com", PasswordHash: hash, FirstName: "R2", LastName: "D2",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	tm, _ := auth.NewTokenManager("s", 1)
	cipher, _ := secrets.NewCipher("k")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	auth.RegisterRoutes(api, pool, tm, cipher, mail.NewNoopMailer())

	meta, err := metadata.NewStore("users_meta")
	if err != nil {
		t.Fatalf("new metadata store: %v", err)
	}

	// seed a valid reset token (so we can test expiry + used without calling forgot)
	token := uuid.NewString()
	tokenHash, _ := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	_ = meta.Set(ctx, pool, user.ID, "auth.reset.token_hash", string(tokenHash))
	_ = meta.Set(ctx, pool, user.ID, "auth.reset.expires_at", time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	tstep(t, "seeded a valid reset token (expiry +1h)")
	tlog(t, "[val ] reset token=%s user=%s", token, user.ID)

	// reset with wrong token -> 422
	tstep(t, "reset with wrong token")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/password/reset", "", `{"user_id":"`+user.ID.String()+`","token":"wrong","password":"newpass123"}`),
		http.StatusUnprocessableEntity, "reset wrong token")

	// reset with expired token -> 422
	tstep(t, "expire the token, then reset")
	_ = meta.Set(ctx, pool, user.ID, "auth.reset.expires_at", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/password/reset", "", `{"user_id":"`+user.ID.String()+`","token":"`+token+`","password":"newpass123"}`),
		http.StatusUnprocessableEntity, "reset expired token")

	// reset with already-used token -> 422
	tstep(t, "mark token used, then reset")
	_ = meta.Set(ctx, pool, user.ID, "auth.reset.expires_at", time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	_ = meta.Set(ctx, pool, user.ID, "auth.reset.used", "true")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/password/reset", "", `{"user_id":"`+user.ID.String()+`","token":"`+token+`","password":"newpass123"}`),
		http.StatusUnprocessableEntity, "reset already-used token")

	// reset succeeds with fresh token and future expiry
	tstep(t, "clear used + future expiry, then reset succeeds")
	_ = meta.Delete(ctx, pool, user.ID, "auth.reset.used")
	_ = meta.Set(ctx, pool, user.ID, "auth.reset.expires_at", time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/password/reset", "", `{"user_id":"`+user.ID.String()+`","token":"`+token+`","password":"newpass123"}`),
		http.StatusNoContent, "reset success (fresh token)")
}
