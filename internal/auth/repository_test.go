package auth_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func newSchemaPool(t *testing.T) (*pgxpool.Pool, func()) {
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

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	schemaContent, err := os.ReadFile("../../schema.sql")
	if err != nil {
		t.Fatalf("failed to read schema.sql: %v", err)
	}
	if _, err := pool.Exec(ctx, string(schemaContent)); err != nil {
		t.Fatalf("failed to apply schema: %v", err)
	}

	cleanup := func() {
		pool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}
	return pool, cleanup
}

func TestRepositoryUserLifecycle(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := auth.NewRepository(pool)

	// actor created without an acting user (created_by NULL, no audit actor)
	actorUser, err := repo.CreateUser(ctx, uuid.Nil, db.CreateUserParams{
		Email: "admin@example.com", PasswordHash: "h1", FirstName: "Ad", LastName: "Min",
	})
	if err != nil {
		t.Fatalf("create actor: %v", err)
	}

	// subject created by the actor (audit actor + created_by = actor)
	subject, err := repo.CreateUser(ctx, actorUser.ID, db.CreateUserParams{
		Email: "sub@example.com", PasswordHash: "h2", FirstName: "Sub", LastName: "Ject",
		Actor: uuid.NullUUID{UUID: actorUser.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}

	// GetUserByID
	got, err := repo.GetUserByID(ctx, subject.ID)
	if err != nil || got.Email != "sub@example.com" {
		t.Fatalf("get by id: row=%+v err=%v", got, err)
	}

	// GetUserByEmail is case-insensitive and exposes the hash for login
	byEmail, err := repo.GetUserByEmail(ctx, "SUB@example.com")
	if err != nil {
		t.Fatalf("get by email: %v", err)
	}
	if byEmail.ID != subject.ID || byEmail.PasswordHash != "h2" {
		t.Errorf("unexpected by-email row: %+v", byEmail)
	}

	// existence checks
	if ok, err := repo.UserExistsByEmail(ctx, "sub@example.com"); err != nil || !ok {
		t.Errorf("expected user to exist, got ok=%v err=%v", ok, err)
	}
	if ok, err := repo.UserExistsByEmail(ctx, "nobody@example.com"); err != nil || ok {
		t.Errorf("expected user not to exist, got ok=%v err=%v", ok, err)
	}

	// list + count
	if n, err := repo.CountActiveUsers(ctx); err != nil || n != 2 {
		t.Errorf("expected 2 active users, got %d err=%v", n, err)
	}
	list, err := repo.ListUsers(ctx, 10, 0)
	if err != nil || len(list) != 2 {
		t.Errorf("expected 2 listed users, got %d err=%v", len(list), err)
	}

	// audit: the subject's INSERT is attributed to the actor
	var auditActor uuid.UUID
	if err := pool.QueryRow(ctx,
		"SELECT actor_user_id FROM audit_logs WHERE table_name='users' AND record_pk=$1 AND action='INSERT'",
		subject.ID.String()).Scan(&auditActor); err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if auditActor != actorUser.ID {
		t.Errorf("expected audit actor %s, got %s", actorUser.ID, auditActor)
	}

	// update
	updated, err := repo.UpdateUser(ctx, actorUser.ID, db.UpdateUserParams{
		ID: subject.ID, FirstName: "New", LastName: "Name", IsActive: true,
		UpdatedBy: uuid.NullUUID{UUID: actorUser.ID, Valid: true},
	})
	if err != nil || updated.FirstName != "New" {
		t.Fatalf("update: row=%+v err=%v", updated, err)
	}

	// soft delete excludes the user from reads
	if err := repo.SoftDeleteUser(ctx, actorUser.ID, db.SoftDeleteUserParams{
		ID: subject.ID, UpdatedBy: uuid.NullUUID{UUID: actorUser.ID, Valid: true},
	}); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := repo.GetUserByID(ctx, subject.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected ErrNoRows after soft delete, got %v", err)
	}
	if n, err := repo.CountActiveUsers(ctx); err != nil || n != 1 {
		t.Errorf("expected 1 active user after soft delete, got %d err=%v", n, err)
	}
}
