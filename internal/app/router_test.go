package app_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/derpixler/skolva-core/database"
	"github.com/derpixler/skolva-core/middleware"
	"github.com/derpixler/skolva-core/module"
	"github.com/derpixler/skolva/internal/app"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func noopVerifier(string) (*middleware.Actor, error) { return nil, nil }

func startPostgres(t *testing.T) (*database.Pools, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("vv"),
		postgres.WithUsername("vv"),
		postgres.WithPassword("vv"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("failed to get connection string: %v", err)
	}

	pools, err := database.NewPools(ctx, connStr)
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("failed to create pools: %v", err)
	}

	return pools, func() {
		_ = pgContainer.Terminate(ctx)
	}
}

func TestNewRouterHealth(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") == "1" {
		t.Skip("skipping integration test")
	}

	pools, cleanup := startPostgres(t)
	defer cleanup()
	defer pools.Close()

	router := app.NewRouter(pools, module.NewRegistry(), module.Deps{}, noopVerifier)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestNewRouterUnhealthy(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") == "1" {
		t.Skip("skipping integration test")
	}

	pools, cleanup := startPostgres(t)
	defer cleanup()

	router := app.NewRouter(pools, module.NewRegistry(), module.Deps{}, noopVerifier)

	// Close the pools so the health check fails.
	pools.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Errorf("expected 503, got %d", w.Code)
	}
}
