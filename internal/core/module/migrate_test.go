package module_test

import (
	"context"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/core/module"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	c, err := postgres.Run(ctx, "postgres:16-alpine",
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
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	connStr, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestRegistryMigrateAppliesAndIsIdempotent(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()

	m := &fakeModule{
		name: "demo",
		migs: []module.Migration{
			{Version: 1, Name: "create_widgets", SQL: "CREATE TABLE widgets (id int PRIMARY KEY)"},
			{Version: 2, Name: "add_label", SQL: "ALTER TABLE widgets ADD COLUMN label text"},
		},
	}
	r := module.NewRegistry(m)

	if err := r.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var cols int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns WHERE table_name = 'widgets'`,
	).Scan(&cols); err != nil {
		t.Fatalf("query columns: %v", err)
	}
	if cols != 2 {
		t.Errorf("widgets columns: want 2, got %d", cols)
	}

	recorded := func() int {
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM schema_migrations WHERE module = 'demo'`,
		).Scan(&n); err != nil {
			t.Fatalf("query schema_migrations: %v", err)
		}
		return n
	}
	if got := recorded(); got != 2 {
		t.Errorf("recorded migrations: want 2, got %d", got)
	}

	// idempotent: a second run applies nothing and records no duplicates
	if err := r.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate (2nd run): %v", err)
	}
	if got := recorded(); got != 2 {
		t.Errorf("after re-run: want 2 records, got %d", got)
	}
}
