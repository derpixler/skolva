package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestLoginEndpoint(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	hash, err := auth.HashPassword("s3cr3t-pw")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "u@example.com", PasswordHash: hash, FirstName: "T", LastName: "U",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	uid := u.ID
	if err := repo.AssignRole(ctx, uid, "mitglied", uuid.NullUUID{UUID: uid, Valid: true}); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	tm, err := auth.NewTokenManager("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	auth.RegisterRoutes(api, pool, tm)

	// valid login -> 200 with a token whose claims carry roles + permissions
	w := doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"u@example.com","password":"s3cr3t-pw"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected a non-empty token")
	}

	claims, err := tm.Verify(resp.Token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.Subject != uid.String() || claims.Email != "u@example.com" {
		t.Errorf("unexpected claims: subject=%s email=%s", claims.Subject, claims.Email)
	}
	hasMitglied := false
	for _, role := range claims.Roles {
		if role == "mitglied" {
			hasMitglied = true
		}
	}
	if !hasMitglied {
		t.Errorf("expected role mitglied in token, got %v", claims.Roles)
	}
	if len(claims.Permissions) == 0 {
		t.Error("expected resolved permissions in token claims")
	}

	// wrong password -> 401
	if w := doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"u@example.com","password":"wrong"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: expected 401, got %d", w.Code)
	}
	// unknown email -> 401 (no enumeration)
	if w := doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"nope@example.com","password":"x"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("unknown email: expected 401, got %d", w.Code)
	}
	// missing password -> 422
	if w := doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"u@example.com"}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing password: expected 422, got %d", w.Code)
	}
}
