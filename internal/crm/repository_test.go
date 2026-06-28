package crm_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/crm"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
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

func TestAddressRepository(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := crm.NewRepository(pool)

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "a@example.com", "h").Scan(&uid); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	actor := uuid.NullUUID{UUID: uid, Valid: true}

	row, err := repo.UpsertAddress(ctx, db.UpsertUserAddressParams{
		UserID: uid, Street1: "Main 1", PostalCode: "12345", City: "Berlin", CountryCode: "DE", Actor: actor,
	})
	if err != nil || row.Street1 != "Main 1" || row.City != "Berlin" || row.CountryCode != "DE" {
		t.Fatalf("upsert address: row=%+v err=%v", row, err)
	}

	got, err := repo.GetAddress(ctx, uid)
	if err != nil || got.Street1 != "Main 1" {
		t.Fatalf("get address: row=%+v err=%v", got, err)
	}

	// upsert again -> 1:1, updated in place
	if _, err := repo.UpsertAddress(ctx, db.UpsertUserAddressParams{
		UserID: uid, Street1: "Main 2", PostalCode: "54321", City: "Hamburg", CountryCode: "DE", Actor: actor,
	}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, _ = repo.GetAddress(ctx, uid)
	if got.Street1 != "Main 2" || got.City != "Hamburg" {
		t.Errorf("expected updated address, got %+v", got)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM user_address WHERE user_id=$1", uid).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 address row (1:1), got %d", count)
	}
}

func TestPreferencesRepository(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()
	repo := crm.NewRepository(pool)

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "p@example.com", "h").Scan(&uid); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	actor := uuid.NullUUID{UUID: uid, Valid: true}

	row, err := repo.UpsertPreferences(ctx, db.UpsertUserPreferencesParams{
		UserID: uid, PreferredContactType: pgtype.Text{String: "email", Valid: true}, Actor: actor,
	})
	if err != nil || row.PreferredContactType.String != "email" {
		t.Fatalf("upsert preferences: row=%+v err=%v", row, err)
	}

	got, err := repo.GetPreferences(ctx, uid)
	if err != nil || got.PreferredContactType.String != "email" {
		t.Fatalf("get preferences: row=%+v err=%v", got, err)
	}

	// update
	if _, err := repo.UpsertPreferences(ctx, db.UpsertUserPreferencesParams{
		UserID: uid, PreferredContactType: pgtype.Text{String: "phone", Valid: true}, Actor: actor,
	}); err != nil {
		t.Fatalf("re-upsert preferences: %v", err)
	}
	got, _ = repo.GetPreferences(ctx, uid)
	if got.PreferredContactType.String != "phone" {
		t.Errorf("expected phone, got %q", got.PreferredContactType.String)
	}
}
