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
	if err := httpserver.Bind(c, &req); err != nil {
		return err
	}
	resp, err := h.svc.Register(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) login(c echo.Context) error {
	var req LoginRequest
	if err := httpserver.Bind(c, &req); err != nil {
		return err
	}
	resp, err := h.svc.Login(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) forgot(c echo.Context) error {
	var req ForgotPasswordRequest
	if err := httpserver.Bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.RequestPasswordReset(c.Request().Context(), req.Email); err != nil {
		return err
	}
	return c.NoContent(http.StatusAccepted)
}

func (h *Handler) reset(c echo.Context) error {
	var req ResetPasswordRequest
	if err := httpserver.Bind(c, &req); err != nil {
		return err
	}
	if err := h.svc.ResetPassword(c.Request().Context(), req); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
