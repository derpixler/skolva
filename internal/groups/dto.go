package groups

import (
	"time"

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
