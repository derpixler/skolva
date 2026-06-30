package groups_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/derpixler/skolva-core/middleware"
	"github.com/derpixler/skolva/internal/db"
	"github.com/derpixler/skolva/internal/groups"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestGroupMemberEndpoints(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()

	var actorID, memberID uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "actor@example.com", "h").Scan(&actorID); err != nil {
		t.Fatalf("insert actor: %v", err)
	}
	if err := pool.QueryRow(ctx, insertUser, "member@example.com", "h").Scan(&memberID); err != nil {
		t.Fatalf("insert member: %v", err)
	}

	g, err := groups.NewRepository(pool).Create(ctx, actorID, db.CreateGroupParams{
		Name: "Team A", GroupType: "mannschaft", Actor: uuid.NullUUID{UUID: actorID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	verify := func(token string) (*middleware.Actor, error) {
		switch token {
		case "admin":
			return &middleware.Actor{UserID: actorID.String(), Roles: []string{"admin"}}, nil
		case "weak":
			return &middleware.Actor{UserID: actorID.String(), Permissions: []string{"groups.read"}}, nil
		default:
			return nil, errors.New("invalid token")
		}
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Authenticate(verify))
	r.Use(middleware.ActorMiddleware())
	api := r.Group("/api")
	groups.RegisterRoutes(api, pool)

	membersPath := "/api/groups/" + g.ID.String() + "/members"

	// add member -> 200 with member list
	w := doReq(t, r, http.MethodPost, membersPath, "admin", `{"user_id":"`+memberID.String()+`","role_in_group":"trainer"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("add member: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var members []groups.Member
	if err := json.Unmarshal(w.Body.Bytes(), &members); err != nil {
		t.Fatalf("unmarshal members: %v", err)
	}
	if len(members) != 1 || members[0].UserID != memberID || members[0].RoleInGroup != "trainer" {
		t.Fatalf("unexpected members: %+v", members)
	}

	// list members
	if w := doReq(t, r, http.MethodGet, membersPath, "admin", ""); w.Code != http.StatusOK {
		t.Errorf("list members: expected 200, got %d", w.Code)
	}

	// user groups
	w = doReq(t, r, http.MethodGet, "/api/users/"+memberID.String()+"/groups", "admin", "")
	if w.Code != http.StatusOK {
		t.Fatalf("user groups: expected 200, got %d", w.Code)
	}
	var ug []groups.UserGroup
	if err := json.Unmarshal(w.Body.Bytes(), &ug); err != nil {
		t.Fatalf("unmarshal user groups: %v", err)
	}
	if len(ug) != 1 || ug[0].GroupID != g.ID || ug[0].RoleInGroup != "trainer" {
		t.Errorf("unexpected user groups: %+v", ug)
	}

	// upsert role
	w = doReq(t, r, http.MethodPost, membersPath, "admin", `{"user_id":"`+memberID.String()+`","role_in_group":"leiter"}`)
	if err := json.Unmarshal(w.Body.Bytes(), &members); err != nil {
		t.Fatalf("unmarshal upsert: %v", err)
	}
	if len(members) != 1 || members[0].RoleInGroup != "leiter" {
		t.Errorf("expected role leiter after upsert, got %+v", members)
	}

	// remove -> 204, then lists empty
	if w := doReq(t, r, http.MethodDelete, membersPath+"/"+memberID.String(), "admin", ""); w.Code != http.StatusNoContent {
		t.Errorf("remove member: expected 204, got %d", w.Code)
	}
	if w := doReq(t, r, http.MethodGet, membersPath, "admin", ""); w.Code != http.StatusOK {
		t.Errorf("members after remove: expected 200, got %d", w.Code)
	}

	// invalid role -> 422
	if w := doReq(t, r, http.MethodPost, membersPath, "admin", `{"user_id":"`+memberID.String()+`","role_in_group":"boss"}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid role: expected 422, got %d", w.Code)
	}
	// invalid user_id -> 422
	if w := doReq(t, r, http.MethodPost, membersPath, "admin", `{"user_id":"not-a-uuid"}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid user_id: expected 422, got %d", w.Code)
	}
	// unknown group -> 404
	if w := doReq(t, r, http.MethodPost, "/api/groups/"+uuid.NewString()+"/members", "admin", `{"user_id":"`+memberID.String()+`"}`); w.Code != http.StatusNotFound {
		t.Errorf("unknown group: expected 404, got %d", w.Code)
	}
	// no auth -> 401
	if w := doReq(t, r, http.MethodPost, membersPath, "", `{"user_id":"`+memberID.String()+`"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("no auth: expected 401, got %d", w.Code)
	}
	// insufficient permission -> 403
	if w := doReq(t, r, http.MethodPost, membersPath, "weak", `{"user_id":"`+memberID.String()+`"}`); w.Code != http.StatusForbidden {
		t.Errorf("weak: expected 403, got %d", w.Code)
	}

	// remove member from nonexistent group -> 404
	if w := doReq(t, r, http.MethodDelete, "/api/groups/"+uuid.NewString()+"/members/"+memberID.String(), "admin", ""); w.Code != http.StatusNotFound {
		t.Errorf("remove from unknown group: expected 404, got %d", w.Code)
	}
}
