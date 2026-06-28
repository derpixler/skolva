package search_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/derpixler/skolva/internal/core/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const insertUser = `INSERT INTO users (email, password_hash, first_name, last_name)
VALUES ($1, $2, $3, $4) RETURNING id`

func TestNewSearcherUnknownTable(t *testing.T) {
	if _, err := search.NewSearcher("not_searchable"); !errors.Is(err, search.ErrUnknownTable) {
		t.Errorf("expected ErrUnknownTable, got %v", err)
	}
}

func TestNewSearcherValid(t *testing.T) {
	if _, err := search.NewSearcher("users"); err != nil {
		t.Errorf("unexpected error for valid table: %v", err)
	}
}

func TestSearchEmptyQueryReturnsNoResults(t *testing.T) {
	s, err := search.NewSearcher("users")
	if err != nil {
		t.Fatalf("new searcher: %v", err)
	}
	// Empty query short-circuits before touching the database (nil DBTX is safe).
	res, err := s.Search(context.Background(), nil, "   ", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected no results, got %d", len(res))
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

func TestSearchUsers(t *testing.T) {
	pool, cleanup := newSchemaPool(t)
	defer cleanup()
	ctx := context.Background()

	var annaID, berndID uuid.UUID
	if err := pool.QueryRow(ctx, insertUser, "anna@example.com", "h", "Anna", "Schmidt").Scan(&annaID); err != nil {
		t.Fatalf("insert anna: %v", err)
	}
	if err := pool.QueryRow(ctx, insertUser, "bernd@example.com", "h", "Bernd", "Fischer").Scan(&berndID); err != nil {
		t.Fatalf("insert bernd: %v", err)
	}
	if _, err := pool.Exec(ctx, insertUser, "clara@example.com", "h", "Clara", "Weber"); err != nil {
		t.Fatalf("insert clara: %v", err)
	}

	s, err := search.NewSearcher("users")
	if err != nil {
		t.Fatalf("new searcher: %v", err)
	}

	// match by last name
	res, err := s.Search(ctx, pool, "Schmidt", 10)
	if err != nil {
		t.Fatalf("search Schmidt: %v", err)
	}
	if len(res) != 1 || res[0].ID != annaID {
		t.Fatalf("expected only Anna, got %+v", res)
	}

	// case-insensitive (German config folds case)
	res, err = s.Search(ctx, pool, "schmidt", 10)
	if err != nil {
		t.Fatalf("search schmidt: %v", err)
	}
	if len(res) != 1 || res[0].ID != annaID {
		t.Errorf("expected case-insensitive match for Anna, got %+v", res)
	}

	// another user
	res, err = s.Search(ctx, pool, "Fischer", 10)
	if err != nil {
		t.Fatalf("search Fischer: %v", err)
	}
	if len(res) != 1 || res[0].ID != berndID {
		t.Errorf("expected only Bernd, got %+v", res)
	}

	// no match
	res, err = s.Search(ctx, pool, "Zzxyqwv", 10)
	if err != nil {
		t.Fatalf("search no-match: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected no results, got %d", len(res))
	}

	// soft-deleted users are excluded
	if _, err := pool.Exec(ctx, "UPDATE users SET deleted_at = NOW() WHERE id = $1", annaID); err != nil {
		t.Fatalf("soft-delete anna: %v", err)
	}
	res, err = s.Search(ctx, pool, "Schmidt", 10)
	if err != nil {
		t.Fatalf("search after soft-delete: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected soft-deleted user excluded, got %d", len(res))
	}

	// limit > maxLimit (100) is clamped
	if _, err := s.Search(ctx, pool, "Schmidt", 200); err != nil {
		t.Errorf("search with high limit: %v", err)
	}
}
