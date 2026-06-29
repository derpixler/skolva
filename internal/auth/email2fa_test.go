package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/core/mail"
	"github.com/derpixler/skolva/internal/core/middleware"
	"github.com/derpixler/skolva/internal/core/secrets"
	"github.com/derpixler/skolva/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var otpRe = regexp.MustCompile(`\d{6}`)

// lastOTP extracts the 6-digit code from the most recently "sent" email.
func lastOTP(t *testing.T, m *mail.NoopMailer) string {
	t.Helper()
	sent := m.Sent()
	if len(sent) == 0 {
		t.Fatal("expected an email to be sent, got none")
	}
	last := sent[len(sent)-1]
	code := otpRe.FindString(last.Body)
	if code == "" {
		t.Fatalf("no 6-digit OTP found in email body: %q", last.Body)
	}
	tlog(t, "[mail] to=%v subject=%q code=%s", last.To, last.Subject, code)
	return code
}

func newEmail2FARouter(t *testing.T) (*gin.Engine, *mail.NoopMailer, *pgxpool.Pool) {
	t.Helper()
	pool, cleanup := newSchemaPool(t)
	t.Cleanup(cleanup)

	tm, err := auth.NewTokenManager("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	cipher, err := secrets.NewCipher("test-encryption-key")
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	mailer := mail.NewNoopMailer()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Authenticate(auth.NewVerifier(tm)))
	r.Use(middleware.ActorMiddleware())
	api := r.Group("/api")
	auth.RegisterRoutes(api, pool, tm, cipher, mailer)
	return r, mailer, pool
}

type loginResp struct {
	Token       string `json:"token"`
	Requires2FA bool   `json:"requires_2fa"`
	TempToken   string `json:"temp_token"`
}

func decodeLogin(t *testing.T, b []byte) loginResp {
	t.Helper()
	var r loginResp
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatalf("decode login response: %v (%s)", err, string(b))
	}
	return r
}

func TestEmail2FAFlow(t *testing.T) {
	r, mailer, pool := newEmail2FARouter(t)
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	hash, _ := auth.HashPassword("password123")
	if _, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "email2fa@example.com", PasswordHash: hash, FirstName: "Mail", LastName: "Otp",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 1) login (no 2FA yet) -> full token
	tstep(t, "login (no 2FA yet) -> full token")
	res := doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"email2fa@example.com","password":"password123"}`)
	if !assertStatus(t, res, http.StatusOK, "login (pre-2FA)") {
		t.FailNow()
	}
	lr := decodeLogin(t, res.Body.Bytes())
	if lr.Token == "" || lr.Requires2FA {
		t.Fatalf("expected direct token, got %+v", lr)
	}
	bearer := lr.Token

	// 2) setup email-2FA -> 204, code emailed
	tstep(t, "enable email-2FA (setup -> code emailed)")
	res = doReq(t, r, http.MethodPost, "/api/auth/2fa/email/setup", bearer, "")
	if !assertStatus(t, res, http.StatusNoContent, "email setup") {
		t.FailNow()
	}
	setupCode := lastOTP(t, mailer)

	// 3) confirm with wrong code -> 401; with correct code -> 204
	tstep(t, "confirm email-2FA (wrong then valid)")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/email/confirm", bearer, `{"code":"000000"}`),
		http.StatusUnauthorized, "confirm wrong code")
	if !assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/email/confirm", bearer, `{"code":"`+setupCode+`"}`),
		http.StatusNoContent, "confirm valid code") {
		t.FailNow()
	}

	// 4) login now requires 2FA + sends a login OTP
	tstep(t, "login now requires 2FA + emails a login OTP")
	res = doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"email2fa@example.com","password":"password123"}`)
	lr = decodeLogin(t, res.Body.Bytes())
	if !assertStatus(t, res, http.StatusOK, "login (2FA challenge)") {
		t.FailNow()
	}
	if !lr.Requires2FA || lr.TempToken == "" {
		t.Fatalf("expected requires_2fa with temp_token: %+v", lr)
	}
	tlog(t, "[val ] requires_2fa=%v temp_token=%s", lr.Requires2FA, shortTok(lr.TempToken))
	loginCode := lastOTP(t, mailer)

	// 5) verify wrong -> 401; correct -> 200 + token
	tstep(t, "verify login OTP (wrong then valid)")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/email/verify", "", `{"temp_token":"`+lr.TempToken+`","code":"000000"}`),
		http.StatusUnauthorized, "verify wrong code")
	res = doReq(t, r, http.MethodPost, "/api/auth/2fa/email/verify", "", `{"temp_token":"`+lr.TempToken+`","code":"`+loginCode+`"}`)
	vr := decodeLogin(t, res.Body.Bytes())
	if !assertStatus(t, res, http.StatusOK, "verify valid code") {
		t.FailNow()
	}
	if vr.Token == "" || vr.Requires2FA {
		t.Fatalf("verify: unexpected resp=%+v", vr)
	}
	tlog(t, "[val ] access token issued (%d chars)", len(vr.Token))

	// 6) resend a fresh login OTP and verify it
	tstep(t, "resend login OTP, then verify it")
	res = doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"email2fa@example.com","password":"password123"}`)
	lr = decodeLogin(t, res.Body.Bytes())
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/email/resend", "", `{"temp_token":"`+lr.TempToken+`"}`),
		http.StatusNoContent, "resend login OTP")
	resendCode := lastOTP(t, mailer)
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/email/verify", "", `{"temp_token":"`+lr.TempToken+`","code":"`+resendCode+`"}`),
		http.StatusOK, "verify after resend")

	// 7) disable email-2FA -> 204
	tstep(t, "disable email-2FA")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/email/disable", bearer, ""),
		http.StatusNoContent, "disable email-2FA")

	// 8) login works without 2FA again
	tstep(t, "login after disable -> direct token")
	res = doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"email2fa@example.com","password":"password123"}`)
	lr = decodeLogin(t, res.Body.Bytes())
	if lr.Requires2FA || lr.Token == "" {
		t.Errorf("after disable: expected direct token, got %+v", lr)
	}
}

func TestEmail2FALockout(t *testing.T) {
	r, mailer, pool := newEmail2FARouter(t)
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	hash, _ := auth.HashPassword("password123")
	if _, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "lockout@example.com", PasswordHash: hash, FirstName: "Lock", LastName: "Out",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// enable email-2FA
	tstep(t, "enable email-2FA (login -> setup -> confirm)")
	res := doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"lockout@example.com","password":"password123"}`)
	bearer := decodeLogin(t, res.Body.Bytes()).Token
	doReq(t, r, http.MethodPost, "/api/auth/2fa/email/setup", bearer, "")
	doReq(t, r, http.MethodPost, "/api/auth/2fa/email/confirm", bearer, `{"code":"`+lastOTP(t, mailer)+`"}`)

	// login -> temp_token
	tstep(t, "login -> temp_token")
	res = doReq(t, r, http.MethodPost, "/api/auth/login", "", `{"email":"lockout@example.com","password":"password123"}`)
	temp := decodeLogin(t, res.Body.Bytes()).TempToken

	// 5 wrong attempts -> 401, then locked -> 403
	tstep(t, "5 wrong codes -> 401 each, 6th -> 403 locked")
	for i := 0; i < 5; i++ {
		if !assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/email/verify", "", `{"temp_token":"`+temp+`","code":"000000"}`),
			http.StatusUnauthorized, "wrong code attempt") {
			t.FailNow()
		}
	}
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/2fa/email/verify", "", `{"temp_token":"`+temp+`","code":"000000"}`),
		http.StatusForbidden, "locked after 5 failures")
}
