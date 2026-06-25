package auth

import (
	"context"
	"errors"

	apperrors "github.com/derpixler/skolva/internal/core/errors"
	"github.com/derpixler/skolva/internal/core/secrets"
	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Service holds the business logic for the identity module.
type Service struct {
	repo   *Repository
	tm     *TokenManager
	cipher *secrets.Cipher
}

func NewService(repo *Repository, tm *TokenManager, cipher *secrets.Cipher) *Service {
	return &Service{repo: repo, tm: tm, cipher: cipher}
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

// Login verifies credentials and returns a signed access token carrying the
// user's roles and resolved permissions. Invalid email or password yields the
// same unauthorized error (no user enumeration).
func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperrors.NewUnauthorized("invalid credentials")
		}
		return "", err
	}
	if !u.IsActive {
		return "", apperrors.NewUnauthorized("account is disabled")
	}
	if !VerifyPassword(u.PasswordHash, password) {
		return "", apperrors.NewUnauthorized("invalid credentials")
	}
	roleRows, err := s.repo.ListUserRoles(ctx, u.ID)
	if err != nil {
		return "", err
	}
	roles := make([]string, len(roleRows))
	for i, r := range roleRows {
		roles[i] = r.Slug
	}
	perms, err := s.repo.GetPermissionsForUser(ctx, u.ID)
	if err != nil {
		return "", err
	}
	return s.tm.IssueAccess(u.ID.String(), u.Email, roles, perms)
}

// --- users ---

func actorNull(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: id, Valid: id != uuid.Nil}
}

// Register creates a new user account (the caller's permission gates this).
func (s *Service) Register(ctx context.Context, actorID uuid.UUID, email, password, firstName, lastName string) (UserResponse, error) {
	exists, err := s.repo.UserExistsByEmail(ctx, email)
	if err != nil {
		return UserResponse{}, err
	}
	if exists {
		return UserResponse{}, apperrors.NewConflict("a user with this email already exists")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return UserResponse{}, apperrors.NewValidation(err.Error())
	}
	row, err := s.repo.CreateUser(ctx, actorID, db.CreateUserParams{
		Email:        email,
		PasswordHash: hash,
		FirstName:    firstName,
		LastName:     lastName,
		Actor:        actorNull(actorID),
	})
	if err != nil {
		return UserResponse{}, err
	}
	return userFromCreate(row), nil
}

func (s *Service) ListUsers(ctx context.Context, limit, offset int32) ([]UserResponse, error) {
	rows, err := s.repo.ListUsers(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]UserResponse, len(rows))
	for i, r := range rows {
		out[i] = userFromList(r)
	}
	return out, nil
}

func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (UserResponse, error) {
	row, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserResponse{}, apperrors.NewNotFound("user")
		}
		return UserResponse{}, err
	}
	return userFromGet(row), nil
}

func (s *Service) UpdateUser(ctx context.Context, actorID, id uuid.UUID, firstName, lastName string, isActive *bool) (UserResponse, error) {
	current, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserResponse{}, apperrors.NewNotFound("user")
		}
		return UserResponse{}, err
	}
	active := current.IsActive
	if isActive != nil {
		active = *isActive
	}
	row, err := s.repo.UpdateUser(ctx, actorID, db.UpdateUserParams{
		ID:        id,
		FirstName: firstName,
		LastName:  lastName,
		IsActive:  active,
		UpdatedBy: actorNull(actorID),
	})
	if err != nil {
		return UserResponse{}, err
	}
	return userFromUpdate(row), nil
}

func (s *Service) DeleteUser(ctx context.Context, actorID, id uuid.UUID) error {
	if _, err := s.repo.GetUserByID(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NewNotFound("user")
		}
		return err
	}
	return s.repo.SoftDeleteUser(ctx, actorID, db.SoftDeleteUserParams{
		ID:        id,
		UpdatedBy: actorNull(actorID),
	})
}

func (s *Service) SearchUsers(ctx context.Context, q string, limit int) ([]UserResponse, error) {
	rows, err := s.repo.SearchUsers(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]UserResponse, len(rows))
	for i, r := range rows {
		out[i] = userFromByIDs(r)
	}
	return out, nil
}
