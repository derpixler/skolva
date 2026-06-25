package auth

import (
	"time"

	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type assignRoleRequest struct {
	RoleSlug string `json:"role_slug" binding:"required"`
}

// RoleResponse is the API representation of a role assigned to a user.
type RoleResponse struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	IsProtected bool   `json:"is_protected"`
}

func toRoleResponses(rows []db.ListUserRolesRow) []RoleResponse {
	out := make([]RoleResponse, len(rows))
	for i, r := range rows {
		out[i] = RoleResponse{Slug: r.Slug, DisplayName: r.DisplayName, IsProtected: r.IsProtected}
	}
	return out
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	Token       string `json:"token,omitempty"`
	Requires2FA bool   `json:"requires_2fa,omitempty"`
	TempToken   string `json:"temp_token,omitempty"`
}

type registerRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
}

type updateUserRequest struct {
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	IsActive  *bool  `json:"is_active"`
}

// UserResponse is the API representation of a user.
type UserResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	IsActive    bool      `json:"is_active"`
	IsProtected bool      `json:"is_protected"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func userFrom(id uuid.UUID, email, firstName, lastName string, isActive, isProtected bool, created, updated pgtype.Timestamptz) UserResponse {
	return UserResponse{
		ID:          id,
		Email:       email,
		FirstName:   firstName,
		LastName:    lastName,
		IsActive:    isActive,
		IsProtected: isProtected,
		CreatedAt:   created.Time,
		UpdatedAt:   updated.Time,
	}
}

func userFromCreate(r db.CreateUserRow) UserResponse {
	return userFrom(r.ID, r.Email, r.FirstName, r.LastName, r.IsActive, r.IsProtected, r.CreatedAt, r.UpdatedAt)
}

func userFromGet(r db.GetUserByIDRow) UserResponse {
	return userFrom(r.ID, r.Email, r.FirstName, r.LastName, r.IsActive, r.IsProtected, r.CreatedAt, r.UpdatedAt)
}

func userFromList(r db.ListUsersRow) UserResponse {
	return userFrom(r.ID, r.Email, r.FirstName, r.LastName, r.IsActive, r.IsProtected, r.CreatedAt, r.UpdatedAt)
}

func userFromUpdate(r db.UpdateUserRow) UserResponse {
	return userFrom(r.ID, r.Email, r.FirstName, r.LastName, r.IsActive, r.IsProtected, r.CreatedAt, r.UpdatedAt)
}

func userFromByIDs(r db.GetUsersByIDsRow) UserResponse {
	return userFrom(r.ID, r.Email, r.FirstName, r.LastName, r.IsActive, r.IsProtected, r.CreatedAt, r.UpdatedAt)
}

type Setup2FAResponse struct {
	ProvisioningURI string   `json:"provisioning_uri"`
	RecoveryCodes   []string `json:"recovery_codes"`
}

type verify2FARequest struct {
	TempToken string `json:"temp_token" binding:"required"`
	Code      string `json:"code" binding:"required"`
}

type codeRequest struct {
	Code string `json:"code" binding:"required"`
}
