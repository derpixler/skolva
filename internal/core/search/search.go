// Package search runs German full-text search against the schema's
// search_vector (tsvector) columns using websearch_to_tsquery + ts_rank.
package search

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrUnknownTable is returned by NewSearcher for a table without a known
// search_vector column.
var ErrUnknownTable = errors.New("unknown searchable table")

// allowedTables are the tables that have a search_vector column + GIN index in
// schema.sql. The table name is interpolated into SQL, so it must come only
// from this list (never from user input).
var allowedTables = map[string]bool{
	"users":             true,
	"units":             true,
	"documents":         true,
	"bank_transactions": true,
	"lendable_items":    true,
	"incidents":         true,
	"work_task_catalog": true,
}

const (
	defaultLimit = 20
	maxLimit     = 100
)

// DBTX is satisfied by *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Result is a ranked search hit.
type Result struct {
	ID   uuid.UUID
	Rank float32
}

// Searcher runs full-text queries against one validated table's search_vector.
type Searcher struct {
	table string
}

// NewSearcher returns a Searcher for the table, or ErrUnknownTable.
func NewSearcher(table string) (*Searcher, error) {
	if !allowedTables[table] {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTable, table)
	}
	return &Searcher{table: table}, nil
}

// Search returns ranked, non-deleted matches for the query (German config).
// An empty query returns no results without touching the database. limit is
// clamped to (0, maxLimit]; limit <= 0 uses defaultLimit.
func (s *Searcher) Search(ctx context.Context, db DBTX, query string, limit int) ([]Result, error) {
	if strings.TrimSpace(query) == "" {
		return []Result{}, nil
	}
	switch {
	case limit <= 0:
		limit = defaultLimit
	case limit > maxLimit:
		limit = maxLimit
	}

	q := fmt.Sprintf(
		"SELECT id, ts_rank(search_vector, websearch_to_tsquery('german', $1)) AS rank "+
			"FROM %s "+
			"WHERE deleted_at IS NULL AND search_vector @@ websearch_to_tsquery('german', $1) "+
			"ORDER BY rank DESC, id "+
			"LIMIT $2",
		s.table,
	)

	rows, err := db.Query(ctx, q, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search %s: %w", s.table, err)
	}
	defer rows.Close()

	results := make([]Result, 0)
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.ID, &r.Rank); err != nil {
			return nil, fmt.Errorf("search scan: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search rows: %w", err)
	}
	return results, nil
}
