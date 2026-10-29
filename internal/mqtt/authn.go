package mqtt

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/cache"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/config"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/crops"
	"github.com/labstack/echo/v4"
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
	cache  cache.Cache
	topics *TopicResolver
	log    *slog.Logger
}

// NewAuthnController crea el controlador de autenticación de dispositivos con soporte de caché.
func NewAuthnController(cfg config.MQTT, cropsSvc DeviceVerifier, store cache.Cache, topics *TopicResolver, logger *slog.Logger) *AuthnController {
	if cfg.AuthnToken == "" {
		logger.Warn("MQTT_AUTHN_TOKEN vacío: el broker no podrá autenticar dispositivos (se rechazarán todos)")
	}
	return &AuthnController{token: cfg.AuthnToken, crops: cropsSvc, cache: store, topics: topics, log: logger}
}

// Register monta la ruta interna fuera de /api/v1 (sin JWT ni límite de tasa).
func (c *AuthnController) Register(engine *echo.Echo) {
	engine.POST("/internal/mqtt/authn", c.authenticate)
	// Sonda de salud: el broker comprueba que el endpoint responde; así marca el recurso como conectado.
	engine.GET("/internal/mqtt/authn", func(ctx echo.Context) error {
		return ctx.JSON(http.StatusOK, echo.Map{"status": "ok"})
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

func (c *AuthnController) authenticate(ctx echo.Context) error {
	if !c.authorized(ctx.Request().Header.Get(brokerTokenHeader)) {
		return ctx.JSON(http.StatusOK, deny)
	}
	var req authnRequest
	if err := json.NewDecoder(ctx.Request().Body).Decode(&req); err != nil {
		return ctx.JSON(http.StatusOK, deny)
	}

	// Consulta rápida en caché (evita pegar a PostgreSQL en cada reconexión o validación)
	cacheKey := "crop:key:" + req.Username
	if c.cache != nil {
		if cachedKey, ok := c.cache.Get(cacheKey); ok {
			if subtle.ConstantTimeCompare([]byte(cachedKey), []byte(req.Password)) != 1 {
				return ctx.JSON(http.StatusOK, deny)
			}
			return ctx.JSON(http.StatusOK, authnResponse{Result: "allow", ACL: c.deviceACL(req.Username)})
		}
	}

	crop, err := c.crops.Find(ctx.Request().Context(), req.Username)
	if err != nil {
		// Error real de la base: 5xx para que EMQX lo trate como "ignore" y no cachee un rechazo.
		c.log.Warn("no se pudo verificar el dispositivo contra la base", "cropId", req.Username, "err", err)
		return ctx.JSON(http.StatusServiceUnavailable, authnResponse{})
	}
	if crop == nil {
		return ctx.JSON(http.StatusOK, deny)
	}
	key, ok := c.crops.DeviceKey(crop)
	if !ok || subtle.ConstantTimeCompare([]byte(key), []byte(req.Password)) != 1 {
		return ctx.JSON(http.StatusOK, deny)
	}
	if c.cache != nil {
		c.cache.Put(cacheKey, key, 5*time.Minute)
	}
	return ctx.JSON(http.StatusOK, authnResponse{Result: "allow", ACL: c.deviceACL(crop.HexID())})
}

// Invalidate invalida la clave del dispositivo en caché (por borrado o rotación).
func (c *AuthnController) Invalidate(cropID string) {
	if c.cache != nil && cropID != "" {
		c.cache.Delete("crop:key:" + cropID)
	}
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
