package crm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/derpixler/skolva/internal/core/middleware"
	"github.com/derpixler/skolva/internal/crm"
	"github.com/derpixler/skolva/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestContactsRepository(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := crm.NewRepository(pool)

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "c@example.com", "h").Scan(&uid); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	actor := uuid.NullUUID{UUID: uid, Valid: true}

	mk := func(value string, primary, clear bool) db.CreateContactRow {
		t.Helper()
		row, err := repo.CreateContact(ctx, uid, clear, db.CreateContactParams{
			UserID: uid, ContactType: "email", Value: value, IsPrimary: primary, AllowContact: true, Actor: actor,
		})
		if err != nil {
			t.Fatalf("create contact %s: %v", value, err)
		}
		return row
	}

	c1 := mk("a@x.com", true, true) // primary
	c2 := mk("b@x.com", true, true) // primary -> clears c1
	if !c2.IsPrimary {
		t.Fatal("c2 should be primary")
	}

	// exactly one primary per type
	list, err := repo.ListContacts(ctx, uid)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	primaries := 0
	for _, c := range list {
		if c.IsPrimary {
			primaries++
		}
	}
	if len(list) != 2 || primaries != 1 {
		t.Fatalf("expected 2 contacts with exactly 1 primary, got %d contacts / %d primaries", len(list), primaries)
	}

	// a 2nd primary without clearing violates the unique constraint
	if _, err := repo.CreateContact(ctx, uid, false, db.CreateContactParams{
		UserID: uid, ContactType: "email", Value: "x@x.com", IsPrimary: true, AllowContact: true, Actor: actor,
	}); err == nil {
		t.Error("expected unique-constraint error for a 2nd primary without clearing")
	}

	// audit attribution on c1 insert
	var auditActor uuid.UUID
	if err := pool.QueryRow(ctx,
		"SELECT actor_user_id FROM audit_logs WHERE table_name='user_contact_points' AND record_pk=$1 AND action='INSERT'",
		c1.ID.String()).Scan(&auditActor); err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if auditActor != uid {
		t.Errorf("expected audit actor %s, got %s", uid, auditActor)
	}

	// SetPrimary: make c1 primary again -> clears c2
	if _, err := repo.UpdateContact(ctx, uid, uid, "email", true, db.UpdateContactParams{
		ID: c1.ID, Value: "a@x.com", IsPrimary: true, AllowContact: true, UpdatedBy: actor,
	}); err != nil {
		t.Fatalf("update c1 primary: %v", err)
	}
	list, _ = repo.ListContacts(ctx, uid)
	for _, c := range list {
		if c.ID == c2.ID && c.IsPrimary {
			t.Error("c2 should no longer be primary")
		}
		if c.ID == c1.ID && !c.IsPrimary {
			t.Error("c1 should be primary")
		}
	}

	// soft delete c2
	if err := repo.SoftDeleteContact(ctx, uid, db.SoftDeleteContactParams{ID: c2.ID, UpdatedBy: actor}); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	list, _ = repo.ListContacts(ctx, uid)
	if len(list) != 1 || list[0].ID != c1.ID {
		t.Errorf("expected only c1 after delete, got %+v", list)
	}
}

func TestContactEndpoints(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "owner@example.com", "h").Scan(&uid); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	verify := func(token string) (*middleware.Actor, error) {
		switch token {
		case "admin":
			return &middleware.Actor{UserID: uid.String(), Roles: []string{"admin"}}, nil
		case "weak":
			return &middleware.Actor{UserID: uid.String(), Permissions: []string{"users.read"}}, nil
		default:
			return nil, errors.New("invalid token")
		}
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Authenticate(verify))
	r.Use(middleware.ActorMiddleware())
	api := r.Group("/api")
	crm.RegisterRoutes(api, pool)

	base := "/api/users/" + uid.String() + "/contacts"

	// create -> 201
	w := doReq(t, r, http.MethodPost, base, "admin", `{"contact_type":"email","value":"a@x.com","is_primary":true}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create contact: expected 201, got %d (%s)", w.Code, w.Body.String())
	}
	var c crm.Contact
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatalf("unmarshal contact: %v", err)
	}
	if c.Value != "a@x.com" || !c.IsPrimary {
		t.Fatalf("unexpected contact: %+v", c)
	}
	cid := c.ID.String()

	// list -> 200, 1 contact
	w = doReq(t, r, http.MethodGet, base, "admin", "")
	var list []crm.Contact
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if w.Code != http.StatusOK || len(list) != 1 {
		t.Errorf("list: code=%d len=%d", w.Code, len(list))
	}

	// update -> 200
	if w := doReq(t, r, http.MethodPatch, base+"/"+cid, "admin", `{"value":"b@x.com","is_primary":true}`); w.Code != http.StatusOK {
		t.Errorf("update: expected 200, got %d (%s)", w.Code, w.Body.String())
	}

	// invalid contact_type -> 422
	if w := doReq(t, r, http.MethodPost, base, "admin", `{"contact_type":"pigeon","value":"x"}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid type: expected 422, got %d", w.Code)
	}
	// update unknown contact -> 404
	if w := doReq(t, r, http.MethodPatch, base+"/"+uuid.NewString(), "admin", `{"value":"x"}`); w.Code != http.StatusNotFound {
		t.Errorf("unknown contact: expected 404, got %d", w.Code)
	}
	// delete -> 204
	if w := doReq(t, r, http.MethodDelete, base+"/"+cid, "admin", ""); w.Code != http.StatusNoContent {
		t.Errorf("delete: expected 204, got %d", w.Code)
	}
	// no auth -> 401
	if w := doReq(t, r, http.MethodPost, base, "", `{"contact_type":"email","value":"x"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("no auth: expected 401, got %d", w.Code)
	}
	// insufficient permission -> 403
	if w := doReq(t, r, http.MethodPost, base, "weak", `{"contact_type":"email","value":"x"}`); w.Code != http.StatusForbidden {
		t.Errorf("weak: expected 403, got %d", w.Code)
	}
}
