package jobs_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/derpixler/skolva/internal/core/database"
	"github.com/derpixler/skolva/internal/core/jobs"
)

func TestNewWorker(t *testing.T) {
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
		t.Fatalf("failed to start postgres: %v", err)
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	pools, err := database.NewPools(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create pools: %v", err)
	}
	defer pools.Close()

	schemaContent, err := os.ReadFile("../../../schema.sql")
	if err != nil {
		t.Fatalf("failed to read schema.sql: %v", err)
	}

	_, err = pools.Web.Exec(ctx, string(schemaContent))
	if err != nil {
		t.Fatalf("failed to execute schema: %v", err)
	}

	worker, err := jobs.NewWorker(ctx, pools.Worker)
	if err != nil {
		t.Fatalf("failed to create worker: %v", err)
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		if err := worker.Start(workerCtx); err != nil {
			t.Logf("worker error: %v", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)

	cancel()
	if err := worker.Stop(context.Background()); err != nil {
		t.Logf("stop error: %v", err)
	}
}
