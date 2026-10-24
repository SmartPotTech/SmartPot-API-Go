package notifications

import (
	"net/http"
	"strconv"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/security"
	"github.com/gin-gonic/gin"
)

// Handler expone las rutas de notificaciones.
type Handler struct {
	svc *Service
}

// NewHandler crea el controlador de notificaciones.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register monta las rutas bajo el grupo autenticado /api/v1.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/notifications", h.list)
	g.GET("/notifications/unread-count", h.unreadCount)
	g.PUT("/notifications/:id/read", h.markRead)
	g.PUT("/notifications/read-all", h.markAllRead)
	g.DELETE("/notifications/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	unreadOnly, ok := queryBool(c, "unreadOnly", false)
	if !ok {
		httpserver.RespondError(c, apperr.BadRequest("Parámetro inválido o faltante"))
		return
	}
	limit, ok := queryInt(c, "limit", 50)
	if !ok {
		httpserver.RespondError(c, apperr.BadRequest("Parámetro inválido o faltante"))
		return
	}
	items, err := h.svc.List(c.Request.Context(), security.CurrentUserID(c), unreadOnly, limit)
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	out := make([]Response, 0, len(items))
	for _, n := range items {
		out = append(out, ToResponse(n))
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handler) unreadCount(c *gin.Context) {
	n, err := h.svc.UnreadCount(c.Request.Context(), security.CurrentUserID(c))
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"unread": n})
}

func (h *Handler) markRead(c *gin.Context) {
	n, err := h.svc.MarkRead(c.Request.Context(), security.CurrentUserID(c), c.Param("id"))
	if err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, ToResponse(*n))
}

func (h *Handler) markAllRead(c *gin.Context) {
	if err := h.svc.MarkAllRead(c.Request.Context(), security.CurrentUserID(c)); err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), security.CurrentUserID(c), c.Param("id")); err != nil {
		httpserver.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func queryBool(c *gin.Context, key string, def bool) (bool, bool) {
	raw := c.Query(key)
	if raw == "" {
		return def, true
	}
	v, err := strconv.ParseBool(raw)
	return v, err == nil
}

func queryInt(c *gin.Context, key string, def int) (int, bool) {
	raw := c.Query(key)
	if raw == "" {
		return def, true
	}
	v, err := strconv.Atoi(raw)
	return v, err == nil
}
