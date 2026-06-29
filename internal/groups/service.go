package groups

import (
	"context"
	"errors"

	apperrors "github.com/derpixler/skolva/internal/core/errors"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var validGroupTypes = map[string]bool{
	"mannschaft":    true,
	"abteilung":     true,
	"arbeitsgruppe": true,
	"vorstand":      true,
	"ausschuss":     true,
	"sonstige":      true,
}

// Service holds the business logic for the groups module.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func actorNull(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: id, Valid: id != uuid.Nil}
}

func (s *Service) Create(ctx context.Context, actorID uuid.UUID, name string, description *string, groupType string) (Group, error) {
	if groupType == "" {
		groupType = "sonstige"
	}
	if !validGroupTypes[groupType] {
		return Group{}, apperrors.NewValidation("invalid group_type")
	}
	row, err := s.repo.Create(ctx, actorID, db.CreateGroupParams{
		Name:        name,
		Description: textFromPtr(description),
		GroupType:   groupType,
		Actor:       actorNull(actorID),
	})
	if err != nil {
		return Group{}, err
	}
	return groupFrom(row.ID, row.Name, row.Description, row.GroupType, row.IsActive, row.CreatedAt, row.UpdatedAt), nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Group, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Group{}, apperrors.NewNotFound("group")
		}
		return Group{}, err
	}
	return groupFrom(row.ID, row.Name, row.Description, row.GroupType, row.IsActive, row.CreatedAt, row.UpdatedAt), nil
}

func (s *Service) List(ctx context.Context, groupType string, limit, offset int32) ([]Group, error) {
	if groupType != "" {
		if !validGroupTypes[groupType] {
			return nil, apperrors.NewValidation("invalid group_type")
		}
		rows, err := s.repo.ListByType(ctx, groupType, limit, offset)
		if err != nil {
			return nil, err
		}
		out := make([]Group, len(rows))
		for i, r := range rows {
			out[i] = groupFrom(r.ID, r.Name, r.Description, r.GroupType, r.IsActive, r.CreatedAt, r.UpdatedAt)
		}
		return out, nil
	}
	rows, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]Group, len(rows))
	for i, r := range rows {
		out[i] = groupFrom(r.ID, r.Name, r.Description, r.GroupType, r.IsActive, r.CreatedAt, r.UpdatedAt)
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, actorID, id uuid.UUID, name string, description *string, groupType string, isActive *bool) (Group, error) {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Group{}, apperrors.NewNotFound("group")
		}
		return Group{}, err
	}
	if !validGroupTypes[groupType] {
		return Group{}, apperrors.NewValidation("invalid group_type")
	}
	active := current.IsActive
	if isActive != nil {
		active = *isActive
	}
	row, err := s.repo.Update(ctx, actorID, db.UpdateGroupParams{
		ID:          id,
		Name:        name,
		Description: textFromPtr(description),
		GroupType:   groupType,
		IsActive:    active,
		UpdatedBy:   actorNull(actorID),
	})
	if err != nil {
		return Group{}, err
	}
	return groupFrom(row.ID, row.Name, row.Description, row.GroupType, row.IsActive, row.CreatedAt, row.UpdatedAt), nil
}

func (s *Service) Delete(ctx context.Context, actorID, id uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NewNotFound("group")
		}
		return err
	}
	return s.repo.SoftDelete(ctx, actorID, db.SoftDeleteGroupParams{ID: id, UpdatedBy: actorNull(actorID)})
}

// --- members ---

var validMemberRoles = map[string]bool{
	"leiter":         true,
	"stellvertreter": true,
	"trainer":        true,
	"mitglied":       true,
}

func (s *Service) ensureGroup(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NewNotFound("group")
		}
		return err
	}
	return nil
}

func (s *Service) members(ctx context.Context, groupID uuid.UUID) ([]Member, error) {
	rows, err := s.repo.ListMembers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	out := make([]Member, len(rows))
	for i, r := range rows {
		out[i] = memberFrom(r)
	}
	return out, nil
}

// AddMember adds (or, on conflict, re-roles) a user in the group and returns
// the updated member list. roleInGroup defaults to "mitglied".
func (s *Service) AddMember(ctx context.Context, actorID, groupID, userID uuid.UUID, roleInGroup string) ([]Member, error) {
	if err := s.ensureGroup(ctx, groupID); err != nil {
		return nil, err
	}
	if roleInGroup == "" {
		roleInGroup = "mitglied"
	}
	if !validMemberRoles[roleInGroup] {
		return nil, apperrors.NewValidation("invalid role_in_group")
	}
	if err := s.repo.AddMember(ctx, db.AddMemberParams{
		GroupID:     groupID,
		UserID:      userID,
		RoleInGroup: roleInGroup,
		CreatedBy:   actorNull(actorID),
	}); err != nil {
		return nil, err
	}
	return s.members(ctx, groupID)
}

func (s *Service) ListMembers(ctx context.Context, groupID uuid.UUID) ([]Member, error) {
	if err := s.ensureGroup(ctx, groupID); err != nil {
		return nil, err
	}
	return s.members(ctx, groupID)
}

func (s *Service) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	if err := s.ensureGroup(ctx, groupID); err != nil {
		return err
	}
	return s.repo.RemoveMember(ctx, groupID, userID)
}

func (s *Service) ListUserGroups(ctx context.Context, userID uuid.UUID) ([]UserGroup, error) {
	rows, err := s.repo.ListUserGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]UserGroup, len(rows))
	for i, r := range rows {
		out[i] = userGroupFrom(r)
	}
	return out, nil
}
