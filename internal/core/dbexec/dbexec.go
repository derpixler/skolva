// Package dbexec runs database work inside an actor-scoped transaction so the
// schema's audit triggers (current_setting('app.actor_user_id')) and the
// soft-delete guard (current_setting('app.allow_delete')) observe the acting
// user and the intended delete policy.
package dbexec

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxFunc receives the active transaction. Every query executed on tx runs
// inside the same actor scope and is committed atomically.
type TxFunc func(ctx context.Context, tx pgx.Tx) error

// Options tunes the actor transaction.
type Options struct {
	// AllowDelete permits physical DELETEs by bypassing prevent_permanent_delete.
	// Leave false to enforce soft-delete (deleted_at).
	AllowDelete bool
}

// WithActor runs fn in a transaction tagged with actorID for audit logging.
// Pass uuid.Nil for system/anonymous actions (audit actor will be NULL).
func WithActor(ctx context.Context, pool *pgxpool.Pool, actorID uuid.UUID, fn TxFunc) error {
	return WithActorOptions(ctx, pool, actorID, Options{}, fn)
}

// WithActorOptions is WithActor with additional transaction controls.
func WithActorOptions(ctx context.Context, pool *pgxpool.Pool, actorID uuid.UUID, opts Options, fn TxFunc) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if actorID != uuid.Nil {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.actor_user_id', $1, true)", actorID.String()); err != nil {
			return fmt.Errorf("set actor: %w", err)
		}
	}
	if opts.AllowDelete {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.allow_delete', '1', true)"); err != nil {
			return fmt.Errorf("set allow_delete: %w", err)
		}
	}

	if err := fn(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	committed = true
	return nil
}
