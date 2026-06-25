package auth

import (
	"context"

	"github.com/derpixler/skolva/internal/core/dbexec"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides data access for the identity tables (users, roles,
// permissions, user_roles, role_permissions). Reads run on the pool; writes
// run inside an actor transaction (dbexec.WithActor) so audit triggers and the
// soft-delete guard observe the acting user.
type Repository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: db.New(pool)}
}

// --- users ---

func (r *Repository) CreateUser(ctx context.Context, actorID uuid.UUID, params db.CreateUserParams) (db.CreateUserRow, error) {
	var out db.CreateUserRow
	err := dbexec.WithActor(ctx, r.pool, actorID, func(ctx context.Context, tx pgx.Tx) error {
		var e error
		out, e = db.New(tx).CreateUser(ctx, params)
		return e
	})
	return out, err
}

func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (db.GetUserByIDRow, error) {
	return r.q.GetUserByID(ctx, id)
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (db.GetUserByEmailRow, error) {
	return r.q.GetUserByEmail(ctx, email)
}

func (r *Repository) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	return r.q.UserExistsByEmail(ctx, email)
}

func (r *Repository) ListUsers(ctx context.Context, limit, offset int32) ([]db.ListUsersRow, error) {
	return r.q.ListUsers(ctx, db.ListUsersParams{Limit: limit, Offset: offset})
}

func (r *Repository) CountActiveUsers(ctx context.Context) (int64, error) {
	return r.q.CountActiveUsers(ctx)
}

func (r *Repository) UpdateUser(ctx context.Context, actorID uuid.UUID, params db.UpdateUserParams) (db.UpdateUserRow, error) {
	var out db.UpdateUserRow
	err := dbexec.WithActor(ctx, r.pool, actorID, func(ctx context.Context, tx pgx.Tx) error {
		var e error
		out, e = db.New(tx).UpdateUser(ctx, params)
		return e
	})
	return out, err
}

func (r *Repository) SoftDeleteUser(ctx context.Context, actorID uuid.UUID, params db.SoftDeleteUserParams) error {
	return dbexec.WithActor(ctx, r.pool, actorID, func(ctx context.Context, tx pgx.Tx) error {
		return db.New(tx).SoftDeleteUser(ctx, params)
	})
}
