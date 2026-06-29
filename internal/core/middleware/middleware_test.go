package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/derpixler/skolva/internal/core/middleware"
	"github.com/gin-gonic/gin"
)

func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.CORS())
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected Access-Control-Allow-Origin: *")
	}
}

func TestCORSPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.CORS())
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/test", func(c *gin.Context) {
		id, _ := c.Get("request_id")
		c.String(200, id.(string))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID header")
	}
}

func TestRequestIDWithHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/test", func(c *gin.Context) {
		id, _ := c.Get("request_id")
		c.String(200, id.(string))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "my-custom-id")
	router.ServeHTTP(w, req)

	if w.Body.String() != "my-custom-id" {
		t.Errorf("expected my-custom-id, got %s", w.Body.String())
	}
}

func TestAuthenticateNoHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	router := gin.New()
	router.Use(middleware.Authenticate(func(string) (*middleware.Actor, error) {
		called = true
		return nil, nil
	}))
	router.GET("/test", func(c *gin.Context) {
		if middleware.GetActor(c) == nil {
			c.String(200, "no-actor")
			return
		}
		c.String(200, "actor")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 || w.Body.String() != "no-actor" {
		t.Errorf("expected 200 no-actor, got %d %q", w.Code, w.Body.String())
	}
	if called {
		t.Error("verifier must not be called without a Bearer token")
	}
}

func TestAuthenticateValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.Authenticate(func(token string) (*middleware.Actor, error) {
		if token != "good" {
			return nil, errors.New("bad token")
		}
		return &middleware.Actor{UserID: "u1", Email: "a@b.c", Roles: []string{"mitglied"}}, nil
	}))
	router.GET("/test", func(c *gin.Context) {
		c.String(200, middleware.GetActor(c).UserID)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer good")
	router.ServeHTTP(w, req)

	if w.Code != 200 || w.Body.String() != "u1" {
		t.Errorf("expected 200 u1, got %d %q", w.Code, w.Body.String())
	}
}

func TestAuthenticateInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.Authenticate(func(string) (*middleware.Actor, error) {
		return nil, errors.New("invalid")
	}))
	router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer bad")
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuthNoActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", middleware.RequireAuth(), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuthWithActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		middleware.SetActor(c, &middleware.Actor{UserID: "u1", Roles: []string{"mitglied"}})
	})
	router.GET("/test", middleware.RequireAuth(), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequirePermissionNoActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", middleware.RequirePermission("users.read"), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequirePermissionDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		middleware.SetActor(c, &middleware.Actor{
			UserID:      "u1",
			Roles:       []string{"mitglied"},
			Permissions: []string{"units.read"},
		})
	})
	router.GET("/test", middleware.RequirePermission("users.write"), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequirePermissionGrantedByPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		middleware.SetActor(c, &middleware.Actor{
			UserID:      "u1",
			Roles:       []string{"kassierer"},
			Permissions: []string{"accounting.read"},
		})
	})
	router.GET("/test", middleware.RequirePermission("accounting.read"), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequirePermissionAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		middleware.SetActor(c, &middleware.Actor{UserID: "admin", Roles: []string{"admin"}})
	})
	router.GET("/test", middleware.RequirePermission("admin.jobs"), func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestActorMiddlewarePropagatesToContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		middleware.SetActor(c, &middleware.Actor{UserID: "u1"})
	})
	router.Use(middleware.ActorMiddleware())
	router.GET("/test", func(c *gin.Context) {
		a := middleware.ActorFromContext(c.Request.Context())
		if a == nil {
			c.String(500, "no-context-actor")
			return
		}
		c.String(200, a.UserID)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 || w.Body.String() != "u1" {
		t.Errorf("expected 200 u1, got %d %q", w.Code, w.Body.String())
	}
}

func TestActorFromContextEmpty(t *testing.T) {
	if middleware.ActorFromContext(context.Background()) != nil {
		t.Error("expected nil actor from empty context")
	}
}

func TestActorHasPermission(t *testing.T) {
	admin := &middleware.Actor{Roles: []string{"admin"}}
	if !admin.HasPermission("anything.at.all") {
		t.Error("admin role should grant any permission")
	}

	m := &middleware.Actor{Roles: []string{"mitglied"}, Permissions: []string{"units.read"}}
	if !m.HasPermission("units.read") {
		t.Error("expected units.read to be granted")
	}
	if m.HasPermission("units.write") {
		t.Error("did not expect units.write to be granted")
	}
}
