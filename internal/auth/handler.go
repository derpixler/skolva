package auth

import (
	"net/http"
	"strconv"

	apperrors "github.com/derpixler/skolva/internal/core/errors"
	"github.com/derpixler/skolva/internal/core/mail"
	"github.com/derpixler/skolva/internal/core/middleware"
	"github.com/derpixler/skolva/internal/core/secrets"
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
func RegisterRoutes(rg *gin.RouterGroup, pool *pgxpool.Pool, tm *TokenManager, cipher *secrets.Cipher, mailer mail.Mailer) {
	h := NewHandler(NewService(NewRepository(pool), tm, cipher, mailer))

	rg.POST("/auth/login", h.Login)
	rg.POST("/auth/register", middleware.RequirePermission("users.write"), h.Register)

	rg.POST("/auth/2fa/setup", middleware.RequireAuth(), h.Setup2FA)
	rg.POST("/auth/2fa/confirm", middleware.RequireAuth(), h.Confirm2FA)
	rg.POST("/auth/2fa/verify", h.Verify2FA)
	rg.POST("/auth/2fa/recovery", h.Recover2FA)
	rg.POST("/auth/2fa/disable", middleware.RequireAuth(), h.Disable2FA)

	rg.POST("/auth/2fa/email/setup", middleware.RequireAuth(), h.SetupEmail2FA)
	rg.POST("/auth/2fa/email/confirm", middleware.RequireAuth(), h.ConfirmEmail2FA)
	rg.POST("/auth/2fa/email/verify", h.VerifyEmail2FA)
	rg.POST("/auth/2fa/email/resend", h.ResendEmail2FA)
	rg.POST("/auth/2fa/email/disable", middleware.RequireAuth(), h.DisableEmail2FA)

	rg.POST("/auth/password/forgot", h.ForgotPassword)
	rg.POST("/auth/password/reset", h.ResetPassword)

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
	token, needs2FA, tempToken, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		respondError(c, err)
		return
	}
	if needs2FA {
		c.JSON(http.StatusOK, loginResponse{Requires2FA: true, TempToken: tempToken})
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

func (h *Handler) Setup2FA(c *gin.Context) {
	userID := actorID(c)
	uri, codes, err := h.svc.Setup2FA(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, Setup2FAResponse{ProvisioningURI: uri, RecoveryCodes: codes})
}

func (h *Handler) Confirm2FA(c *gin.Context) {
	var req codeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("code is required"))
		return
	}
	if err := h.svc.Confirm2FA(c.Request.Context(), actorID(c), req.Code); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Verify2FA(c *gin.Context) {
	var req verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("temp_token and code are required"))
		return
	}
	claims, err := h.svc.tm.Verify(req.TempToken)
	if err != nil || claims.Kind != TokenKindPending2FA {
		respondError(c, apperrors.NewUnauthorized("invalid or expired 2FA token"))
		return
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		respondError(c, apperrors.NewUnauthorized("invalid 2FA token"))
		return
	}
	if err := h.svc.Verify2FA(c.Request.Context(), userID, req.Code); err != nil {
		respondError(c, err)
		return
	}
	token, err := h.svc.IssueAccessForUser(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, loginResponse{Token: token})
}

func (h *Handler) Recover2FA(c *gin.Context) {
	var req verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("temp_token and code are required"))
		return
	}
	claims, err := h.svc.tm.Verify(req.TempToken)
	if err != nil || claims.Kind != TokenKindPending2FA {
		respondError(c, apperrors.NewUnauthorized("invalid or expired 2FA token"))
		return
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		respondError(c, apperrors.NewUnauthorized("invalid 2FA token"))
		return
	}
	if err := h.svc.ConsumeRecoveryCode(c.Request.Context(), userID, req.Code); err != nil {
		respondError(c, err)
		return
	}
	token, err := h.svc.IssueAccessForUser(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, loginResponse{Token: token})
}

func (h *Handler) Disable2FA(c *gin.Context) {
	var req codeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("code is required"))
		return
	}
	if err := h.svc.Disable2FA(c.Request.Context(), actorID(c), req.Code); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) SetupEmail2FA(c *gin.Context) {
	if err := h.svc.SetupEmail2FA(c.Request.Context(), actorID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ConfirmEmail2FA(c *gin.Context) {
	var req codeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("code is required"))
		return
	}
	if err := h.svc.ConfirmEmail2FA(c.Request.Context(), actorID(c), req.Code); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) VerifyEmail2FA(c *gin.Context) {
	var req verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("temp_token and code are required"))
		return
	}
	userID, ok := h.pending2FAUser(c, req.TempToken)
	if !ok {
		return
	}
	if err := h.svc.VerifyEmail2FALogin(c.Request.Context(), userID, req.Code); err != nil {
		respondError(c, err)
		return
	}
	token, err := h.svc.IssueAccessForUser(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, loginResponse{Token: token})
}

func (h *Handler) ResendEmail2FA(c *gin.Context) {
	var req tempTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("temp_token is required"))
		return
	}
	userID, ok := h.pending2FAUser(c, req.TempToken)
	if !ok {
		return
	}
	if err := h.svc.SendEmail2FALoginOTP(c.Request.Context(), userID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) DisableEmail2FA(c *gin.Context) {
	if err := h.svc.DisableEmail2FA(c.Request.Context(), actorID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// pending2FAUser validates a 2FA pending token and returns the user it
// identifies. On failure it writes the error response and returns ok=false.
func (h *Handler) pending2FAUser(c *gin.Context, tempToken string) (uuid.UUID, bool) {
	claims, err := h.svc.tm.Verify(tempToken)
	if err != nil || claims.Kind != TokenKindPending2FA {
		respondError(c, apperrors.NewUnauthorized("invalid or expired 2FA token"))
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		respondError(c, apperrors.NewUnauthorized("invalid 2FA token"))
		return uuid.Nil, false
	}
	return userID, true
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req passwordForgotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("email is required"))
		return
	}
	if err := h.svc.ForgotPassword(c.Request.Context(), req.Email); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a reset link has been sent."})
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req passwordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.NewValidation("user_id, token and password (min 8) are required"))
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		respondError(c, apperrors.NewValidation("invalid user_id"))
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), userID, req.Token, req.Password); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
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
