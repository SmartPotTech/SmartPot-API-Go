package mqtt

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/crops"
)

const (
	deviceRole      = "device"
	adminRole       = "admin"
	batchSize       = 40
	responseTimeout = 5 * time.Second
)

// Provisioner administra las cuentas de dispositivo en Mosquitto con el plugin
// de seguridad dinámica: un cliente por cultivo (usuario = id del cultivo) con
// el rol «device». Implementa crops.DeviceProvisioner.
type Provisioner struct {
	gateway *Gateway
	topics  *TopicResolver
	crops   *crops.Service
	log     *slog.Logger

	mu      sync.Mutex
	pending map[string]chan []dynsecResponse
}

type dynsecResponse struct {
	CorrelationData string `json:"correlationData"`
	Command         string `json:"command"`
	Error           string `json:"error"`
}

// NewProvisioner crea el aprovisionador.
func NewProvisioner(gateway *Gateway, topics *TopicResolver, cropsSvc *crops.Service, logger *slog.Logger) *Provisioner {
	return &Provisioner{
		gateway: gateway, topics: topics, crops: cropsSvc, log: logger,
		pending: make(map[string]chan []dynsecResponse),
	}
}

// Provision crea o actualiza la cuenta del dispositivo. Con rotate desconecta la
// sesión anterior.
func (p *Provisioner) Provision(_ context.Context, cropID, key string, rotate bool) error {
	if !p.gateway.IsEnabled() {
		return nil
	}
	cmds := upsertClient(cropID, key)
	if rotate {
		cmds = append(cmds,
			cmd("disableClient", map[string]any{"username": cropID}),
			cmd("enableClient", map[string]any{"username": cropID}))
	}
	p.send(cmds)
	return nil
}

// Deprovision borra la cuenta del dispositivo.
func (p *Provisioner) Deprovision(_ context.Context, cropID string) error {
	if !p.gateway.IsEnabled() {
		return nil
	}
	p.send([]map[string]any{cmd("deleteClient", map[string]any{"username": cropID})})
	return nil
}

// OnConnected sincroniza el broker al (re)conectar: crea los roles y las cuentas
// de todos los cultivos con clave.
func (p *Provisioner) OnConnected(_ bool) {
	p.ensureRoles()
	accounts, err := p.crops.DeviceAccounts(context.Background())
	if err != nil {
		p.log.Warn("no se pudieron listar las cuentas de dispositivo", "err", err)
		return
	}
	var cmds []map[string]any
	for _, a := range accounts {
		cmds = append(cmds, upsertClient(a.CropID, a.Key)...)
	}
	for from := 0; from < len(cmds); from += batchSize {
		to := from + batchSize
		if to > len(cmds) {
			to = len(cmds)
		}
		p.send(cmds[from:to])
	}
	p.log.Info("broker sincronizado", "dispositivos", len(accounts))
}

// OnControlResponse procesa las respuestas del plugin de seguridad dinámica.
func (p *Provisioner) OnControlResponse(payload string) {
	var parsed struct {
		Responses []dynsecResponse `json:"responses"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return
	}
	byBatch := make(map[string][]dynsecResponse)
	for _, r := range parsed.Responses {
		batch := r.CorrelationData
		if i := strings.IndexByte(batch, ':'); i >= 0 {
			batch = batch[:i]
		}
		byBatch[batch] = append(byBatch[batch], r)
		if r.Error != "" && !isIgnorable(r.Error) {
			p.log.Warn("el broker rechazó un comando de seguridad", "command", r.Command, "error", r.Error)
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for batch, list := range byBatch {
		if ch, ok := p.pending[batch]; ok {
			delete(p.pending, batch)
			ch <- list
		}
	}
}

func (p *Provisioner) ensureRoles() {
	cmds := []map[string]any{cmd("createRole", map[string]any{"rolename": deviceRole})}
	for _, topic := range p.topics.DevicePublishPatterns() {
		cmds = append(cmds, acl(deviceRole, "publishClientSend", topic))
	}
	cmds = append(cmds,
		acl(deviceRole, "subscribePattern", p.topics.DeviceSubscribePattern()),
		acl(adminRole, "publishClientSend", p.topics.ServiceScope()))
	p.send(cmds)
}

func (p *Provisioner) send(cmds []map[string]any) bool {
	if len(cmds) == 0 || !p.gateway.IsConnected() {
		return false
	}
	batch := randomBatch()
	tagged := make([]map[string]any, len(cmds))
	for i, c := range cmds {
		copy := make(map[string]any, len(c)+1)
		for k, v := range c {
			copy[k] = v
		}
		copy["correlationData"] = batch + ":" + strconv.Itoa(i)
		tagged[i] = copy
	}
	ch := make(chan []dynsecResponse, 1)
	p.mu.Lock()
	p.pending[batch] = ch
	p.mu.Unlock()

	payload, _ := json.Marshal(map[string]any{"commands": tagged})
	if !p.gateway.Publish(ControlTopic, string(payload), 1, false) {
		p.mu.Lock()
		delete(p.pending, batch)
		p.mu.Unlock()
		return false
	}
	select {
	case <-ch:
		return true
	case <-time.After(responseTimeout):
		p.mu.Lock()
		delete(p.pending, batch)
		p.mu.Unlock()
		p.log.Warn("el broker no respondió a tiempo a los comandos de seguridad", "comandos", len(cmds))
		return false
	}
}

func upsertClient(cropID, key string) []map[string]any {
	return []map[string]any{
		cmd("createClient", map[string]any{
			"username": cropID,
			"password": key,
			"roles":    []map[string]any{{"rolename": deviceRole}},
		}),
		cmd("setClientPassword", map[string]any{"username": cropID, "password": key}),
	}
}

func acl(role, aclType, topic string) map[string]any {
	return cmd("addRoleACL", map[string]any{
		"rolename": role,
		"acltype":  aclType,
		"topic":    topic,
		"priority": 5,
		"allow":    true,
	})
}

func cmd(name string, fields map[string]any) map[string]any {
	m := map[string]any{"command": name}
	for k, v := range fields {
		m[k] = v
	}
	return m
}

func isIgnorable(err string) bool {
	e := strings.ToLower(err)
	return strings.Contains(e, "already") || strings.Contains(e, "not found")
}

func randomBatch() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
