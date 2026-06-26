package auth_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/core/mail"
	"github.com/derpixler/skolva/internal/core/metadata"
	"github.com/derpixler/skolva/internal/core/secrets"
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
	auth.RegisterRoutes(api, pool, tm, cipher, mail.NewNoopMailer())

	// forgot -> 200 (no user enumeration)
	w := doReq(t, r, http.MethodPost, "/api/auth/password/forgot", "", `{"email":"nonexistent@example.com"}`)
	if w.Code != http.StatusOK {
		t.Errorf("forgot unknown: expected 200, got %d", w.Code)
	}
	// forgot -> 200 (sends email)
	w = doReq(t, r, http.MethodPost, "/api/auth/password/forgot", "", `{"email":"reset@example.com"}`)
	if w.Code != http.StatusOK {
		t.Errorf("forgot: expected 200, got %d", w.Code)
	}

	// reset with wrong token -> 422
	w = doReq(t, r, http.MethodPost, "/api/auth/password/reset", "", `{"user_id":"`+user.ID.String()+`","token":"wrong-token","password":"newpass123"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("reset wrong token: expected 422, got %d", w.Code)
	}

	// reset with invalid user_id -> 422
	w = doReq(t, r, http.MethodPost, "/api/auth/password/reset", "", `{"user_id":"not-a-uuid","token":"x","password":"newpass123"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("reset invalid id: expected 422, got %d", w.Code)
	}

	// missing fields -> 422
	w = doReq(t, r, http.MethodPost, "/api/auth/password/forgot", "", `{}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("forgot missing: expected 422, got %d", w.Code)
	}
	// short password -> 422
	w = doReq(t, r, http.MethodPost, "/api/auth/password/forgot", "", `{"email":"a@b.c"}`)
	if w.Code != http.StatusOK {
		t.Errorf("forgot valid: expected 200, got %d", w.Code)
	}
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

	// reset with wrong token -> 422
	w := doReq(t, r, http.MethodPost, "/api/auth/password/reset", "", `{"user_id":"`+user.ID.String()+`","token":"wrong","password":"newpass123"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("reset wrong token: expected 422, got %d", w.Code)
	}

	// reset with expired token -> 422
	_ = meta.Set(ctx, pool, user.ID, "auth.reset.expires_at", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
	w = doReq(t, r, http.MethodPost, "/api/auth/password/reset", "", `{"user_id":"`+user.ID.String()+`","token":"`+token+`","password":"newpass123"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("reset expired: expected 422, got %d", w.Code)
	}

	// reset with already-used token -> 422
	_ = meta.Set(ctx, pool, user.ID, "auth.reset.expires_at", time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	_ = meta.Set(ctx, pool, user.ID, "auth.reset.used", "true")
	w = doReq(t, r, http.MethodPost, "/api/auth/password/reset", "", `{"user_id":"`+user.ID.String()+`","token":"`+token+`","password":"newpass123"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("reset used: expected 422, got %d", w.Code)
	}

	// reset succeeds with fresh token and future expiry
	_ = meta.Delete(ctx, pool, user.ID, "auth.reset.used")
	_ = meta.Set(ctx, pool, user.ID, "auth.reset.expires_at", time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	if w := doReq(t, r, http.MethodPost, "/api/auth/password/reset", "", `{"user_id":"`+user.ID.String()+`","token":"`+token+`","password":"newpass123"}`); w.Code != http.StatusNoContent {
		t.Errorf("reset success: expected 204, got %d", w.Code)
	}
}
