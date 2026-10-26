package mqtt

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/config"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/crops"
	"github.com/gin-gonic/gin"
)

// brokerTokenHeader es la cabecera con el secreto compartido que el broker envía en cada llamada.
const brokerTokenHeader = "x-smartpot-broker-token"

// DeviceVerifier es lo que el controlador necesita para autenticar un dispositivo: buscar el
// cultivo por id y obtener su clave en claro. Lo implementa crops.Service.
type DeviceVerifier interface {
	Find(ctx context.Context, cropID string) (*crops.Crop, error)
	DeviceKey(crop *crops.Crop) (string, bool)
}

// AuthnController es el endpoint interno que el broker (EMQX) llama para autenticar
// dispositivos. La API verifica la clave del cultivo contra su base y devuelve el
// permiso y un ACL acotado a ese cultivo. No lleva JWT ni límite de tasa: lo protegen
// el secreto compartido (x-smartpot-broker-token) y la red interna de Docker.
type AuthnController struct {
	token  string
	crops  DeviceVerifier
	topics *TopicResolver
	log    *slog.Logger
}

// NewAuthnController crea el controlador de autenticación de dispositivos.
func NewAuthnController(cfg config.MQTT, cropsSvc DeviceVerifier, topics *TopicResolver, logger *slog.Logger) *AuthnController {
	if cfg.AuthnToken == "" {
		logger.Warn("MQTT_AUTHN_TOKEN vacío: el broker no podrá autenticar dispositivos (se rechazarán todos)")
	}
	return &AuthnController{token: cfg.AuthnToken, crops: cropsSvc, topics: topics, log: logger}
}

// Register monta la ruta interna fuera de /api/v1 (sin JWT ni límite de tasa).
func (c *AuthnController) Register(engine gin.IRouter) {
	engine.POST("/internal/mqtt/authn", c.authenticate)
	// Sonda de salud: el broker comprueba que el endpoint responde; así marca el recurso como conectado.
	engine.GET("/internal/mqtt/authn", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

type authnRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	ClientID string `json:"clientid"`
}

// aclRule es una regla del ACL que EMQX adjunta a la conexión (campo acl, EMQX 5.8+).
type aclRule struct {
	Permission string `json:"permission"`
	Action     string `json:"action"`
	Topic      string `json:"topic"`
}

type authnResponse struct {
	Result string    `json:"result"`
	ACL    []aclRule `json:"acl,omitempty"`
}

var deny = authnResponse{Result: "deny"}

func (c *AuthnController) authenticate(ctx *gin.Context) {
	if !c.authorized(ctx.GetHeader(brokerTokenHeader)) {
		ctx.JSON(http.StatusOK, deny)
		return
	}
	var req authnRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusOK, deny)
		return
	}
	crop, err := c.crops.Find(ctx.Request.Context(), req.Username)
	if err != nil {
		// Error real de la base: 5xx para que EMQX lo trate como "ignore" y no cachee un rechazo.
		c.log.Warn("no se pudo verificar el dispositivo contra la base", "cropId", req.Username, "err", err)
		ctx.JSON(http.StatusServiceUnavailable, authnResponse{})
		return
	}
	if crop == nil {
		ctx.JSON(http.StatusOK, deny)
		return
	}
	key, ok := c.crops.DeviceKey(crop)
	if !ok || subtle.ConstantTimeCompare([]byte(key), []byte(req.Password)) != 1 {
		ctx.JSON(http.StatusOK, deny)
		return
	}
	ctx.JSON(http.StatusOK, authnResponse{Result: "allow", ACL: c.deviceACL(crop.HexID())})
}

// deviceACL acota el dispositivo a los tópicos de su propio cultivo; lo demás se niega.
func (c *AuthnController) deviceACL(cropID string) []aclRule {
	return []aclRule{
		{Permission: "allow", Action: "publish", Topic: c.topics.Telemetry(cropID)},
		{Permission: "allow", Action: "publish", Topic: c.topics.Status(cropID)},
		{Permission: "allow", Action: "publish", Topic: c.topics.CommandAck(cropID)},
		{Permission: "allow", Action: "subscribe", Topic: c.topics.Commands(cropID)},
		{Permission: "deny", Action: "all", Topic: "#"},
	}
}

func (c *AuthnController) authorized(token string) bool {
	if c.token == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.token), []byte(token)) == 1
}
