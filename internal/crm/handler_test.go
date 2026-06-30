package crm_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/derpixler/skolva-core/middleware"
	"github.com/derpixler/skolva/internal/crm"
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

func TestAddressAndPreferencesEndpoints(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()

	var uid, other uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "owner@example.com", "h").Scan(&uid); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := pool.QueryRow(ctx, insertUser, "other@example.com", "h").Scan(&other); err != nil {
		t.Fatalf("insert other: %v", err)
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

	addrPath := "/api/users/" + uid.String() + "/address"
	prefPath := "/api/users/" + uid.String() + "/preferences"

	// PUT address (lowercase country code is upper-cased) -> 200
	w := doReq(t, r, http.MethodPut, addrPath, "admin", `{"street1":"Main 1","postal_code":"12345","city":"Berlin","country_code":"de"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("put address: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var addr crm.Address
	if err := json.Unmarshal(w.Body.Bytes(), &addr); err != nil {
		t.Fatalf("unmarshal address: %v", err)
	}
	if addr.Street1 != "Main 1" || addr.CountryCode != "DE" {
		t.Errorf("unexpected address: %+v", addr)
	}

	// GET address -> 200
	if w := doReq(t, r, http.MethodGet, addrPath, "admin", ""); w.Code != http.StatusOK {
		t.Errorf("get address: expected 200, got %d", w.Code)
	}
	// GET address for a user without one -> 404
	if w := doReq(t, r, http.MethodGet, "/api/users/"+other.String()+"/address", "admin", ""); w.Code != http.StatusNotFound {
		t.Errorf("get missing address: expected 404, got %d", w.Code)
	}
	// invalid country_code -> 422
	if w := doReq(t, r, http.MethodPut, addrPath, "admin", `{"street1":"X","postal_code":"1","city":"Y","country_code":"DEU"}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid country_code: expected 422, got %d", w.Code)
	}
	// missing street1 -> 422
	if w := doReq(t, r, http.MethodPut, addrPath, "admin", `{"postal_code":"1","city":"Y","country_code":"DE"}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing street1: expected 422, got %d", w.Code)
	}

	// PUT preferences -> 200
	w = doReq(t, r, http.MethodPut, prefPath, "admin", `{"preferred_contact_type":"email"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("put preferences: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var prefs crm.Preferences
	if err := json.Unmarshal(w.Body.Bytes(), &prefs); err != nil {
		t.Fatalf("unmarshal preferences: %v", err)
	}
	if prefs.PreferredContactType == nil || *prefs.PreferredContactType != "email" {
		t.Errorf("unexpected preferences: %+v", prefs)
	}
	// GET preferences -> 200
	if w := doReq(t, r, http.MethodGet, prefPath, "admin", ""); w.Code != http.StatusOK {
		t.Errorf("get preferences: expected 200, got %d", w.Code)
	}
	// invalid preferred_contact_type -> 422
	if w := doReq(t, r, http.MethodPut, prefPath, "admin", `{"preferred_contact_type":"fax"}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid preferred_contact_type: expected 422, got %d", w.Code)
	}
	// get preferences for user without any -> 404
	if w := doReq(t, r, http.MethodGet, "/api/users/"+other.String()+"/preferences", "admin", ""); w.Code != http.StatusNotFound {
		t.Errorf("get missing preferences: expected 404, got %d", w.Code)
	}

	// no auth -> 401
	if w := doReq(t, r, http.MethodPut, addrPath, "", `{"street1":"X","postal_code":"1","city":"Y","country_code":"DE"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("no auth: expected 401, got %d", w.Code)
	}
	// insufficient permission -> 403
	if w := doReq(t, r, http.MethodPut, addrPath, "weak", `{"street1":"X","postal_code":"1","city":"Y","country_code":"DE"}`); w.Code != http.StatusForbidden {
		t.Errorf("weak: expected 403, got %d", w.Code)
	}
}
