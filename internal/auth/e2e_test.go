package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/core/middleware"
	"github.com/derpixler/skolva/internal/core/secrets"
	"github.com/derpixler/skolva/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

// TestE2ERegisterLogin2FA exercises the full user journey via HTTP only:
// admin registers a user -> login -> profile -> 2FA setup/confirm ->
// login-2fa -> verify -> access profile.
func TestE2ERegisterLogin2FA(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	// fixture: an admin user to perform the registration
	adminHash, _ := auth.HashPassword("adminpw")
	admin, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "admin@example.com", PasswordHash: adminHash, FirstName: "Ad", LastName: "Min",
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := repo.AssignRole(ctx, admin.ID, "admin", uuid.NullUUID{UUID: admin.ID, Valid: true}); err != nil {
		t.Fatalf("assign admin: %v", err)
	}

	// --- wiring ---
	tm, err := auth.NewTokenManager("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	cipher, err := secrets.NewCipher("test-encryption-key")
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	verify := auth.NewVerifier(tm)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Authenticate(verify))
	r.Use(middleware.ActorMiddleware())
	api := r.Group("/api")
	auth.RegisterRoutes(api, pool, tm, cipher, nil)

	// helper: unmarshal login/token response
	type credentials struct {
		Token       string `json:"token"`
		Requires2FA bool   `json:"requires_2fa"`
		TempToken   string `json:"temp_token"`
	}

	// --- 1) admin registers a new user ---
	adminToken := getToken(t, r, "admin@example.com", "adminpw", false)

	w := doReq(t, r, http.MethodPost, "/api/auth/register", adminToken, `{"email":"e2e@example.com","password":"password123","first_name":"E2E","last_name":"User"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("register: %d (%s)", w.Code, w.Body.String())
	}
	var user auth.UserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Fatalf("unmarshal user: %v", err)
	}
	uid := user.ID.String()

	// assign the new user a role that grants users.read (profile access)
	w = doReq(t, r, http.MethodPost, "/api/users/"+uid+"/roles", adminToken, `{"role_slug":"kassierer"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("assign mitglied: %d (%s)", w.Code, w.Body.String())
	}

	// --- 2) new user logs in (no 2FA) ---
	loginToken := getToken(t, r, "e2e@example.com", "password123", false)

	// --- 3) get the user's own profile ---
	w = doReq(t, r, http.MethodGet, "/api/users/"+uid, loginToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get profile: %d (%s)", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &user)
	if user.Email != "e2e@example.com" {
		t.Errorf("expected email e2e@example.com, got %s", user.Email)
	}

	// --- 4) setup + confirm 2FA ---
	w = doReq(t, r, http.MethodPost, "/api/auth/2fa/setup", loginToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("setup 2fa: %d (%s)", w.Code, w.Body.String())
	}
	var setupResp auth.Setup2FAResponse
	if err := json.Unmarshal(w.Body.Bytes(), &setupResp); err != nil {
		t.Fatalf("unmarshal setup: %v", err)
	}
	parsed, _ := url.Parse(setupResp.ProvisioningURI)
	totpSecret := parsed.Query().Get("secret")
	code, err := totp.GenerateCode(totpSecret, time.Now())
	if err != nil {
		t.Fatalf("generate TOTP: %v", err)
	}

	w = doReq(t, r, http.MethodPost, "/api/auth/2fa/confirm", loginToken, `{"code":"`+code+`"}`)
	if w.Code != http.StatusNoContent {
		t.Fatalf("confirm 2fa: %d (%s)", w.Code, w.Body.String())
	}

	// --- 5) login now requires 2FA ---
	w = doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"e2e@example.com","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login (2fa): %d (%s)", w.Code, w.Body.String())
	}
	var creds credentials
	_ = json.Unmarshal(w.Body.Bytes(), &creds)
	if !creds.Requires2FA || creds.TempToken == "" {
		t.Fatalf("expected requires_2fa: %+v", creds)
	}

	// --- 6) verify 2FA -> full token ---
	code, _ = totp.GenerateCode(totpSecret, time.Now())
	w = doReq(t, r, http.MethodPost, "/api/auth/2fa/verify", "", `{"temp_token":"`+creds.TempToken+`","code":"`+code+`"}`)
	_ = json.Unmarshal(w.Body.Bytes(), &creds)
	if w.Code != http.StatusOK || creds.Token == "" {
		t.Fatalf("verify: %d resp=%+v", w.Code, creds)
	}

	// --- 7) access profile with the full token ---
	twoFAToken := creds.Token
	w = doReq(t, r, http.MethodGet, "/api/users/"+uid, twoFAToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get profile after 2fa: %d (%s)", w.Code, w.Body.String())
	}
}

func getToken(t *testing.T, r http.Handler, email, password string, expect2FA bool) string {
	t.Helper()
	w := doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"`+email+`","password":"`+password+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login %s: %d (%s)", email, w.Code, w.Body.String())
	}
	var c struct {
		Token       string `json:"token"`
		Requires2FA bool   `json:"requires_2fa"`
		TempToken   string `json:"temp_token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	if expect2FA {
		if !c.Requires2FA || c.TempToken == "" {
			t.Fatalf("expected requires_2fa for %s, got %+v", email, c)
		}
		return c.TempToken
	}
	if c.Token == "" {
		t.Fatalf("expected a direct token for %s, got %+v", email, c)
	}
	return c.Token
}
