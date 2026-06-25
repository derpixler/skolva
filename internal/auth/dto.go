package auth

import "github.com/derpixler/skolva/internal/db"

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
