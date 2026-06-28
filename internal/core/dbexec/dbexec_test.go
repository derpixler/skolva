package dbexec_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/core/dbexec"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const insertUser = `INSERT INTO users (email, password_hash, first_name, last_name)
VALUES ($1, $2, 'Test', 'User') RETURNING id`

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

	schemaContent, err := os.ReadFile("../../../schema.sql")
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

func TestWithActor(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()

	var actorID uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "actor@example.com", "hash").Scan(&actorID); err != nil {
		t.Fatalf("insert actor: %v", err)
	}

	// Commit path: subject is created and tagged with the actor in audit_logs.
	var subjectID uuid.UUID
	err := dbexec.WithActor(ctx, pool, actorID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, insertUser, "subject@example.com", "hash").Scan(&subjectID)
	})
	if err != nil {
		t.Fatalf("WithActor commit: %v", err)
	}

	var cnt int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE id=$1", subjectID).Scan(&cnt); err != nil {
		t.Fatalf("count subject: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected subject committed, got count=%d", cnt)
	}

	var auditActor uuid.UUID
	err = pool.QueryRow(ctx,
		"SELECT actor_user_id FROM audit_logs WHERE table_name='users' AND record_pk=$1 AND action='INSERT'",
		subjectID.String()).Scan(&auditActor)
	if err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if auditActor != actorID {
		t.Errorf("expected audit actor %s, got %s", actorID, auditActor)
	}

	// Rollback path: fn error rolls back the inserted row.
	sentinel := errors.New("boom")
	err = dbexec.WithActor(ctx, pool, actorID, func(ctx context.Context, tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, insertUser, "rollback@example.com", "hash"); e != nil {
			return e
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE email=$1", "rollback@example.com").Scan(&cnt); err != nil {
		t.Fatalf("count rollback: %v", err)
	}
	if cnt != 0 {
		t.Errorf("expected rollback (count=0), got %d", cnt)
	}
}

func TestAllowDelete(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "victim@example.com", "hash").Scan(&uid); err != nil {
		t.Fatalf("insert victim: %v", err)
	}

	// Physical delete is blocked without AllowDelete.
	err := dbexec.WithActor(ctx, pool, uuid.Nil, func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
		return e
	})
	if err == nil {
		t.Fatal("expected physical delete to be blocked")
	}

	var cnt int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE id=$1", uid).Scan(&cnt); err != nil {
		t.Fatalf("count after blocked delete: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected user still present, got count=%d", cnt)
	}

	// Physical delete succeeds with AllowDelete.
	err = dbexec.WithActorOptions(ctx, pool, uuid.Nil, dbexec.Options{AllowDelete: true}, func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
		return e
	})
	if err != nil {
		t.Fatalf("WithActorOptions allow-delete: %v", err)
	}

	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE id=$1", uid).Scan(&cnt); err != nil {
		t.Fatalf("count after allowed delete: %v", err)
	}
	if cnt != 0 {
		t.Errorf("expected user deleted, got count=%d", cnt)
	}
}
