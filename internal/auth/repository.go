package auth

import (
	"context"

	"github.com/derpixler/skolva-core/dbexec"
	"github.com/derpixler/skolva-core/search"
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

// --- roles & user_roles ---
//
// user_roles is neither audited nor soft-deletable, so its writes run directly
// on the pool; assignedBy records who performed the assignment.

func (r *Repository) ListRoles(ctx context.Context) ([]db.ListRolesRow, error) {
	return r.q.ListRoles(ctx)
}

func (r *Repository) GetRole(ctx context.Context, slug string) (db.GetRoleRow, error) {
	return r.q.GetRole(ctx, slug)
}

func (r *Repository) AssignRole(ctx context.Context, userID uuid.UUID, roleSlug string, assignedBy uuid.NullUUID) error {
	return r.q.AssignRole(ctx, db.AssignRoleParams{UserID: userID, RoleSlug: roleSlug, AssignedBy: assignedBy})
}

func (r *Repository) RemoveRole(ctx context.Context, userID uuid.UUID, roleSlug string) error {
	return r.q.RemoveRole(ctx, db.RemoveRoleParams{UserID: userID, RoleSlug: roleSlug})
}

func (r *Repository) ListUserRoles(ctx context.Context, userID uuid.UUID) ([]db.ListUserRolesRow, error) {
	return r.q.ListUserRoles(ctx, userID)
}

func (r *Repository) UserHasRole(ctx context.Context, userID uuid.UUID, roleSlug string) (bool, error) {
	return r.q.UserHasRole(ctx, db.UserHasRoleParams{UserID: userID, RoleSlug: roleSlug})
}

// --- permissions & role_permissions ---
//
// role_permissions is neither audited nor soft-deletable -> direct pool writes.

func (r *Repository) ListPermissions(ctx context.Context) ([]db.Permission, error) {
	return r.q.ListPermissions(ctx)
}

// GetPermissionsForUser resolves the user's effective permission slugs via
// user_roles -> role_permissions (active roles only). Used for JWT claims.
func (r *Repository) GetPermissionsForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	return r.q.GetPermissionsForUser(ctx, userID)
}

func (r *Repository) GetPermissionsForRole(ctx context.Context, roleSlug string) ([]db.Permission, error) {
	return r.q.GetPermissionsForRole(ctx, roleSlug)
}

func (r *Repository) AddRolePermission(ctx context.Context, roleSlug, permissionSlug string) error {
	return r.q.AddRolePermission(ctx, db.AddRolePermissionParams{RoleSlug: roleSlug, PermissionSlug: permissionSlug})
}

func (r *Repository) RemoveRolePermission(ctx context.Context, roleSlug, permissionSlug string) error {
	return r.q.RemoveRolePermission(ctx, db.RemoveRolePermissionParams{RoleSlug: roleSlug, PermissionSlug: permissionSlug})
}

// UserHasPermission reports whether the user holds permissionSlug via any of
// their active roles (role_permissions join).
func (r *Repository) UserHasPermission(ctx context.Context, userID uuid.UUID, permissionSlug string) (bool, error) {
	return r.q.UserHasPermission(ctx, db.UserHasPermissionParams{UserID: userID, PermissionSlug: permissionSlug})
}

// SearchUsers runs a German full-text search over users (core/search) and
// returns the matching rows ordered by relevance.
func (r *Repository) SearchUsers(ctx context.Context, q string, limit int) ([]db.GetUsersByIDsRow, error) {
	searcher, err := search.NewSearcher("users")
	if err != nil {
		return nil, err
	}
	results, err := searcher.Search(ctx, r.pool, q, limit)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []db.GetUsersByIDsRow{}, nil
	}
	ids := make([]uuid.UUID, len(results))
	for i, res := range results {
		ids[i] = res.ID
	}
	rows, err := r.q.GetUsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]db.GetUsersByIDsRow, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	ordered := make([]db.GetUsersByIDsRow, 0, len(ids))
	for _, id := range ids {
		if row, ok := byID[id]; ok {
			ordered = append(ordered, row)
		}
	}
	return ordered, nil
}

// --- 2FA (TOTP) ---
// user_totp_secrets is not audited/soft-deletable -> direct pool.

func (r *Repository) GetTOTPSecret(ctx context.Context, userID uuid.UUID) (db.UserTotpSecret, error) {
	return r.q.GetTOTPSecret(ctx, userID)
}

func (r *Repository) UpsertTOTPSecret(ctx context.Context, params db.UpsertTOTPSecretParams) (db.UserTotpSecret, error) {
	return r.q.UpsertTOTPSecret(ctx, params)
}

func (r *Repository) DeleteTOTPSecret(ctx context.Context, userID uuid.UUID) error {
	return r.q.DeleteTOTPSecret(ctx, userID)
}

func (r *Repository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	return r.q.UpdatePassword(ctx, db.UpdatePasswordParams{ID: userID, PasswordHash: passwordHash})
}
