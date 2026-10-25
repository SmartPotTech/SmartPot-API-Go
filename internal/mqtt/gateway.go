package mqtt

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/config"
	paho "github.com/eclipse/paho.mqtt.golang"
)

const (
	publishTimeout = 5 * time.Second
	connectTimeout = 10 * time.Second
)

// Gateway es el cliente del broker con reconexión automática. Los mensajes se
// procesan en goroutines para no bloquear el hilo de callbacks de Paho.
type Gateway struct {
	enabled   bool
	client    paho.Client
	topics    *TopicResolver
	router    func(topic, payload string)
	onConnect func(reconnect bool)
	log       *slog.Logger
	connected atomic.Bool
}

// NewGateway crea el gateway. Si MQTT está deshabilitado, no abre conexión.
func NewGateway(cfg config.MQTT, topics *TopicResolver, logger *slog.Logger) *Gateway {
	g := &Gateway{enabled: cfg.Enabled, topics: topics, log: logger}
	if !cfg.Enabled {
		logger.Info("MQTT deshabilitado: la API no recibirá telemetría ni enviará comandos")
		return g
	}
	opts := paho.NewClientOptions().
		AddBroker(cfg.BrokerURI).
		SetClientID(cfg.ClientID + "-" + randomSuffix()).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetMaxReconnectInterval(30 * time.Second).
		SetKeepAlive(30 * time.Second).
		SetConnectTimeout(connectTimeout).
		SetOnConnectHandler(g.onConnected).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			logger.Warn("se perdió la conexión con el broker MQTT", "err", err)
		}).
		SetDefaultPublishHandler(g.onMessage)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		opts.SetPassword(cfg.Password)
	}
	g.client = paho.NewClient(opts)
	return g
}

// SetRouter fija el enrutador de mensajes entrantes.
func (g *Gateway) SetRouter(router func(topic, payload string)) { g.router = router }

// SetOnConnect fija el callback que corre al (re)conectar.
func (g *Gateway) SetOnConnect(fn func(reconnect bool)) { g.onConnect = fn }

// Start conecta al broker (con reintentos en segundo plano).
func (g *Gateway) Start() {
	if !g.enabled {
		return
	}
	g.client.Connect()
}

// Close cierra la conexión.
func (g *Gateway) Close() {
	if g.enabled && g.client != nil {
		g.client.Disconnect(3000)
	}
}

// IsEnabled indica si MQTT está habilitado.
func (g *Gateway) IsEnabled() bool { return g.enabled }

// IsConnected indica si hay conexión con el broker.
func (g *Gateway) IsConnected() bool {
	return g.enabled && g.client != nil && g.client.IsConnected()
}

// Publish publica un mensaje; devuelve false si el broker no lo aceptó.
func (g *Gateway) Publish(topic, payload string, qos byte, retained bool) bool {
	if !g.IsConnected() {
		return false
	}
	token := g.client.Publish(topic, qos, retained, payload)
	if !token.WaitTimeout(publishTimeout) {
		g.log.Warn("tiempo de espera agotado al publicar", "topic", topic)
		return false
	}
	if err := token.Error(); err != nil {
		g.log.Warn("no se pudo publicar", "topic", topic, "err", err)
		return false
	}
	return true
}

// PublishCommand publica una orden en el tópico de comandos del cultivo (QoS 1).
func (g *Gateway) PublishCommand(cropID, payload string) bool {
	return g.Publish(g.topics.Commands(cropID), payload, 1, false)
}

func (g *Gateway) onConnected(c paho.Client) {
	subscriptions := map[string]byte{ControlResponseTopic: 1}
	for _, topic := range g.topics.Subscriptions() {
		subscriptions[topic] = 1
	}
	token := c.SubscribeMultiple(subscriptions, g.onMessage)
	if token.WaitTimeout(connectTimeout) && token.Error() != nil {
		g.log.Error("no se pudo suscribir a los tópicos de SmartPot", "err", token.Error())
	}
	reconnect := g.connected.Swap(true)
	g.log.Info("conectado al broker MQTT", "reconexion", reconnect)
	if g.onConnect != nil {
		go g.onConnect(reconnect)
	}
}

func (g *Gateway) onMessage(_ paho.Client, m paho.Message) {
	if g.router == nil {
		return
	}
	topic, payload := m.Topic(), string(m.Payload())
	go g.router(topic, payload)
}

func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
