package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
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
	auth.RegisterRoutes(api, pool, nil)

	base := "/api/users/" + target.ID.String() + "/roles"

	// assign -> 200 with the updated role list
	w := doReq(t, r, http.MethodPost, base, "admin", `{"role_slug":"mitglied"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("assign: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var roles []auth.RoleResponse
	if err := json.Unmarshal(w.Body.Bytes(), &roles); err != nil {
		t.Fatalf("unmarshal assign response: %v", err)
	}
	if len(roles) != 1 || roles[0].Slug != "mitglied" {
		t.Errorf("expected [mitglied], got %+v", roles)
	}

	// list -> 200
	w = doReq(t, r, http.MethodGet, base, "admin", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	// remove -> 204
	w = doReq(t, r, http.MethodDelete, base+"/mitglied", "admin", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("remove: expected 204, got %d", w.Code)
	}

	// list again -> empty
	w = doReq(t, r, http.MethodGet, base, "admin", "")
	roles = nil
	if err := json.Unmarshal(w.Body.Bytes(), &roles); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("expected no roles, got %+v", roles)
	}

	// unknown role -> 404
	if w := doReq(t, r, http.MethodPost, base, "admin", `{"role_slug":"nope"}`); w.Code != http.StatusNotFound {
		t.Errorf("unknown role: expected 404, got %d", w.Code)
	}

	// unknown user -> 404
	if w := doReq(t, r, http.MethodPost, "/api/users/"+uuid.NewString()+"/roles", "admin", `{"role_slug":"mitglied"}`); w.Code != http.StatusNotFound {
		t.Errorf("unknown user: expected 404, got %d", w.Code)
	}

	// missing role_slug -> 422
	if w := doReq(t, r, http.MethodPost, base, "admin", `{}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing role_slug: expected 422, got %d", w.Code)
	}

	// invalid user id -> 422
	if w := doReq(t, r, http.MethodPost, "/api/users/not-a-uuid/roles", "admin", `{"role_slug":"mitglied"}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid user id: expected 422, got %d", w.Code)
	}

	// no auth -> 401
	if w := doReq(t, r, http.MethodPost, base, "", `{"role_slug":"mitglied"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("no auth: expected 401, got %d", w.Code)
	}

	// insufficient permission -> 403
	if w := doReq(t, r, http.MethodPost, base, "weak", `{"role_slug":"mitglied"}`); w.Code != http.StatusForbidden {
		t.Errorf("weak token: expected 403, got %d", w.Code)
	}
}
