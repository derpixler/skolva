package groups

import (
	"time"

	"github.com/derpixler/skolva/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type createGroupRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	GroupType   string  `json:"group_type"`
}

type updateGroupRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	GroupType   string  `json:"group_type" binding:"required"`
	IsActive    *bool   `json:"is_active"`
}

// Group is the API representation of a group.
type Group struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	GroupType   string    `json:"group_type"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func groupFrom(id uuid.UUID, name string, desc pgtype.Text, groupType string, isActive bool, created, updated pgtype.Timestamptz) Group {
	g := Group{
		ID:        id,
		Name:      name,
		GroupType: groupType,
		IsActive:  isActive,
		CreatedAt: created.Time,
		UpdatedAt: updated.Time,
	}
	if desc.Valid {
		d := desc.String
		g.Description = &d
	}
	return g
}

func textFromPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

type addMemberRequest struct {
	UserID      string `json:"user_id" binding:"required"`
	RoleInGroup string `json:"role_in_group"`
}

// Member is the API representation of a group membership.
type Member struct {
	UserID      uuid.UUID `json:"user_id"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	Email       string    `json:"email"`
	RoleInGroup string    `json:"role_in_group"`
	JoinedAt    time.Time `json:"joined_at"`
}

// UserGroup is the API representation of a group a user belongs to.
type UserGroup struct {
	GroupID     uuid.UUID `json:"group_id"`
	Name        string    `json:"name"`
	GroupType   string    `json:"group_type"`
	RoleInGroup string    `json:"role_in_group"`
	JoinedAt    time.Time `json:"joined_at"`
}

func memberFrom(r db.ListMembersRow) Member {
	return Member{
		UserID:      r.UserID,
		FirstName:   r.FirstName,
		LastName:    r.LastName,
		Email:       r.Email,
		RoleInGroup: r.RoleInGroup,
		JoinedAt:    r.JoinedAt.Time,
	}
}

func userGroupFrom(r db.ListUserGroupsRow) UserGroup {
	return UserGroup{
		GroupID:     r.ID,
		Name:        r.Name,
		GroupType:   r.GroupType,
		RoleInGroup: r.RoleInGroup,
		JoinedAt:    r.JoinedAt.Time,
	}
}
