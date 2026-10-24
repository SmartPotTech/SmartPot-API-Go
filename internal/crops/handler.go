package crops

import (
	"context"
	"net/http"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/geo"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/readings"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/security"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/virtualdevices"
	"github.com/gin-gonic/gin"
)

// VirtualDeviceManager arranca y valida la simulación de los cultivos virtuales
// (lo implementa el paquete virtualdevices).
type VirtualDeviceManager interface {
	CheckCanCreate(ctx context.Context, ownerID string, req *virtualdevices.VirtualDeviceRequest, placement *geo.Placement) error
	StartFor(ctx context.Context, cropID string, req *virtualdevices.VirtualDeviceRequest) error
}

// Handler expone las rutas de cultivos.
type Handler struct {
	crops    *Service
	readings *readings.Service
	virtual  VirtualDeviceManager
	weather  *WeatherService
}

// NewHandler crea el controlador de cultivos.
func NewHandler(crops *Service, readingsSvc *readings.Service, virtual VirtualDeviceManager, weather *WeatherService) *Handler {
	return &Handler{crops: crops, readings: readingsSvc, virtual: virtual, weather: weather}
}

// Register monta las rutas bajo el grupo autenticado /api/v1.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/crops", h.list)
	g.POST("/crops", h.create)
	g.PUT("/crops/automation", h.bulkAutomation)
	g.GET("/crops/:cropId", h.get)
	g.PUT("/crops/:cropId", h.update)
	g.DELETE("/crops/:cropId", h.delete)
	g.PUT("/crops/:cropId/automation", h.automation)
	g.GET("/crops/:cropId/weather", h.weatherOf)
	g.GET("/crops/:cropId/device", h.device)
	g.POST("/crops/:cropId/device/key", h.rotateKey)
}

func (h *Handler) list(c *gin.Context) {
	crops, err := h.crops.List(c.Request.Context(), security.CurrentUserID(c))
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	out := make([]CropResponse, 0, len(crops))
	for i := range crops {
		out = append(out, h.toResponse(c.Request.Context(), &crops[i]))
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) create(c *gin.Context) {
	var req CropRequest
	if !httpserver.Bind(c, &req) {
		return
	}
	ownerID := security.CurrentUserID(c)
	virtual := req.Kind != nil && *req.Kind == KindVirtual
	if virtual {
		if err := h.virtual.CheckCanCreate(c.Request.Context(), ownerID, req.Virtual, req.Placement.ToPlacement()); err != nil {
			httpserver.RespondError(c, err)
			return
		}
	} else if req.Virtual != nil {
		httpserver.RespondError(c, apperr.BadRequest("Solo los cultivos virtuales tienen simulación"))
		return
	}
	created, err := h.crops.Create(c.Request.Context(), ownerID, req)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	if virtual {
		if err := h.virtual.StartFor(c.Request.Context(), created.Crop.HexID(), req.Virtual); err != nil {
			httpserver.RespondError(c, err)
			return
		}
	}
	c.JSON(http.StatusCreated, CropCreatedResponse{Crop: ToResponse(created.Crop, nil), Device: created.Credentials})
}

func (h *Handler) get(c *gin.Context) {
	crop, err := h.crops.GetOwned(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"))
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.toResponse(c.Request.Context(), crop))
}

func (h *Handler) update(c *gin.Context) {
	var req CropRequest
	if !httpserver.Bind(c, &req) {
		return
	}
	crop, err := h.crops.Update(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"), req)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.toResponse(c.Request.Context(), crop))
}

func (h *Handler) delete(c *gin.Context) {
	if err := h.crops.Delete(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId")); err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) bulkAutomation(c *gin.Context) {
	var req BulkAutomationRequest
	if !httpserver.Bind(c, &req) {
		return
	}
	crops, err := h.crops.SetAutomationBulk(c.Request.Context(), security.CurrentUserID(c), req.CropIds, *req.Enabled)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	out := make([]CropResponse, 0, len(crops))
	for i := range crops {
		out = append(out, h.toResponse(c.Request.Context(), &crops[i]))
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) automation(c *gin.Context) {
	var req AutomationRequest
	if !httpserver.Bind(c, &req) {
		return
	}
	crop, err := h.crops.SetAutomation(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"), *req.Enabled)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.toResponse(c.Request.Context(), crop))
}

func (h *Handler) weatherOf(c *gin.Context) {
	crop, err := h.crops.GetOwned(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"))
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	weather, ok := h.weather.Current(c.Request.Context(), crop)
	if !ok {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, weather)
}

func (h *Handler) device(c *gin.Context) {
	creds, err := h.crops.DeviceInfo(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"))
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, creds)
}

func (h *Handler) rotateKey(c *gin.Context) {
	creds, err := h.crops.RotateDeviceKey(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"))
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, creds)
}

func (h *Handler) toResponse(ctx context.Context, crop *Crop) CropResponse {
	latest, _ := h.readings.Latest(ctx, crop.HexID())
	return ToResponse(crop, latest)
}
