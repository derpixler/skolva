package crm

import (
	"context"

	"github.com/derpixler/skolva/internal/core/dbexec"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides data access for the CRM tables (user_address,
// user_contact_points, user_preferences).
type Repository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: db.New(pool)}
}

// --- address (not audited -> direct pool; created_by/updated_by via params) ---

func (r *Repository) GetAddress(ctx context.Context, userID uuid.UUID) (db.GetUserAddressRow, error) {
	return r.q.GetUserAddress(ctx, userID)
}

func (r *Repository) UpsertAddress(ctx context.Context, params db.UpsertUserAddressParams) (db.UpsertUserAddressRow, error) {
	return r.q.UpsertUserAddress(ctx, params)
}

// --- preferences (not audited -> direct pool) ---

func (r *Repository) GetPreferences(ctx context.Context, userID uuid.UUID) (db.GetUserPreferencesRow, error) {
	return r.q.GetUserPreferences(ctx, userID)
}

func (r *Repository) UpsertPreferences(ctx context.Context, params db.UpsertUserPreferencesParams) (db.UpsertUserPreferencesRow, error) {
	return r.q.UpsertUserPreferences(ctx, params)
}

// --- contacts (audited -> writes via dbexec.WithActor) ---

func (r *Repository) ListContacts(ctx context.Context, userID uuid.UUID) ([]db.ListContactsRow, error) {
	return r.q.ListContacts(ctx, userID)
}

func (r *Repository) GetContact(ctx context.Context, id uuid.UUID) (db.GetContactRow, error) {
	return r.q.GetContact(ctx, id)
}

func (r *Repository) CreateContact(ctx context.Context, actorID uuid.UUID, clearPrimary bool, params db.CreateContactParams) (db.CreateContactRow, error) {
	var out db.CreateContactRow
	err := dbexec.WithActor(ctx, r.pool, actorID, func(ctx context.Context, tx pgx.Tx) error {
		q := db.New(tx)
		if clearPrimary {
			if err := q.ClearPrimaryContacts(ctx, db.ClearPrimaryContactsParams{
				UserID: params.UserID, ContactType: params.ContactType, ExcludeID: uuid.Nil,
			}); err != nil {
				return err
			}
		}
		var e error
		out, e = q.CreateContact(ctx, params)
		return e
	})
	return out, err
}

func (r *Repository) UpdateContact(ctx context.Context, actorID, userID uuid.UUID, contactType string, clearPrimary bool, params db.UpdateContactParams) (db.UpdateContactRow, error) {
	var out db.UpdateContactRow
	err := dbexec.WithActor(ctx, r.pool, actorID, func(ctx context.Context, tx pgx.Tx) error {
		q := db.New(tx)
		if clearPrimary {
			if err := q.ClearPrimaryContacts(ctx, db.ClearPrimaryContactsParams{
				UserID: userID, ContactType: contactType, ExcludeID: params.ID,
			}); err != nil {
				return err
			}
		}
		var e error
		out, e = q.UpdateContact(ctx, params)
		return e
	})
	return out, err
}

func (r *Repository) SoftDeleteContact(ctx context.Context, actorID uuid.UUID, params db.SoftDeleteContactParams) error {
	return dbexec.WithActor(ctx, r.pool, actorID, func(ctx context.Context, tx pgx.Tx) error {
		return db.New(tx).SoftDeleteContact(ctx, params)
	})
}
