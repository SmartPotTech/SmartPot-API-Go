package virtualdevices

import (
	"net/http"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/security"
	"github.com/gin-gonic/gin"
)

// Handler expone las rutas de los cultivos virtuales.
type Handler struct {
	svc *Service
}

// NewHandler crea el controlador de cultivos virtuales.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register monta las rutas bajo el grupo autenticado /api/v1.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/crops/:cropId/virtual-device", h.get)
	g.PUT("/crops/:cropId/virtual-device", h.configure)
	g.DELETE("/crops/:cropId/virtual-device", h.pause)
	g.GET("/virtual-devices/places", h.places)
}

func (h *Handler) get(c *gin.Context) {
	res, err := h.svc.Get(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"))
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) configure(c *gin.Context) {
	var req VirtualDeviceRequest
	if !httpserver.Bind(c, &req) {
		return
	}
	res, err := h.svc.Configure(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"), req)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) pause(c *gin.Context) {
	if err := h.svc.Pause(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId")); err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) places(c *gin.Context) {
	query, ok := c.GetQuery("q")
	if !ok {
		httpserver.RespondError(c, apperr.BadRequest("Parámetro inválido o faltante"))
		return
	}
	places, err := h.svc.Places(c.Request.Context(), query)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, places)
}
