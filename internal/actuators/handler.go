package actuators

import (
	"net/http"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/security"
	"github.com/labstack/echo/v4"
)

// Handler expone las rutas de actuadores de un cultivo.
type Handler struct {
	svc   *Service
	clock func() time.Time
}

// NewHandler crea el controlador de actuadores.
func NewHandler(svc *Service, clock func() time.Time) *Handler {
	return &Handler{svc: svc, clock: clock}
}

// Register monta las rutas bajo el grupo autenticado /api/v1.
func (h *Handler) Register(g *echo.Group) {
	g.GET("/crops/:cropId/actuators", h.list)
	g.POST("/crops/:cropId/actuators", h.add)
	g.DELETE("/crops/:cropId/actuators/:actuatorId", h.remove)
}

func (h *Handler) list(c echo.Context) error {
	items, err := h.svc.List(c.Request().Context(), security.CurrentUserID(c), c.Param("cropId"))
	if err != nil {
		return err
	}
	now := h.clock()
	out := make([]ActuatorResponse, 0, len(items))
	for i := range items {
		out = append(out, ToResponse(&items[i], now))
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) add(c echo.Context) error {
	var req ActuatorRequest
	if err := httpserver.Bind(c, &req); err != nil {
		return err
	}
	a, err := h.svc.Add(c.Request().Context(), security.CurrentUserID(c), c.Param("cropId"), *req.Type)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, ToResponse(a, h.clock()))
}

func (h *Handler) remove(c echo.Context) error {
	if err := h.svc.Remove(c.Request().Context(), security.CurrentUserID(c), c.Param("cropId"), c.Param("actuatorId")); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
