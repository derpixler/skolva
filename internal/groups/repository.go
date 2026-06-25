package groups

import (
	"context"

	"github.com/derpixler/skolva/internal/core/dbexec"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides data access for groups. Reads run on the pool; writes
// run inside an actor transaction (dbexec.WithActor) so audit triggers and the
// soft-delete guard observe the acting user.
type Repository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: db.New(pool)}
}

func (r *Repository) Create(ctx context.Context, actorID uuid.UUID, params db.CreateGroupParams) (db.CreateGroupRow, error) {
	var out db.CreateGroupRow
	err := dbexec.WithActor(ctx, r.pool, actorID, func(ctx context.Context, tx pgx.Tx) error {
		var e error
		out, e = db.New(tx).CreateGroup(ctx, params)
		return e
	})
	return out, err
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (db.GetGroupByIDRow, error) {
	return r.q.GetGroupByID(ctx, id)
}

func (r *Repository) List(ctx context.Context, limit, offset int32) ([]db.ListGroupsRow, error) {
	return r.q.ListGroups(ctx, db.ListGroupsParams{Limit: limit, Offset: offset})
}

func (r *Repository) ListByType(ctx context.Context, groupType string, limit, offset int32) ([]db.ListGroupsByTypeRow, error) {
	return r.q.ListGroupsByType(ctx, db.ListGroupsByTypeParams{GroupType: groupType, Limit: limit, Offset: offset})
}

func (r *Repository) Update(ctx context.Context, actorID uuid.UUID, params db.UpdateGroupParams) (db.UpdateGroupRow, error) {
	var out db.UpdateGroupRow
	err := dbexec.WithActor(ctx, r.pool, actorID, func(ctx context.Context, tx pgx.Tx) error {
		var e error
		out, e = db.New(tx).UpdateGroup(ctx, params)
		return e
	})
	return out, err
}

func (r *Repository) SoftDelete(ctx context.Context, actorID uuid.UUID, params db.SoftDeleteGroupParams) error {
	return dbexec.WithActor(ctx, r.pool, actorID, func(ctx context.Context, tx pgx.Tx) error {
		return db.New(tx).SoftDeleteGroup(ctx, params)
	})
}
