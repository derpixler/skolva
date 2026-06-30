package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/derpixler/skolva-core/middleware"
	"github.com/derpixler/skolva-core/secrets"
	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

func Test2FAFlow(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	// create a user with a known password
	hash, _ := auth.HashPassword("password123")
	_, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "2fa@example.com", PasswordHash: hash, FirstName: "Two", LastName: "Fa",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

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

	// 1) login without 2FA -> full access token
	tstep(t, "login without 2FA -> full token")
	w := doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"2fa@example.com","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Token       string `json:"token"`
		Requires2FA bool   `json:"requires_2fa"`
		TempToken   string `json:"temp_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Token == "" || resp.Requires2FA {
		t.Fatalf("unexpected login response: %+v err=%v", resp, err)
	}
	bearer := resp.Token

	// 2) setup 2FA -> provisioning URI + recovery codes
	w = doReq(t, r, http.MethodPost, "/api/auth/2fa/setup", bearer, "")
	if w.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", w.Code, w.Body.String())
	}
	var setupResp auth.Setup2FAResponse
	if err := json.Unmarshal(w.Body.Bytes(), &setupResp); err != nil {
		t.Fatalf("unmarshal setup: %v", err)
	}
	if setupResp.ProvisioningURI == "" || len(setupResp.RecoveryCodes) != 10 {
		t.Fatalf("unexpected setup response: %+v", setupResp)
	}

	// extract TOTP secret from provisioning URI and generate a valid code
	parsed, _ := url.Parse(setupResp.ProvisioningURI)
	totpSecret := parsed.Query().Get("secret")
	validCode, err := totp.GenerateCode(totpSecret, time.Now())
	if err != nil || totpSecret == "" {
		t.Fatalf("generate TOTP code (secret=%q): err=%v", totpSecret, err)
	}
	tlog(t, "[val ] TOTP secret len=%d, recovery codes=%d, code=%s",
		len(totpSecret), len(setupResp.RecoveryCodes), validCode)

	// 3) confirm 2FA — wrong code first, then valid
	tstep(t, "confirm 2FA (wrong then valid)")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/confirm", bearer, `{"code":"000000"}`),
		http.StatusUnprocessableEntity, "confirm wrong code")
	if !assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/confirm", bearer, `{"code":"`+validCode+`"}`),
		http.StatusNoContent, "confirm valid code") {
		t.FailNow()
	}

	// 4) login now returns requires_2fa
	w = doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"2fa@example.com","password":"password123"}`)
	resp = struct {
		Token       string `json:"token"`
		Requires2FA bool   `json:"requires_2fa"`
		TempToken   string `json:"temp_token"`
	}{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if w.Code != http.StatusOK || !resp.Requires2FA || resp.TempToken == "" {
		t.Fatalf("expected requires_2fa with temp_token: code=%d resp=%+v", w.Code, resp)
	}

	// 5) verify 2FA -> full token
	validCode2, _ := totp.GenerateCode(string(totpSecret), time.Now())
	w = doReq(t, r, http.MethodPost, "/api/auth/2fa/verify", "", `{"temp_token":"`+resp.TempToken+`","code":"`+validCode2+`"}`)
	resp = struct {
		Token       string `json:"token"`
		Requires2FA bool   `json:"requires_2fa"`
		TempToken   string `json:"temp_token"`
	}{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if w.Code != http.StatusOK || resp.Token == "" || resp.Requires2FA {
		t.Fatalf("verify: %d resp=%+v", w.Code, resp)
	}

	// 6) recovery code
	w = doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"2fa@example.com","password":"password123"}`)
	resp = struct {
		Token       string `json:"token"`
		Requires2FA bool   `json:"requires_2fa"`
		TempToken   string `json:"temp_token"`
	}{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	tstep(t, "recovery code (use once, reuse fails)")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/recovery", "", `{"temp_token":"`+resp.TempToken+`","code":"`+setupResp.RecoveryCodes[0]+`"}`),
		http.StatusOK, "recovery code")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/recovery", "", `{"temp_token":"`+resp.TempToken+`","code":"`+setupResp.RecoveryCodes[0]+`"}`),
		http.StatusUnauthorized, "recovery code reuse")

	// 7) disable 2FA (needs a fresh full token from verify)
	w = doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"2fa@example.com","password":"password123"}`)
	resp = struct {
		Token       string `json:"token"`
		Requires2FA bool   `json:"requires_2fa"`
		TempToken   string `json:"temp_token"`
	}{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	verifyBody := `{"temp_token":"` + resp.TempToken + `","code":"` + validCode2 + `"}`
	w = doReq(t, r, http.MethodPost, "/api/auth/2fa/verify", "", verifyBody)
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	fullToken := resp.Token

	// disable with wrong then valid code
	tstep(t, "disable 2FA (wrong then valid)")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/disable", fullToken, `{"code":"000000"}`),
		http.StatusUnauthorized, "disable wrong code")
	validCode3, _ := totp.GenerateCode(string(totpSecret), time.Now())
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/disable", fullToken, `{"code":"`+validCode3+`"}`),
		http.StatusNoContent, "disable valid code")

	// login works without 2FA again
	tstep(t, "login after disable -> direct token")
	w = doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"2fa@example.com","password":"password123"}`)
	resp = struct {
		Token       string `json:"token"`
		Requires2FA bool   `json:"requires_2fa"`
		TempToken   string `json:"temp_token"`
	}{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Requires2FA || resp.Token == "" {
		t.Errorf("after disable: expected direct token, got requires_2fa=%v token=%q", resp.Requires2FA, resp.Token)
	}
}
