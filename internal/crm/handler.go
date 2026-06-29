package crm

import (
	"net/http"

	apperrors "github.com/derpixler/skolva/internal/core/errors"
	"github.com/derpixler/skolva/internal/core/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler exposes the CRM HTTP endpoints.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes wires the CRM endpoints (address, contacts, preferences)
// onto the given /api group.
func RegisterRoutes(rg *gin.RouterGroup, pool *pgxpool.Pool) {
	h := NewHandler(NewService(NewRepository(pool)))

	rg.GET("/users/:id/address", middleware.RequirePermission("users.read"), h.GetAddress)
	rg.PUT("/users/:id/address", middleware.RequirePermission("users.write"), h.PutAddress)

	rg.GET("/users/:id/preferences", middleware.RequirePermission("users.read"), h.GetPreferences)
	rg.PUT("/users/:id/preferences", middleware.RequirePermission("users.write"), h.PutPreferences)

	rg.GET("/users/:id/contacts", middleware.RequirePermission("users.read"), h.ListContacts)
	rg.POST("/users/:id/contacts", middleware.RequirePermission("users.write"), h.CreateContact)
	rg.PATCH("/users/:id/contacts/:cid", middleware.RequirePermission("users.write"), h.UpdateContact)
	rg.DELETE("/users/:id/contacts/:cid", middleware.RequirePermission("users.write"), h.DeleteContact)
}

func (h *Handler) GetAddress(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	addr, err := h.svc.GetAddress(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, addr)
}

func (h *Handler) PutAddress(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	var req addressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("street1, postal_code, city and country_code are required"))
		return
	}
	addr, err := h.svc.UpsertAddress(c.Request.Context(), actorID(c), userID, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, addr)
}

func (h *Handler) GetPreferences(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	prefs, err := h.svc.GetPreferences(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, prefs)
}

func (h *Handler) PutPreferences(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	var req preferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("invalid request body"))
		return
	}
	prefs, err := h.svc.UpsertPreferences(c.Request.Context(), actorID(c), userID, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, prefs)
}

func (h *Handler) ListContacts(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	contacts, err := h.svc.ListContacts(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, contacts)
}

func (h *Handler) CreateContact(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	var req createContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("contact_type and value are required"))
		return
	}
	contact, err := h.svc.CreateContact(c.Request.Context(), actorID(c), userID, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, contact)
}

func (h *Handler) UpdateContact(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	contactID, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid contact id"))
		return
	}
	var req updateContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("value is required"))
		return
	}
	contact, err := h.svc.UpdateContact(c.Request.Context(), actorID(c), userID, contactID, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, contact)
}

func (h *Handler) DeleteContact(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user id"))
		return
	}
	contactID, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid contact id"))
		return
	}
	if err := h.svc.DeleteContact(c.Request.Context(), actorID(c), userID, contactID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
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
