package auth

import (
	"net/http"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/gin-gonic/gin"
)

// Handler expone las rutas públicas de autenticación.
type Handler struct {
	svc *Service
}

// NewHandler crea el controlador de autenticación.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register monta las rutas bajo el grupo público /api/v1.
func (h *Handler) Register(g *gin.RouterGroup) {
	a := g.Group("/auth")
	a.POST("/register", h.register)
	a.POST("/login", h.login)
	a.POST("/password/forgot", h.forgot)
	a.POST("/password/reset", h.reset)
}

func (h *Handler) register(c *gin.Context) {
	var req RegisterRequest
	if !httpserver.Bind(c, &req) {
		return
	}
	resp, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) login(c *gin.Context) {
	var req LoginRequest
	if !httpserver.Bind(c, &req) {
		return
	}
	resp, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) forgot(c *gin.Context) {
	var req ForgotPasswordRequest
	if !httpserver.Bind(c, &req) {
		return
	}
	if err := h.svc.RequestPasswordReset(c.Request.Context(), req.Email); err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (h *Handler) reset(c *gin.Context) {
	var req ResetPasswordRequest
	if !httpserver.Bind(c, &req) {
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), req); err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
