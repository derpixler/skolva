package auth

import (
	"net/http"

	apperrors "github.com/derpixler/skolva/internal/core/errors"
	"github.com/derpixler/skolva/internal/core/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler exposes the identity module's HTTP endpoints.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes wires the identity endpoints onto the given /api group,
// constructing the repository/service from the pool.
func RegisterRoutes(rg *gin.RouterGroup, pool *pgxpool.Pool) {
	h := NewHandler(NewService(NewRepository(pool)))
	users := rg.Group("/users")
	users.GET("/:id/roles", middleware.RequirePermission("users.read"), h.ListUserRoles)
	users.POST("/:id/roles", middleware.RequirePermission("users.write"), h.AssignRole)
	users.DELETE("/:id/roles/:slug", middleware.RequirePermission("users.write"), h.RemoveRole)
}

func (h *Handler) AssignRole(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	var req assignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("role_slug is required"))
		return
	}
	roles, err := h.svc.AssignRole(c.Request.Context(), actorID(c), userID, req.RoleSlug)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toRoleResponses(roles))
}

func (h *Handler) RemoveRole(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	if err := h.svc.RemoveRole(c.Request.Context(), userID, c.Param("slug")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListUserRoles(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	roles, err := h.svc.ListUserRoles(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toRoleResponses(roles))
}

// actorID extracts the authenticated user's UUID from the request context.
func actorID(c *gin.Context) uuid.UUID {
	actor := middleware.GetActor(c)
	if actor == nil {
		return uuid.Nil
	}
	id, err := uuid.Parse(actor.UserID)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// respondError maps any error to its AppError JSON representation.
func respondError(c *gin.Context, err error) {
	appErr := apperrors.FromError(err)
	c.JSON(appErr.HTTPStatus, appErr)
}
