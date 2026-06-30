package app_test

import (
	"strings"
	"testing"

	"github.com/derpixler/skolva-core/database"
	"github.com/derpixler/skolva-core/module"
	apispec "github.com/derpixler/skolva/api"
	"github.com/derpixler/skolva/internal/app"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
)

func TestOpenAPISpecValid(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(apispec.Spec)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("invalid openapi.yaml: %v", err)
	}
}

// normalizePath turns a gin route path into an OpenAPI path:
// strips the /api server prefix and converts :param / *param to {param}.
func normalizePath(p string) string {
	p = strings.TrimPrefix(p, "/api")
	segs := strings.Split(p, "/")
	for i, s := range segs {
		if strings.HasPrefix(s, ":") || strings.HasPrefix(s, "*") {
			segs[i] = "{" + s[1:] + "}"
		}
	}
	return strings.Join(segs, "/")
}

func TestOpenAPIRouteParity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(apispec.Spec)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}

	spec := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			spec[method+" "+path] = true
		}
	}

	// Build the real router without a database: route registration does not
	// query, so nil pools are fine for enumerating the route table.
	engine := app.NewRouter(&database.Pools{}, app.DefaultRegistry(nil), module.Deps{}, noopVerifier)

	exempt := map[string]bool{
		"GET /openapi.yaml":  true,
		"GET /docs":          true,
		"GET /docs/redoc.js": true,
	}

	routes := map[string]bool{}
	for _, r := range engine.Routes() {
		key := r.Method + " " + normalizePath(r.Path)
		if exempt[key] {
			continue
		}
		routes[key] = true
		if !spec[key] {
			t.Errorf("mounted route %q is not documented in openapi.yaml", key)
		}
	}
	for key := range spec {
		if !routes[key] {
			t.Errorf("documented operation %q has no matching mounted route", key)
		}
	}
}
