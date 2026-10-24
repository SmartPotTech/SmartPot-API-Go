package actuators

import (
	"net/http"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/security"
	"github.com/gin-gonic/gin"
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
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/crops/:cropId/actuators", h.list)
	g.POST("/crops/:cropId/actuators", h.add)
	g.DELETE("/crops/:cropId/actuators/:actuatorId", h.remove)
}

func (h *Handler) list(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"))
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	now := h.clock()
	out := make([]ActuatorResponse, 0, len(items))
	for i := range items {
		out = append(out, ToResponse(&items[i], now))
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) add(c *gin.Context) {
	var req ActuatorRequest
	if !httpserver.Bind(c, &req) {
		return
	}
	a, err := h.svc.Add(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"), *req.Type)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, ToResponse(a, h.clock()))
}

func (h *Handler) remove(c *gin.Context) {
	if err := h.svc.Remove(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"), c.Param("actuatorId")); err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
