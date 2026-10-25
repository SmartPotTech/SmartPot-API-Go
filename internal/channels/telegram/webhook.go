package telegram

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/config"
	"github.com/gin-gonic/gin"
)

// WebhookController es la entrada pública de los mensajes del bot en modo
// webhook. Telegram firma cada llamada con X-Telegram-Bot-Api-Secret-Token; sin
// el secreto correcto la llamada no existe (404).
type WebhookController struct {
	cfg      config.Telegram
	listener *Listener
}

// NewWebhookController crea el controlador del webhook.
func NewWebhookController(cfg config.Telegram, listener *Listener) *WebhookController {
	return &WebhookController{cfg: cfg, listener: listener}
}

// Register monta la ruta pública del webhook bajo /api/v1.
func (c *WebhookController) Register(g *gin.RouterGroup) {
	g.POST("/channels/telegram/webhook", c.receive)
}

func (c *WebhookController) receive(ctx *gin.Context) {
	secret := ctx.GetHeader("X-Telegram-Bot-Api-Secret-Token")
	if !c.enabled() || c.cfg.Mode != "webhook" || !c.matches(secret) {
		ctx.Status(http.StatusNotFound)
		return
	}
	var update Update
	if err := json.NewDecoder(ctx.Request.Body).Decode(&update); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	c.listener.HandleSafely(update)
	ctx.Status(http.StatusOK)
}

func (c *WebhookController) enabled() bool {
	return c.cfg.BotToken != "" && c.cfg.BotUsername != ""
}

func (c *WebhookController) matches(secret string) bool {
	expected := c.cfg.WebhookSecret
	if secret == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(secret)) == 1
}
