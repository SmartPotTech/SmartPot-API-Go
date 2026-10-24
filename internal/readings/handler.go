package readings

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/security"
	"github.com/gin-gonic/gin"
)

// Handler expone las rutas de lecturas de un cultivo.
type Handler struct {
	svc *Service
}

// NewHandler crea el controlador de lecturas.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register monta las rutas bajo el grupo autenticado /api/v1.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/crops/:cropId/readings", h.list)
	g.GET("/crops/:cropId/readings/latest", h.latest)
	g.GET("/crops/:cropId/readings/summary", h.summary)
	g.GET("/crops/:cropId/readings/export", h.export)
	g.POST("/crops/:cropId/readings", h.create)
}

func (h *Handler) list(c *gin.Context) {
	from, ok := queryTime(c, "from")
	if !ok {
		httpserver.RespondError(c, apperr.BadRequest("Parámetro inválido o faltante"))
		return
	}
	to, ok := queryTime(c, "to")
	if !ok {
		httpserver.RespondError(c, apperr.BadRequest("Parámetro inválido o faltante"))
		return
	}
	limit, ok := queryIntPtr(c, "limit")
	if !ok {
		httpserver.RespondError(c, apperr.BadRequest("Parámetro inválido o faltante"))
		return
	}
	items, err := h.svc.List(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"), from, to, limit)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	out := make([]*ReadingResponse, 0, len(items))
	for i := range items {
		out = append(out, ToResponse(&items[i]))
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) latest(c *gin.Context) {
	reading, err := h.svc.LatestOwned(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"))
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	if reading == nil {
		httpserver.RespondError(c, apperr.NotFound("El cultivo todavía no tiene lecturas"))
		return
	}
	c.JSON(http.StatusOK, ToResponse(reading))
}

func (h *Handler) summary(c *gin.Context) {
	hours := 24
	if raw := c.Query("hours"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			httpserver.RespondError(c, apperr.BadRequest("Parámetro inválido o faltante"))
			return
		}
		hours = n
	}
	res, err := h.svc.Summary(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"), hours)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) export(c *gin.Context) {
	from, ok := queryTime(c, "from")
	if !ok {
		httpserver.RespondError(c, apperr.BadRequest("Parámetro inválido o faltante"))
		return
	}
	to, ok := queryTime(c, "to")
	if !ok {
		httpserver.RespondError(c, apperr.BadRequest("Parámetro inválido o faltante"))
		return
	}
	cropID := c.Param("cropId")
	csv, err := h.svc.ExportCSV(c.Request.Context(), security.CurrentUserID(c), cropID, from, to)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"smartpot-%s.csv\"", cropID))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", []byte(csv))
}

func (h *Handler) create(c *gin.Context) {
	var req MeasuresRequest
	if !httpserver.DecodeJSON(c, &req) {
		return
	}
	reading, err := h.svc.RecordManual(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"), req.ToMeasures())
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, ToResponse(reading))
}

func queryTime(c *gin.Context, key string) (*time.Time, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, false
	}
	return &t, true
}

func queryIntPtr(c *gin.Context, key string) (*int, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil, false
	}
	return &n, true
}
