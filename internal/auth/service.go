package auth

import (
	"context"
	"errors"

	apperrors "github.com/derpixler/skolva/internal/core/errors"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Service holds the business logic for the identity module.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ensureUser(ctx context.Context, userID uuid.UUID) error {
	if _, err := s.repo.GetUserByID(ctx, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NewNotFound("user")
		}
		return err
	}
	return nil
}

// AssignRole assigns roleSlug to userID and returns the user's updated roles.
func (s *Service) AssignRole(ctx context.Context, actorID, userID uuid.UUID, roleSlug string) ([]db.ListUserRolesRow, error) {
	if err := s.ensureUser(ctx, userID); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetRole(ctx, roleSlug); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewNotFound("role")
		}
		return nil, err
	}
	assignedBy := uuid.NullUUID{UUID: actorID, Valid: actorID != uuid.Nil}
	if err := s.repo.AssignRole(ctx, userID, roleSlug, assignedBy); err != nil {
		return nil, err
	}
	return s.repo.ListUserRoles(ctx, userID)
}

// RemoveRole removes roleSlug from userID (idempotent).
func (s *Service) RemoveRole(ctx context.Context, userID uuid.UUID, roleSlug string) error {
	if err := s.ensureUser(ctx, userID); err != nil {
		return err
	}
	return s.repo.RemoveRole(ctx, userID, roleSlug)
}

// ListUserRoles returns the roles assigned to userID.
func (s *Service) ListUserRoles(ctx context.Context, userID uuid.UUID) ([]db.ListUserRolesRow, error) {
	if err := s.ensureUser(ctx, userID); err != nil {
		return nil, err
	}
	return s.repo.ListUserRoles(ctx, userID)
}

const adminRole = "admin"

// CheckPermission reports whether the user may perform the action identified by
// permissionSlug. The admin role grants every permission implicitly; otherwise
// the permission is resolved via the role_permissions join.
func (s *Service) CheckPermission(ctx context.Context, userID uuid.UUID, permissionSlug string) (bool, error) {
	isAdmin, err := s.repo.UserHasRole(ctx, userID, adminRole)
	if err != nil {
		return false, err
	}
	if isAdmin {
		return true, nil
	}
	return s.repo.UserHasPermission(ctx, userID, permissionSlug)
}
