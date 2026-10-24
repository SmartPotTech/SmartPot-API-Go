// Package mqtt implementa el cliente del broker, el contrato de tópicos v1 y el
// aprovisionamiento de dispositivos en Mosquitto (dynamic security).
package mqtt

import (
	"strings"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/objectid"
)

// Tópicos de control de la seguridad dinámica de Mosquitto.
const (
	ControlTopic         = "$CONTROL/dynamic-security/v1"
	ControlResponseTopic = ControlTopic + "/response"
)

// Kind es el tipo de mensaje de un tópico del dispositivo.
type Kind int

const (
	KindTelemetry Kind = iota
	KindCommandAck
	KindStatus
)

// ParsedTopic es el resultado de interpretar un tópico entrante.
type ParsedTopic struct {
	CropID string
	Kind   Kind
}

// TopicResolver arma e interpreta los tópicos v1: {prefijo}/{cropId}/telemetry |
// commands | commands/ack | status.
type TopicResolver struct {
	prefix string
}

// NewTopicResolver crea el resolutor con el prefijo configurado.
func NewTopicResolver(prefix string) *TopicResolver {
	return &TopicResolver{prefix: prefix}
}

func (r *TopicResolver) Telemetry(cropID string) string {
	return r.prefix + "/" + cropID + "/telemetry"
}
func (r *TopicResolver) Commands(cropID string) string { return r.prefix + "/" + cropID + "/commands" }
func (r *TopicResolver) CommandAck(cropID string) string {
	return r.prefix + "/" + cropID + "/commands/ack"
}
func (r *TopicResolver) Status(cropID string) string { return r.prefix + "/" + cropID + "/status" }

// Subscriptions son los tópicos que la API escucha de todos los cultivos.
func (r *TopicResolver) Subscriptions() []string {
	return []string{r.Telemetry("+"), r.CommandAck("+"), r.Status("+")}
}

// DevicePublishPatterns son los patrones de publicación del rol de dispositivo
// (%u es el usuario MQTT, que es el id del cultivo).
func (r *TopicResolver) DevicePublishPatterns() []string {
	return []string{r.Telemetry("%u"), r.CommandAck("%u"), r.Status("%u")}
}

// DeviceSubscribePattern es el patrón de suscripción del rol de dispositivo.
func (r *TopicResolver) DeviceSubscribePattern() string {
	return r.Commands("%u")
}

// ServiceScope es el alcance de la cuenta administradora.
func (r *TopicResolver) ServiceScope() string {
	return r.prefix + "/#"
}

// Parse interpreta un tópico entrante; devuelve false si no es un tópico v1 válido.
func (r *TopicResolver) Parse(topic string) (ParsedTopic, bool) {
	if topic == "" || !strings.HasPrefix(topic, r.prefix+"/") {
		return ParsedTopic{}, false
	}
	segments := strings.Split(topic[len(r.prefix)+1:], "/")
	if len(segments) < 2 || !objectid.Valid(segments[0]) {
		return ParsedTopic{}, false
	}
	cropID := segments[0]
	switch {
	case len(segments) == 2 && segments[1] == "telemetry":
		return ParsedTopic{CropID: cropID, Kind: KindTelemetry}, true
	case len(segments) == 2 && segments[1] == "status":
		return ParsedTopic{CropID: cropID, Kind: KindStatus}, true
	case len(segments) == 3 && segments[1] == "commands" && segments[2] == "ack":
		return ParsedTopic{CropID: cropID, Kind: KindCommandAck}, true
	default:
		return ParsedTopic{}, false
	}
}
