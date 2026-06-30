package groups_test

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
	"github.com/derpixler/skolva/internal/groups"
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

func TestGroupEndpoints(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()

	var actorID uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "actor@example.com", "h").Scan(&actorID); err != nil {
		t.Fatalf("insert actor: %v", err)
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

	// create -> 201
	w := doReq(t, r, http.MethodPost, "/api/groups", "admin", `{"name":"Team X","group_type":"mannschaft"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d (%s)", w.Code, w.Body.String())
	}
	var g groups.Group
	if err := json.Unmarshal(w.Body.Bytes(), &g); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	if g.Name != "Team X" || g.GroupType != "mannschaft" || g.ID == uuid.Nil {
		t.Fatalf("unexpected created group: %+v", g)
	}
	gid := g.ID.String()

	// list -> contains it
	w = doReq(t, r, http.MethodGet, "/api/groups", "admin", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}
	var list []groups.Group
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 group, got %d", len(list))
	}

	// filter by type
	if w := doReq(t, r, http.MethodGet, "/api/groups?group_type=mannschaft", "admin", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Team X") {
		t.Errorf("filter mannschaft: code=%d body=%s", w.Code, w.Body.String())
	}
	w = doReq(t, r, http.MethodGet, "/api/groups?group_type=abteilung", "admin", "")
	list = nil
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal filter: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 abteilung groups, got %d", len(list))
	}

	// get -> 200
	if w := doReq(t, r, http.MethodGet, "/api/groups/"+gid, "admin", ""); w.Code != http.StatusOK {
		t.Errorf("get: expected 200, got %d", w.Code)
	}

	// update -> 200, type changed
	w = doReq(t, r, http.MethodPatch, "/api/groups/"+gid, "admin", `{"name":"Team Y","group_type":"abteilung"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &g); err != nil {
		t.Fatalf("unmarshal update: %v", err)
	}
	if g.Name != "Team Y" || g.GroupType != "abteilung" {
		t.Errorf("unexpected updated group: %+v", g)
	}

	// delete -> 204, then get -> 404
	if w := doReq(t, r, http.MethodDelete, "/api/groups/"+gid, "admin", ""); w.Code != http.StatusNoContent {
		t.Errorf("delete: expected 204, got %d", w.Code)
	}
	if w := doReq(t, r, http.MethodGet, "/api/groups/"+gid, "admin", ""); w.Code != http.StatusNotFound {
		t.Errorf("get after delete: expected 404, got %d", w.Code)
	}

	// invalid group_type -> 422
	if w := doReq(t, r, http.MethodPost, "/api/groups", "admin", `{"name":"Z","group_type":"nope"}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid type: expected 422, got %d", w.Code)
	}
	// missing name -> 422
	if w := doReq(t, r, http.MethodPost, "/api/groups", "admin", `{"group_type":"sonstige"}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing name: expected 422, got %d", w.Code)
	}
	// invalid id -> 422
	if w := doReq(t, r, http.MethodGet, "/api/groups/not-a-uuid", "admin", ""); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid id: expected 422, got %d", w.Code)
	}
	// no auth -> 401
	if w := doReq(t, r, http.MethodPost, "/api/groups", "", `{"name":"A","group_type":"sonstige"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("no auth: expected 401, got %d", w.Code)
	}
	// insufficient permission -> 403
	if w := doReq(t, r, http.MethodPost, "/api/groups", "weak", `{"name":"A","group_type":"sonstige"}`); w.Code != http.StatusForbidden {
		t.Errorf("weak: expected 403, got %d", w.Code)
	}

	// delete nonexistent group -> 404
	if w := doReq(t, r, http.MethodDelete, "/api/groups/"+uuid.NewString(), "admin", ""); w.Code != http.StatusNotFound {
		t.Errorf("delete unknown group: expected 404, got %d", w.Code)
	}
	// list with pagination query params
	if w := doReq(t, r, http.MethodGet, "/api/groups?limit=5&offset=0", "admin", ""); w.Code != http.StatusOK {
		t.Errorf("list with limit: expected 200, got %d", w.Code)
	}
	// list with invalid limit -> falls back to default (200)
	if w := doReq(t, r, http.MethodGet, "/api/groups?limit=abc", "admin", ""); w.Code != http.StatusOK {
		t.Errorf("list with invalid limit: expected 200, got %d", w.Code)
	}
}
