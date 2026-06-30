package groups

import (
	"net/http"
	"strconv"

	apperrors "github.com/derpixler/skolva-core/errors"
	"github.com/derpixler/skolva-core/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler exposes the groups HTTP endpoints.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes wires the group endpoints onto the given /api group.
func RegisterRoutes(rg *gin.RouterGroup, pool *pgxpool.Pool) {
	h := NewHandler(NewService(NewRepository(pool)))
	rg.GET("/groups", middleware.RequirePermission("groups.read"), h.List)
	rg.POST("/groups", middleware.RequirePermission("groups.write"), h.Create)
	rg.GET("/groups/:id", middleware.RequirePermission("groups.read"), h.Get)
	rg.PATCH("/groups/:id", middleware.RequirePermission("groups.write"), h.Update)
	rg.DELETE("/groups/:id", middleware.RequirePermission("groups.write"), h.Delete)
	rg.GET("/groups/:id/members", middleware.RequirePermission("groups.read"), h.ListMembers)
	rg.POST("/groups/:id/members", middleware.RequirePermission("groups.write"), h.AddMember)
	rg.DELETE("/groups/:id/members/:user_id", middleware.RequirePermission("groups.write"), h.RemoveMember)
	rg.GET("/users/:id/groups", middleware.RequirePermission("groups.read"), h.ListUserGroups)
}

func (h *Handler) List(c *gin.Context) {
	limit, offset := pagination(c)
	gs, err := h.svc.List(c.Request.Context(), c.Query("group_type"), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gs)
}

func (h *Handler) Create(c *gin.Context) {
	var req createGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("name is required"))
		return
	}
	g, err := h.svc.Create(c.Request.Context(), actorID(c), req.Name, req.Description, req.GroupType)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, g)
}

func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid group id"))
		return
	}
	g, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, g)
}

func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid group id"))
		return
	}
	var req updateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("name and group_type are required"))
		return
	}
	g, err := h.svc.Update(c.Request.Context(), actorID(c), id, req.Name, req.Description, req.GroupType, req.IsActive)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, g)
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid group id"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), actorID(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListMembers(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid group id"))
		return
	}
	members, err := h.svc.ListMembers(c.Request.Context(), groupID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, members)
}

func (h *Handler) AddMember(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid group id"))
		return
	}
	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("user_id is required"))
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user_id"))
		return
	}
	members, err := h.svc.AddMember(c.Request.Context(), actorID(c), groupID, userID, req.RoleInGroup)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, members)
}

func (h *Handler) RemoveMember(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid group id"))
		return
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	if err := h.svc.RemoveMember(c.Request.Context(), groupID, userID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListUserGroups(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	gs, err := h.svc.ListUserGroups(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gs)
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

func respondError(c *gin.Context, err error) {
	appErr := apperrors.FromError(err)
	c.JSON(appErr.HTTPStatus, appErr)
}
