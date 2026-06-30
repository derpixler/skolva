package crm

import (
	"context"
	"errors"
	"strings"

	apperrors "github.com/derpixler/skolva-core/errors"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var validPreferredContactTypes = map[string]bool{
	"email":  true,
	"phone":  true,
	"mobile": true,
	"postal": true,
	"other":  true,
}

// Service holds the business logic for the CRM module.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func actorNull(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: id, Valid: id != uuid.Nil}
}

func validCountryCode(cc string) bool {
	if len(cc) != 2 {
		return false
	}
	for _, r := range cc {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// --- address ---

func (s *Service) GetAddress(ctx context.Context, userID uuid.UUID) (Address, error) {
	row, err := s.repo.GetAddress(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Address{}, apperrors.NewNotFound("address")
		}
		return Address{}, err
	}
	return addressFromGet(row), nil
}

func (s *Service) UpsertAddress(ctx context.Context, actorID, userID uuid.UUID, req addressRequest) (Address, error) {
	cc := strings.ToUpper(strings.TrimSpace(req.CountryCode))
	if !validCountryCode(cc) {
		return Address{}, apperrors.NewValidation("country_code must be a 2-letter ISO code")
	}
	row, err := s.repo.UpsertAddress(ctx, db.UpsertUserAddressParams{
		UserID:      userID,
		Company:     textFromPtr(req.Company),
		CareOf:      textFromPtr(req.CareOf),
		Street1:     req.Street1,
		Street2:     textFromPtr(req.Street2),
		PostalCode:  req.PostalCode,
		City:        req.City,
		State:       textFromPtr(req.State),
		CountryCode: cc,
		Note:        textFromPtr(req.Note),
		Actor:       actorNull(actorID),
	})
	if err != nil {
		return Address{}, err
	}
	return addressFromUpsert(row), nil
}

// --- preferences ---

func (s *Service) GetPreferences(ctx context.Context, userID uuid.UUID) (Preferences, error) {
	row, err := s.repo.GetPreferences(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Preferences{}, apperrors.NewNotFound("preferences")
		}
		return Preferences{}, err
	}
	return preferencesFromGet(row), nil
}

func (s *Service) UpsertPreferences(ctx context.Context, actorID, userID uuid.UUID, req preferencesRequest) (Preferences, error) {
	if req.PreferredContactType != nil && *req.PreferredContactType != "" && !validPreferredContactTypes[*req.PreferredContactType] {
		return Preferences{}, apperrors.NewValidation("invalid preferred_contact_type")
	}
	row, err := s.repo.UpsertPreferences(ctx, db.UpsertUserPreferencesParams{
		UserID:               userID,
		PreferredContactType: textFromPtr(req.PreferredContactType),
		Note:                 textFromPtr(req.Note),
		Actor:                actorNull(actorID),
	})
	if err != nil {
		return Preferences{}, err
	}
	return preferencesFromUpsert(row), nil
}

// --- contacts ---

var validContactTypes = map[string]bool{
	"email":   true,
	"phone":   true,
	"mobile":  true,
	"fax":     true,
	"website": true,
	"other":   true,
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func (s *Service) ListContacts(ctx context.Context, userID uuid.UUID) ([]Contact, error) {
	rows, err := s.repo.ListContacts(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Contact, len(rows))
	for i, r := range rows {
		out[i] = contactFromList(r)
	}
	return out, nil
}

func (s *Service) CreateContact(ctx context.Context, actorID, userID uuid.UUID, req createContactRequest) (Contact, error) {
	if !validContactTypes[req.ContactType] {
		return Contact{}, apperrors.NewValidation("invalid contact_type")
	}
	row, err := s.repo.CreateContact(ctx, actorID, req.IsPrimary, db.CreateContactParams{
		UserID:              userID,
		ContactType:         req.ContactType,
		Label:               textFromPtr(req.Label),
		Value:               req.Value,
		IsPrimary:           req.IsPrimary,
		IsPreferred:         req.IsPreferred,
		AllowContact:        boolOr(req.AllowContact, true),
		PreferredTimeWindow: textFromPtr(req.PreferredTimeWindow),
		Note:                textFromPtr(req.Note),
		Actor:               actorNull(actorID),
	})
	if err != nil {
		return Contact{}, err
	}
	return contactFromCreate(row), nil
}

func (s *Service) UpdateContact(ctx context.Context, actorID, userID, contactID uuid.UUID, req updateContactRequest) (Contact, error) {
	existing, err := s.repo.GetContact(ctx, contactID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Contact{}, apperrors.NewNotFound("contact")
		}
		return Contact{}, err
	}
	if existing.UserID != userID {
		return Contact{}, apperrors.NewNotFound("contact")
	}
	row, err := s.repo.UpdateContact(ctx, actorID, existing.UserID, existing.ContactType, req.IsPrimary, db.UpdateContactParams{
		ID:                  contactID,
		Label:               textFromPtr(req.Label),
		Value:               req.Value,
		IsPrimary:           req.IsPrimary,
		IsPreferred:         req.IsPreferred,
		AllowContact:        boolOr(req.AllowContact, existing.AllowContact),
		PreferredTimeWindow: textFromPtr(req.PreferredTimeWindow),
		Note:                textFromPtr(req.Note),
		UpdatedBy:           actorNull(actorID),
	})
	if err != nil {
		return Contact{}, err
	}
	return contactFromUpdate(row), nil
}

func (s *Service) DeleteContact(ctx context.Context, actorID, userID, contactID uuid.UUID) error {
	existing, err := s.repo.GetContact(ctx, contactID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NewNotFound("contact")
		}
		return err
	}
	if existing.UserID != userID {
		return apperrors.NewNotFound("contact")
	}
	return s.repo.SoftDeleteContact(ctx, actorID, db.SoftDeleteContactParams{ID: contactID, UpdatedBy: actorNull(actorID)})
}
