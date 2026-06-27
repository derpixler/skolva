package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/core/middleware"
	"github.com/derpixler/skolva/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func doReq(t *testing.T, r http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, path, rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if testing.Verbose() {
		authInfo := "no"
		if token != "" {
			authInfo = "yes(" + shortTok(token) + ")"
		}
		t.Logf("[req ] %-6s %s  auth=%s  body=%s", method, path, authInfo, clip(body))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if testing.Verbose() {
		t.Logf("[resp] %d  body=%s", w.Code, clip(w.Body.String()))
	}
	return w
}

// =============================================================================
// Detailed test logging helpers (all gated behind `go test -v`).
//
// They surface, per scenario: what is exercised, with which input values, and
// how the result looks. Output lands in the harness log file for the curated
// steps (which pass -v); plain `go test ./...` (steps 06/10) stays quiet.
// =============================================================================

const tlogMax = 200 // truncate long bodies in logs

var (
	jwtRe = regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+(?:\.[A-Za-z0-9_\-]+){1,2}`) // full access / pending JWTs (header.payload[.signature])
	pwRe  = regexp.MustCompile(`("password"\s*:\s*")[^"]*(")`)                 // password field
)

// redactJSON hides secrets/noise: passwords -> ***, JWTs -> eyJ…. OTP codes,
// reset tokens (UUIDs) and recovery codes stay visible (they are test data the
// reader wants to see).
func redactJSON(s string) string {
	s = pwRe.ReplaceAllString(s, `${1}***${2}`)
	s = jwtRe.ReplaceAllString(s, `eyJ…`)
	return s
}

// clip redacts then rune-safely truncates a body for logging; "" -> "-".
func clip(s string) string {
	s = redactJSON(s)
	if s == "" {
		return "-"
	}
	r := []rune(s)
	if len(r) > tlogMax {
		return string(r[:tlogMax]) + "…"
	}
	return s
}

// shortTok keeps meaningful stub tokens (e.g. "admin"/"weak") readable but
// collapses real JWTs.
func shortTok(tok string) string {
	if strings.HasPrefix(tok, "eyJ") && len(tok) > 8 {
		return "eyJ…"
	}
	return tok
}

// tlog prints a gated, caller-attributed detail line.
func tlog(t *testing.T, format string, args ...any) {
	t.Helper()
	if testing.Verbose() {
		t.Logf(format, args...)
	}
}

// tstep marks a logical scenario phase.
func tstep(t *testing.T, name string) {
	t.Helper()
	tlog(t, "[step] %s", name)
}

// assertStatus checks an HTTP status, logging want/got on success (verbose)
// and always failing with the response body on mismatch. Returns whether it
// matched, so callers can stop a dependent sequence via `if !assertStatus(...)`.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int, desc string) bool {
	t.Helper()
	if w.Code == want {
		tlog(t, "[chk ] %s: want=%d got=%d  OK", desc, want, w.Code)
		return true
	}
	t.Errorf("[chk ] %s: want=%d got=%d  FAIL  body=%s", desc, want, w.Code, w.Body.String())
	return false
}

func TestRoleAssignmentEndpoints(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	adminUser, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "admin@example.com", PasswordHash: "h", FirstName: "Ad", LastName: "Min",
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	target, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "target@example.com", PasswordHash: "h", FirstName: "Tar", LastName: "Get",
	})
	if err != nil {
		t.Fatalf("create target: %v", err)
	}

	// stub verifier: admin (all perms), weak (users.read only), else invalid.
	verify := func(token string) (*middleware.Actor, error) {
		switch token {
		case "admin":
			return &middleware.Actor{UserID: adminUser.ID.String(), Roles: []string{"admin"}}, nil
		case "weak":
			return &middleware.Actor{UserID: adminUser.ID.String(), Permissions: []string{"users.read"}}, nil
		default:
			return nil, errors.New("invalid token")
		}
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Authenticate(verify))
	r.Use(middleware.ActorMiddleware())
	api := r.Group("/api")
	auth.RegisterRoutes(api, pool, nil, nil, nil)

	base := "/api/users/" + target.ID.String() + "/roles"

	// assign -> 200 with the updated role list
	tstep(t, "assign role 'mitglied' as admin")
	w := doReq(t, r, http.MethodPost, base, "admin", `{"role_slug":"mitglied"}`)
	if !assertStatus(t, w, http.StatusOK, "assign role") {
		t.FailNow()
	}
	var roles []auth.RoleResponse
	if err := json.Unmarshal(w.Body.Bytes(), &roles); err != nil {
		t.Fatalf("unmarshal assign response: %v", err)
	}
	if len(roles) != 1 || roles[0].Slug != "mitglied" {
		t.Errorf("expected [mitglied], got %+v", roles)
	}
	tlog(t, "[val ] resulting roles=%+v", roles)

	// list -> 200
	tstep(t, "list roles")
	assertStatus(t, doReq(t, r, http.MethodGet, base, "admin", ""), http.StatusOK, "list roles")

	// remove -> 204
	tstep(t, "remove role 'mitglied'")
	assertStatus(t, doReq(t, r, http.MethodDelete, base+"/mitglied", "admin", ""), http.StatusNoContent, "remove role")

	// list again -> empty
	tstep(t, "list roles after removal")
	w = doReq(t, r, http.MethodGet, base, "admin", "")
	roles = nil
	if err := json.Unmarshal(w.Body.Bytes(), &roles); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("expected no roles, got %+v", roles)
	}
	tlog(t, "[val ] roles after removal=%+v", roles)

	// --- permission / validation edge cases (token in [req] line shows the role) ---
	tstep(t, "RBAC edge cases (admin / weak / no token)")
	assertStatus(t, doReq(t, r, http.MethodPost, base, "admin", `{"role_slug":"nope"}`),
		http.StatusNotFound, "unknown role")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/users/"+uuid.NewString()+"/roles", "admin", `{"role_slug":"mitglied"}`),
		http.StatusNotFound, "unknown user")
	assertStatus(t, doReq(t, r, http.MethodPost, base, "admin", `{}`),
		http.StatusUnprocessableEntity, "missing role_slug")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/users/not-a-uuid/roles", "admin", `{"role_slug":"mitglied"}`),
		http.StatusUnprocessableEntity, "invalid user id")
	assertStatus(t, doReq(t, r, http.MethodPost, base, "", `{"role_slug":"mitglied"}`),
		http.StatusUnauthorized, "no auth")
	assertStatus(t, doReq(t, r, http.MethodPost, base, "weak", `{"role_slug":"mitglied"}`),
		http.StatusForbidden, "weak token (insufficient permission)")
	assertStatus(t, doReq(t, r, http.MethodDelete, "/api/users/"+uuid.NewString()+"/roles/mitglied", "admin", ""),
		http.StatusNotFound, "delete role unknown user")
}
