package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/core/middleware"
	"github.com/derpixler/skolva/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestUserEndpoints(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	admin, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "admin@example.com", PasswordHash: "h", FirstName: "Ad", LastName: "Min",
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}

	verify := func(token string) (*middleware.Actor, error) {
		switch token {
		case "admin":
			return &middleware.Actor{UserID: admin.ID.String(), Roles: []string{"admin"}}, nil
		case "weak":
			return &middleware.Actor{UserID: admin.ID.String(), Permissions: []string{"users.read"}}, nil
		default:
			return nil, errors.New("invalid token")
		}
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Authenticate(verify))
	r.Use(middleware.ActorMiddleware())
	api := r.Group("/api")
	auth.RegisterRoutes(api, pool, nil, nil)

	// register -> 201
	w := doReq(t, r, http.MethodPost, "/api/auth/register", "admin", `{"email":"searchme@example.com","password":"password123","first_name":"Search","last_name":"Mee"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d (%s)", w.Code, w.Body.String())
	}
	var u auth.UserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
		t.Fatalf("unmarshal register: %v", err)
	}
	if u.Email != "searchme@example.com" {
		t.Fatalf("unexpected user: %+v", u)
	}
	nid := u.ID.String()

	// duplicate email -> 409
	if w := doReq(t, r, http.MethodPost, "/api/auth/register", "admin", `{"email":"searchme@example.com","password":"password123","first_name":"X","last_name":"Y"}`); w.Code != http.StatusConflict {
		t.Errorf("duplicate email: expected 409, got %d", w.Code)
	}
	// short password -> 422
	if w := doReq(t, r, http.MethodPost, "/api/auth/register", "admin", `{"email":"a@b.co","password":"short","first_name":"X","last_name":"Y"}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("short password: expected 422, got %d", w.Code)
	}

	// list -> 200 with at least admin + new user
	w = doReq(t, r, http.MethodGet, "/api/users", "admin", "")
	var list []auth.UserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if w.Code != http.StatusOK || len(list) < 2 {
		t.Errorf("list: code=%d len=%d", w.Code, len(list))
	}

	// get -> 200; unknown -> 404
	if w := doReq(t, r, http.MethodGet, "/api/users/"+nid, "admin", ""); w.Code != http.StatusOK {
		t.Errorf("get: expected 200, got %d", w.Code)
	}
	if w := doReq(t, r, http.MethodGet, "/api/users/"+uuid.NewString(), "admin", ""); w.Code != http.StatusNotFound {
		t.Errorf("get unknown: expected 404, got %d", w.Code)
	}

	// search finds the user (core/search #24)
	w = doReq(t, r, http.MethodGet, "/api/search/users?q=Mee", "admin", "")
	if w.Code != http.StatusOK {
		t.Fatalf("search: expected 200, got %d", w.Code)
	}
	var found []auth.UserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &found); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	hit := false
	for _, x := range found {
		if x.ID.String() == nid {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected search to find the user, got %+v", found)
	}

	// update -> 200
	if w := doReq(t, r, http.MethodPatch, "/api/users/"+nid, "admin", `{"first_name":"Renamed","last_name":"Mee"}`); w.Code != http.StatusOK {
		t.Errorf("update: expected 200, got %d (%s)", w.Code, w.Body.String())
	}

	// delete -> 204; then get -> 404
	if w := doReq(t, r, http.MethodDelete, "/api/users/"+nid, "admin", ""); w.Code != http.StatusNoContent {
		t.Errorf("delete: expected 204, got %d", w.Code)
	}
	if w := doReq(t, r, http.MethodGet, "/api/users/"+nid, "admin", ""); w.Code != http.StatusNotFound {
		t.Errorf("get after delete: expected 404, got %d", w.Code)
	}

	// register without permission -> 403
	if w := doReq(t, r, http.MethodPost, "/api/auth/register", "weak", `{"email":"z@z.zz","password":"password123","first_name":"Z","last_name":"Z"}`); w.Code != http.StatusForbidden {
		t.Errorf("weak register: expected 403, got %d", w.Code)
	}
	// list without auth -> 401
	if w := doReq(t, r, http.MethodGet, "/api/users", "", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("no-auth list: expected 401, got %d", w.Code)
	}

	// delete nonexistent user -> 404
	if w := doReq(t, r, http.MethodDelete, "/api/users/"+uuid.NewString(), "admin", ""); w.Code != http.StatusNotFound {
		t.Errorf("delete unknown user: expected 404, got %d", w.Code)
	}
	// update nonexistent user -> 404
	if w := doReq(t, r, http.MethodPatch, "/api/users/"+uuid.NewString(), "admin", `{"first_name":"X","last_name":"Y"}`); w.Code != http.StatusNotFound {
		t.Errorf("update unknown user: expected 404, got %d", w.Code)
	}
	// update invalid id -> 422
	if w := doReq(t, r, http.MethodPatch, "/api/users/not-a-uuid", "admin", `{"first_name":"X","last_name":"Y"}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("update invalid id: expected 422, got %d", w.Code)
	}

	// pagination: limit query param (covers pagination ParseInt branches)
	if w := doReq(t, r, http.MethodGet, "/api/users?limit=5&offset=0", "admin", ""); w.Code != http.StatusOK {
		t.Errorf("list with limit: expected 200, got %d", w.Code)
	}
}
