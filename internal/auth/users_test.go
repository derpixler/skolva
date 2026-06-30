package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/derpixler/skolva-core/middleware"
	"github.com/derpixler/skolva/internal/auth"
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
	auth.RegisterRoutes(api, pool, nil, nil, nil)

	// register -> 201
	tstep(t, "register user searchme@example.com as admin")
	w := doReq(t, r, http.MethodPost, "/api/auth/register", "admin", `{"email":"searchme@example.com","password":"password123","first_name":"Search","last_name":"Mee"}`)
	if !assertStatus(t, w, http.StatusCreated, "register") {
		t.FailNow()
	}
	var u auth.UserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
		t.Fatalf("unmarshal register: %v", err)
	}
	if u.Email != "searchme@example.com" {
		t.Fatalf("unexpected user: %+v", u)
	}
	nid := u.ID.String()
	tlog(t, "[val ] created user id=%s email=%s", nid, u.Email)

	tstep(t, "register negative cases")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/register", "admin", `{"email":"searchme@example.com","password":"password123","first_name":"X","last_name":"Y"}`),
		http.StatusConflict, "duplicate email")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/register", "admin", `{"email":"a@b.co","password":"short","first_name":"X","last_name":"Y"}`),
		http.StatusUnprocessableEntity, "short password")

	// list -> 200 with at least admin + new user
	tstep(t, "list users")
	w = doReq(t, r, http.MethodGet, "/api/users", "admin", "")
	var list []auth.UserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	assertStatus(t, w, http.StatusOK, "list users")
	if len(list) < 2 {
		t.Errorf("expected >=2 users, got %d", len(list))
	}
	tlog(t, "[val ] list returned %d users", len(list))

	// get -> 200; unknown -> 404
	tstep(t, "get user (existing + unknown)")
	assertStatus(t, doReq(t, r, http.MethodGet, "/api/users/"+nid, "admin", ""), http.StatusOK, "get user")
	assertStatus(t, doReq(t, r, http.MethodGet, "/api/users/"+uuid.NewString(), "admin", ""), http.StatusNotFound, "get unknown user")

	// search finds the user (core/search #24)
	tstep(t, "search users q=Mee")
	w = doReq(t, r, http.MethodGet, "/api/search/users?q=Mee", "admin", "")
	if !assertStatus(t, w, http.StatusOK, "search users") {
		t.FailNow()
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
	tlog(t, "[val ] search hits=%d, found target=%v", len(found), hit)

	// update -> 200
	tstep(t, "update user (rename)")
	assertStatus(t, doReq(t, r, http.MethodPatch, "/api/users/"+nid, "admin", `{"first_name":"Renamed","last_name":"Mee"}`),
		http.StatusOK, "update user")

	// delete -> 204; then get -> 404
	tstep(t, "soft-delete user, then re-get")
	assertStatus(t, doReq(t, r, http.MethodDelete, "/api/users/"+nid, "admin", ""), http.StatusNoContent, "delete user")
	assertStatus(t, doReq(t, r, http.MethodGet, "/api/users/"+nid, "admin", ""), http.StatusNotFound, "get after delete")

	// permission / validation edge cases (token in [req] line shows the role)
	tstep(t, "users edge cases (weak / no token / unknown / invalid)")
	assertStatus(t, doReq(t, r, http.MethodPost, "/api/auth/register", "weak", `{"email":"z@z.zz","password":"password123","first_name":"Z","last_name":"Z"}`),
		http.StatusForbidden, "weak token register")
	assertStatus(t, doReq(t, r, http.MethodGet, "/api/users", "", ""),
		http.StatusUnauthorized, "no-auth list")
	assertStatus(t, doReq(t, r, http.MethodDelete, "/api/users/"+uuid.NewString(), "admin", ""),
		http.StatusNotFound, "delete unknown user")
	assertStatus(t, doReq(t, r, http.MethodPatch, "/api/users/"+uuid.NewString(), "admin", `{"first_name":"X","last_name":"Y"}`),
		http.StatusNotFound, "update unknown user")
	assertStatus(t, doReq(t, r, http.MethodPatch, "/api/users/not-a-uuid", "admin", `{"first_name":"X","last_name":"Y"}`),
		http.StatusUnprocessableEntity, "update invalid id")
	assertStatus(t, doReq(t, r, http.MethodGet, "/api/users?limit=5&offset=0", "admin", ""),
		http.StatusOK, "list with limit")
}
