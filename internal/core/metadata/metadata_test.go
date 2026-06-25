package metadata_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/core/metadata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const insertUser = `INSERT INTO users (email, password_hash, first_name, last_name)
VALUES ($1, $2, 'Test', 'User') RETURNING id`

func TestNewStoreUnknownTable(t *testing.T) {
	if _, err := metadata.NewStore("not_a_meta_table"); !errors.Is(err, metadata.ErrUnknownTable) {
		t.Errorf("expected ErrUnknownTable, got %v", err)
	}
}

func TestNewStoreValid(t *testing.T) {
	if _, err := metadata.NewStore("users_meta"); err != nil {
		t.Errorf("unexpected error for valid table: %v", err)
	}
}

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

func TestStoreCRUD(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "meta@example.com", "hash").Scan(&uid); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	store, err := metadata.NewStore("users_meta")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	// missing key
	if _, found, err := store.Get(ctx, pool, uid, "nickname"); err != nil || found {
		t.Fatalf("expected not found, got found=%v err=%v", found, err)
	}

	// set + get
	if err := store.Set(ctx, pool, uid, "nickname", "Ren"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if v, found, err := store.Get(ctx, pool, uid, "nickname"); err != nil || !found || v != "Ren" {
		t.Fatalf("get after set: v=%q found=%v err=%v", v, found, err)
	}

	// update (upsert)
	if err := store.Set(ctx, pool, uid, "nickname", "Rene"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if v, _, _ := store.Get(ctx, pool, uid, "nickname"); v != "Rene" {
		t.Errorf("expected updated value, got %q", v)
	}

	// all
	if err := store.Set(ctx, pool, uid, "lang", "de"); err != nil {
		t.Fatalf("set lang: %v", err)
	}
	all, err := store.All(ctx, pool, uid)
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if len(all) != 2 || all["nickname"] != "Rene" || all["lang"] != "de" {
		t.Errorf("unexpected all: %v", all)
	}

	// delete
	if err := store.Delete(ctx, pool, uid, "nickname"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, found, _ := store.Get(ctx, pool, uid, "nickname"); found {
		t.Error("expected nickname deleted")
	}
	all, err = store.All(ctx, pool, uid)
	if err != nil {
		t.Fatalf("all after delete: %v", err)
	}
	if len(all) != 1 || all["lang"] != "de" {
		t.Errorf("unexpected all after delete: %v", all)
	}
}
