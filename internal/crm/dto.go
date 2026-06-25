package crm

import (
	"time"

	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// --- address ---

type addressRequest struct {
	Company     *string `json:"company"`
	CareOf      *string `json:"care_of"`
	Street1     string  `json:"street1" binding:"required"`
	Street2     *string `json:"street2"`
	PostalCode  string  `json:"postal_code" binding:"required"`
	City        string  `json:"city" binding:"required"`
	State       *string `json:"state"`
	CountryCode string  `json:"country_code" binding:"required"`
	Note        *string `json:"note"`
}

// Address is the API representation of a user's address.
type Address struct {
	UserID      uuid.UUID `json:"user_id"`
	Company     *string   `json:"company,omitempty"`
	CareOf      *string   `json:"care_of,omitempty"`
	Street1     string    `json:"street1"`
	Street2     *string   `json:"street2,omitempty"`
	PostalCode  string    `json:"postal_code"`
	City        string    `json:"city"`
	State       *string   `json:"state,omitempty"`
	CountryCode string    `json:"country_code"`
	Note        *string   `json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func addressFromGet(r db.GetUserAddressRow) Address {
	return Address{
		UserID:      r.UserID,
		Company:     ptrFromText(r.Company),
		CareOf:      ptrFromText(r.CareOf),
		Street1:     r.Street1,
		Street2:     ptrFromText(r.Street2),
		PostalCode:  r.PostalCode,
		City:        r.City,
		State:       ptrFromText(r.State),
		CountryCode: r.CountryCode,
		Note:        ptrFromText(r.Note),
		CreatedAt:   r.CreatedAt.Time,
		UpdatedAt:   r.UpdatedAt.Time,
	}
}

func addressFromUpsert(r db.UpsertUserAddressRow) Address {
	return Address{
		UserID:      r.UserID,
		Company:     ptrFromText(r.Company),
		CareOf:      ptrFromText(r.CareOf),
		Street1:     r.Street1,
		Street2:     ptrFromText(r.Street2),
		PostalCode:  r.PostalCode,
		City:        r.City,
		State:       ptrFromText(r.State),
		CountryCode: r.CountryCode,
		Note:        ptrFromText(r.Note),
		CreatedAt:   r.CreatedAt.Time,
		UpdatedAt:   r.UpdatedAt.Time,
	}
}

// --- preferences ---

type preferencesRequest struct {
	PreferredContactType *string `json:"preferred_contact_type"`
	Note                 *string `json:"note"`
}

// Preferences is the API representation of a user's contact preferences.
type Preferences struct {
	UserID               uuid.UUID `json:"user_id"`
	PreferredContactType *string   `json:"preferred_contact_type,omitempty"`
	Note                 *string   `json:"note,omitempty"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func preferencesFromGet(r db.GetUserPreferencesRow) Preferences {
	return Preferences{
		UserID:               r.UserID,
		PreferredContactType: ptrFromText(r.PreferredContactType),
		Note:                 ptrFromText(r.Note),
		UpdatedAt:            r.UpdatedAt.Time,
	}
}

func preferencesFromUpsert(r db.UpsertUserPreferencesRow) Preferences {
	return Preferences{
		UserID:               r.UserID,
		PreferredContactType: ptrFromText(r.PreferredContactType),
		Note:                 ptrFromText(r.Note),
		UpdatedAt:            r.UpdatedAt.Time,
	}
}

// --- contacts ---

type createContactRequest struct {
	ContactType         string  `json:"contact_type" binding:"required"`
	Label               *string `json:"label"`
	Value               string  `json:"value" binding:"required"`
	IsPrimary           bool    `json:"is_primary"`
	IsPreferred         bool    `json:"is_preferred"`
	AllowContact        *bool   `json:"allow_contact"`
	PreferredTimeWindow *string `json:"preferred_time_window"`
	Note                *string `json:"note"`
}

type updateContactRequest struct {
	Label               *string `json:"label"`
	Value               string  `json:"value" binding:"required"`
	IsPrimary           bool    `json:"is_primary"`
	IsPreferred         bool    `json:"is_preferred"`
	AllowContact        *bool   `json:"allow_contact"`
	PreferredTimeWindow *string `json:"preferred_time_window"`
	Note                *string `json:"note"`
}

// Contact is the API representation of a user contact point.
type Contact struct {
	ID                  uuid.UUID  `json:"id"`
	UserID              uuid.UUID  `json:"user_id"`
	ContactType         string     `json:"contact_type"`
	Label               *string    `json:"label,omitempty"`
	Value               string     `json:"value"`
	IsPrimary           bool       `json:"is_primary"`
	IsPreferred         bool       `json:"is_preferred"`
	AllowContact        bool       `json:"allow_contact"`
	PreferredTimeWindow *string    `json:"preferred_time_window,omitempty"`
	VerifiedAt          *time.Time `json:"verified_at,omitempty"`
	Note                *string    `json:"note,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func contactFrom(id, userID uuid.UUID, contactType string, label pgtype.Text, value string, isPrimary, isPreferred, allowContact bool, ptw pgtype.Text, verifiedAt pgtype.Timestamptz, note pgtype.Text, created, updated pgtype.Timestamptz) Contact {
	c := Contact{
		ID:                  id,
		UserID:              userID,
		ContactType:         contactType,
		Label:               ptrFromText(label),
		Value:               value,
		IsPrimary:           isPrimary,
		IsPreferred:         isPreferred,
		AllowContact:        allowContact,
		PreferredTimeWindow: ptrFromText(ptw),
		Note:                ptrFromText(note),
		CreatedAt:           created.Time,
		UpdatedAt:           updated.Time,
	}
	if verifiedAt.Valid {
		v := verifiedAt.Time
		c.VerifiedAt = &v
	}
	return c
}

func contactFromCreate(r db.CreateContactRow) Contact {
	return contactFrom(r.ID, r.UserID, r.ContactType, r.Label, r.Value, r.IsPrimary, r.IsPreferred, r.AllowContact, r.PreferredTimeWindow, r.VerifiedAt, r.Note, r.CreatedAt, r.UpdatedAt)
}

func contactFromUpdate(r db.UpdateContactRow) Contact {
	return contactFrom(r.ID, r.UserID, r.ContactType, r.Label, r.Value, r.IsPrimary, r.IsPreferred, r.AllowContact, r.PreferredTimeWindow, r.VerifiedAt, r.Note, r.CreatedAt, r.UpdatedAt)
}

func contactFromList(r db.ListContactsRow) Contact {
	return contactFrom(r.ID, r.UserID, r.ContactType, r.Label, r.Value, r.IsPrimary, r.IsPreferred, r.AllowContact, r.PreferredTimeWindow, r.VerifiedAt, r.Note, r.CreatedAt, r.UpdatedAt)
}

// --- helpers ---

func ptrFromText(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func textFromPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
