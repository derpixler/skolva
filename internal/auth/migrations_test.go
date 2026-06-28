package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/core/module"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// newMigratedPool builds a database purely from the identity module's
// migrations (no schema.sql), proving the per-module migration cutover.
func newMigratedPool(t *testing.T) *pgxpool.Pool {
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

	// Bootstrap the schema solely from the identity module's migrations.
	if err := module.NewRegistry(auth.NewModule(nil)).Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate identity: %v", err)
	}
	return pool
}

// TestIdentityMigrationsBootstrapRepository proves that the identity module's
// migrations (run via Registry.Migrate, no schema.sql) produce a schema the
// auth repository works against, including the seeded RBAC.
func TestIdentityMigrationsBootstrapRepository(t *testing.T) {
	pool := newMigratedPool(t)
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	hash, _ := auth.HashPassword("password123")
	u, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "proof@example.com", PasswordHash: hash, FirstName: "Proof", LastName: "User",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := repo.AssignRole(ctx, u.ID, "mitglied", uuid.NullUUID{UUID: u.ID, Valid: true}); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	perms, err := repo.GetPermissionsForUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("get permissions: %v", err)
	}
	// The seeded "mitglied" role grants exactly 7 read permissions.
	if len(perms) != 7 {
		t.Errorf("mitglied permissions: want 7, got %d (%v)", len(perms), perms)
	}
}
