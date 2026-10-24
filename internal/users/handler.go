package users

import (
	"net/http"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/security"
	"github.com/gin-gonic/gin"
)

// Handler expone las rutas del perfil de la cuenta.
type Handler struct {
	svc *Service
}

// NewHandler crea el controlador de usuarios.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register monta las rutas bajo el grupo autenticado /api/v1.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/users/me", h.me)
	g.PUT("/users/me", h.update)
	g.PUT("/users/me/password", h.changePassword)
	g.DELETE("/users/me", h.delete)
}

func (h *Handler) me(c *gin.Context) {
	u, err := h.svc.GetByID(c.Request.Context(), security.CurrentUserID(c))
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, ToResponse(u))
}

func (h *Handler) update(c *gin.Context) {
	var req UpdateProfileRequest
	if !httpserver.Bind(c, &req) {
		return
	}
	u, err := h.svc.UpdateProfile(c.Request.Context(), security.CurrentUserID(c), req)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, ToResponse(u))
}

func (h *Handler) changePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if !httpserver.Bind(c, &req) {
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), security.CurrentUserID(c), req); err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) delete(c *gin.Context) {
	if err := h.svc.DeleteAccount(c.Request.Context(), security.CurrentUserID(c)); err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
