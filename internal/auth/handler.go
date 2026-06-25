package auth

import (
	"net/http"
	"strconv"

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
func RegisterRoutes(rg *gin.RouterGroup, pool *pgxpool.Pool, tm *TokenManager) {
	h := NewHandler(NewService(NewRepository(pool), tm))

	rg.POST("/auth/login", h.Login)
	rg.POST("/auth/register", middleware.RequirePermission("users.write"), h.Register)

	rg.GET("/users", middleware.RequirePermission("users.read"), h.ListUsers)
	rg.GET("/users/:id", middleware.RequirePermission("users.read"), h.GetUser)
	rg.PATCH("/users/:id", middleware.RequirePermission("users.write"), h.UpdateUser)
	rg.DELETE("/users/:id", middleware.RequirePermission("users.write"), h.DeleteUser)

	rg.GET("/users/:id/roles", middleware.RequirePermission("users.read"), h.ListUserRoles)
	rg.POST("/users/:id/roles", middleware.RequirePermission("users.write"), h.AssignRole)
	rg.DELETE("/users/:id/roles/:slug", middleware.RequirePermission("users.write"), h.RemoveRole)

	rg.GET("/search/users", middleware.RequirePermission("users.read"), h.SearchUsers)
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

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("email and password are required"))
		return
	}
	token, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, loginResponse{Token: token})
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("email, password (min 8), first_name and last_name are required"))
		return
	}
	u, err := h.svc.Register(c.Request.Context(), actorID(c), req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (h *Handler) ListUsers(c *gin.Context) {
	limit, offset := pagination(c)
	users, err := h.svc.ListUsers(c.Request.Context(), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *Handler) GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	u, err := h.svc.GetUser(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *Handler) UpdateUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("first_name and last_name are required"))
		return
	}
	u, err := h.svc.UpdateUser(c.Request.Context(), actorID(c), id, req.FirstName, req.LastName, req.IsActive)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *Handler) DeleteUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	if err := h.svc.DeleteUser(c.Request.Context(), actorID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) SearchUsers(c *gin.Context) {
	limit, _ := pagination(c)
	users, err := h.svc.SearchUsers(c.Request.Context(), c.Query("q"), int(limit))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, users)
}

func pagination(c *gin.Context) (limit, offset int32) {
	limit, offset = 50, 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil && n > 0 && n <= 200 {
			limit = int32(n)
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	return limit, offset
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
