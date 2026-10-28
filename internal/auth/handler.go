package auth

import (
	"net/http"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/labstack/echo/v4"
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
func (h *Handler) Register(g *echo.Group) {
	a := g.Group("/auth")
	a.POST("/register", h.register)
	a.POST("/login", h.login)
	a.POST("/password/forgot", h.forgot)
	a.POST("/password/reset", h.reset)
}

func (h *Handler) register(c echo.Context) error {
	var req RegisterRequest
	if !httpserver.Bind(c, &req) {
		return nil
	}
	resp, err := h.svc.Register(c.Request().Context(), req)
	if err != nil {
		httpserver.RespondError(c, err)
		return nil
	}
	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) login(c echo.Context) error {
	var req LoginRequest
	if !httpserver.Bind(c, &req) {
		return nil
	}
	resp, err := h.svc.Login(c.Request().Context(), req)
	if err != nil {
		httpserver.RespondError(c, err)
		return nil
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) forgot(c echo.Context) error {
	var req ForgotPasswordRequest
	if !httpserver.Bind(c, &req) {
		return nil
	}
	if err := h.svc.RequestPasswordReset(c.Request().Context(), req.Email); err != nil {
		httpserver.RespondError(c, err)
		return nil
	}
	return c.NoContent(http.StatusAccepted)
}

func (h *Handler) reset(c echo.Context) error {
	var req ResetPasswordRequest
	if !httpserver.Bind(c, &req) {
		return nil
	}
	if err := h.svc.ResetPassword(c.Request().Context(), req); err != nil {
		httpserver.RespondError(c, err)
		return nil
	}
	return c.NoContent(http.StatusNoContent)
}
