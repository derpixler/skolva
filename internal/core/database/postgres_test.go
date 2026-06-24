package database_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/derpixler/skolva/internal/core/database"
)

func setupTestDB(t *testing.T) (*database.Pools, func()) {
	t.Helper()

	if os.Getenv("SKIP_INTEGRATION") == "1" {
		t.Skip("skipping integration test")
	}

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
		t.Fatalf("failed to start postgres container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	pools, err := database.NewPools(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create pools: %v", err)
	}

	cleanup := func() {
		pools.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return pools, cleanup
}

func TestNewPools(t *testing.T) {
	pools, cleanup := setupTestDB(t)
	defer cleanup()

	if pools.Web == nil {
		t.Fatal("expected web pool")
	}
	if pools.Worker == nil {
		t.Fatal("expected worker pool")
	}
}

func TestPoolsHealth(t *testing.T) {
	pools, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	if err := pools.Health(ctx); err != nil {
		t.Fatalf("health check failed: %v", err)
	}
}

func TestDualPoolIsolation(t *testing.T) {
	pools, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	if err := pools.Web.Ping(ctx); err != nil {
		t.Fatalf("web pool ping failed: %v", err)
	}

	if err := pools.Worker.Ping(ctx); err != nil {
		t.Fatalf("worker pool ping failed: %v", err)
	}

	webStats := pools.Web.Stat()
	workerStats := pools.Worker.Stat()

	if webStats.MaxConns() <= 0 {
		t.Error("web pool has no max connections")
	}
	if workerStats.MaxConns() <= 0 {
		t.Error("worker pool has no max connections")
	}
}

func TestSchemaExecution(t *testing.T) {
	pools, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	schemaContent, err := os.ReadFile("../../../schema.sql")
	if err != nil {
		t.Fatalf("failed to read schema.sql: %v", err)
	}

	_, err = pools.Web.Exec(ctx, string(schemaContent))
	if err != nil {
		t.Fatalf("failed to execute schema: %v", err)
	}

	var tableCount int
	err = pools.Web.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
	`).Scan(&tableCount)
	if err != nil {
		t.Fatalf("failed to count tables: %v", err)
	}

	if tableCount < 10 {
		t.Errorf("expected at least 10 tables, got %d", tableCount)
	}
}

func TestNewPoolsInvalidURL(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") == "1" {
		t.Skip("skipping integration test")
	}

	_, err := database.NewPools(context.Background(), "invalid://url")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestPoolsClose(t *testing.T) {
	pools, cleanup := setupTestDB(t)
	defer cleanup()

	pools.Close()
}

func TestPoolsHealthError(t *testing.T) {
	pools, cleanup := setupTestDB(t)
	defer cleanup()

	pools.Close()

	err := pools.Health(context.Background())
	if err == nil {
		t.Error("expected error after close")
	}
}
