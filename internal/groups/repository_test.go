package groups_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/db"
	"github.com/derpixler/skolva/internal/groups"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

func TestGroupRepositoryLifecycle(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := groups.NewRepository(pool)

	var actorID uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "actor@example.com", "h").Scan(&actorID); err != nil {
		t.Fatalf("insert actor: %v", err)
	}
	actor := uuid.NullUUID{UUID: actorID, Valid: true}

	g, err := repo.Create(ctx, actorID, db.CreateGroupParams{
		Name:        "Team A",
		Description: pgtype.Text{String: "first team", Valid: true},
		GroupType:   "mannschaft",
		Actor:       actor,
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if g.Name != "Team A" || g.GroupType != "mannschaft" || g.Description.String != "first team" {
		t.Fatalf("unexpected created group: %+v", g)
	}

	got, err := repo.GetByID(ctx, g.ID)
	if err != nil || got.Name != "Team A" {
		t.Fatalf("get by id: row=%+v err=%v", got, err)
	}

	all, err := repo.List(ctx, 10, 0)
	if err != nil || len(all) != 1 {
		t.Fatalf("expected 1 group, got %d err=%v", len(all), err)
	}

	// FindByType
	if byType, err := repo.ListByType(ctx, "mannschaft", 10, 0); err != nil || len(byType) != 1 {
		t.Errorf("expected 1 mannschaft, got %d err=%v", len(byType), err)
	}
	if byType, err := repo.ListByType(ctx, "abteilung", 10, 0); err != nil || len(byType) != 0 {
		t.Errorf("expected 0 abteilung, got %d err=%v", len(byType), err)
	}

	// audit: creation attributed to the actor
	var auditActor uuid.UUID
	if err := pool.QueryRow(ctx,
		"SELECT actor_user_id FROM audit_logs WHERE table_name='groups' AND record_pk=$1 AND action='INSERT'",
		g.ID.String()).Scan(&auditActor); err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if auditActor != actorID {
		t.Errorf("expected audit actor %s, got %s", actorID, auditActor)
	}

	// update (also changes type)
	upd, err := repo.Update(ctx, actorID, db.UpdateGroupParams{
		ID:        g.ID,
		Name:      "Team B",
		GroupType: "abteilung",
		IsActive:  true,
		UpdatedBy: actor,
	})
	if err != nil || upd.Name != "Team B" || upd.GroupType != "abteilung" {
		t.Fatalf("update: row=%+v err=%v", upd, err)
	}
	if byType, _ := repo.ListByType(ctx, "abteilung", 10, 0); len(byType) != 1 {
		t.Errorf("expected group under abteilung after update, got %d", len(byType))
	}
	if byType, _ := repo.ListByType(ctx, "mannschaft", 10, 0); len(byType) != 0 {
		t.Errorf("expected no group under mannschaft after update, got %d", len(byType))
	}

	// soft delete
	if err := repo.SoftDelete(ctx, actorID, db.SoftDeleteGroupParams{ID: g.ID, UpdatedBy: actor}); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, g.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected ErrNoRows after soft delete, got %v", err)
	}
	if all, err := repo.List(ctx, 10, 0); err != nil || len(all) != 0 {
		t.Errorf("expected 0 groups after soft delete, got %d err=%v", len(all), err)
	}
}
