package ai

import (
	"net/http"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/security"
	"github.com/gin-gonic/gin"
)

// Handler expone las rutas del asistente de IA.
type Handler struct {
	insights *InsightService
	client   *Client
}

// NewHandler crea el controlador de IA.
func NewHandler(insights *InsightService, client *Client) *Handler {
	return &Handler{insights: insights, client: client}
}

// RegisterAuthed monta las rutas autenticadas bajo /api/v1.
func (h *Handler) RegisterAuthed(g *gin.RouterGroup) {
	g.GET("/crops/:cropId/insights", h.insightsOf)
	g.GET("/ai/learning", h.learning)
}

// RegisterPublic monta las rutas públicas bajo /api/v1.
func (h *Handler) RegisterPublic(g *gin.RouterGroup) {
	g.GET("/crop-profiles", h.cropProfiles)
}

func (h *Handler) insightsOf(c *gin.Context) {
	resp, err := h.insights.ForOwner(c.Request.Context(), security.CurrentUserID(c), c.Param("cropId"))
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) learning(c *gin.Context) {
	body, err := h.client.LearningStatusJSON(c.Request.Context())
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.Header("Cache-Control", "private, max-age=60")
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(body))
}

func (h *Handler) cropProfiles(c *gin.Context) {
	body, err := h.client.CropProfilesJSON(c.Request.Context())
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(body))
}
