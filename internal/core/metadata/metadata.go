// Package metadata is a generic handler for the schema's EAV *_meta tables
// (users_meta, groups_meta, ...). Each such table stores arbitrary
// (meta_key, meta_value) pairs per entity_id.
package metadata

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrUnknownTable is returned by NewStore for a table not in the allow-list.
var ErrUnknownTable = errors.New("unknown meta table")

// allowedTables is the fixed set of EAV meta tables defined in schema.sql.
// The table name is interpolated into SQL, so it must come from this list
// only (never from user input).
var allowedTables = map[string]bool{
	"users_meta":       true,
	"groups_meta":      true,
	"units_meta":       true,
	"leases_meta":      true,
	"documents_meta":   true,
	"applicants_meta":  true,
	"journal_meta":     true,
	"billing_meta":     true,
	"bank_tx_meta":     true,
	"lendable_meta":    true,
	"work_events_meta": true,
	"incidents_meta":   true,
}

// DBTX is satisfied by *pgxpool.Pool and pgx.Tx, so a Store works both on a
// pool directly and inside an actor transaction (dbexec.WithActor).
type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Store reads and writes meta key/value pairs for one validated meta table.
type Store struct {
	table string
}

// NewStore returns a Store for the given meta table, or ErrUnknownTable if the
// name is not a known EAV table.
func NewStore(table string) (*Store, error) {
	if !allowedTables[table] {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTable, table)
	}
	return &Store{table: table}, nil
}

// Get returns the value for key. found is false when no row exists.
func (s *Store) Get(ctx context.Context, db DBTX, entityID uuid.UUID, key string) (value string, found bool, err error) {
	q := fmt.Sprintf("SELECT meta_value FROM %s WHERE entity_id = $1 AND meta_key = $2", s.table)
	var v pgtype.Text
	switch err := db.QueryRow(ctx, q, entityID, key).Scan(&v); {
	case errors.Is(err, pgx.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("metadata get: %w", err)
	default:
		return v.String, true, nil
	}
}

// Set inserts or updates the value for key (upsert on entity_id, meta_key).
func (s *Store) Set(ctx context.Context, db DBTX, entityID uuid.UUID, key, value string) error {
	q := fmt.Sprintf(
		"INSERT INTO %s (entity_id, meta_key, meta_value) VALUES ($1, $2, $3) "+
			"ON CONFLICT (entity_id, meta_key) DO UPDATE SET meta_value = EXCLUDED.meta_value",
		s.table,
	)
	if _, err := db.Exec(ctx, q, entityID, key, value); err != nil {
		return fmt.Errorf("metadata set: %w", err)
	}
	return nil
}

// Delete removes the key for the entity (no-op if absent).
func (s *Store) Delete(ctx context.Context, db DBTX, entityID uuid.UUID, key string) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE entity_id = $1 AND meta_key = $2", s.table)
	if _, err := db.Exec(ctx, q, entityID, key); err != nil {
		return fmt.Errorf("metadata delete: %w", err)
	}
	return nil
}

// All returns every meta key/value pair for the entity.
func (s *Store) All(ctx context.Context, db DBTX, entityID uuid.UUID) (map[string]string, error) {
	q := fmt.Sprintf("SELECT meta_key, meta_value FROM %s WHERE entity_id = $1", s.table)
	rows, err := db.Query(ctx, q, entityID)
	if err != nil {
		return nil, fmt.Errorf("metadata all: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k string
		var v pgtype.Text
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("metadata all scan: %w", err)
		}
		out[k] = v.String
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("metadata all rows: %w", err)
	}
	return out, nil
}
