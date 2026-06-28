package module

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

const createSchemaMigrations = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	module     text        NOT NULL,
	version    bigint      NOT NULL,
	name       text        NOT NULL,
	applied_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (module, version)
)`

// Migrate applies every module's pending migrations in registration order;
// each module's own migrations run in ascending version order. Applied
// migrations are recorded in schema_migrations, so re-runs are idempotent.
// Each migration runs in its own transaction (DDL + bookkeeping commit
// together or not at all).
func (r *Registry) Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, createSchemaMigrations); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, m := range r.modules {
		migs := append([]Migration(nil), m.Migrations()...)
		sort.Slice(migs, func(i, j int) bool { return migs[i].Version < migs[j].Version })

		for _, mig := range migs {
			applied, err := migrationApplied(ctx, pool, m.Name(), mig.Version)
			if err != nil {
				return err
			}
			if applied {
				continue
			}
			if err := applyMigration(ctx, pool, m.Name(), mig); err != nil {
				return fmt.Errorf("module %s migration %d (%s): %w", m.Name(), mig.Version, mig.Name, err)
			}
		}
	}
	return nil
}

func migrationApplied(ctx context.Context, pool *pgxpool.Pool, module string, version int64) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE module = $1 AND version = $2)`,
		module, version,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %s/%d: %w", module, version, err)
	}
	return exists, nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, module string, mig Migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, mig.SQL); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (module, version, name) VALUES ($1, $2, $3)`,
		module, mig.Version, mig.Name,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
