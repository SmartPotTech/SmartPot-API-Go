package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/cache"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/config"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
)

const (
	unavailableMsg = "El asistente de IA no está disponible en este momento"
	disabledMsg    = "El asistente de IA está deshabilitado en este servidor"
	profilesTTL    = time.Hour
	learningTTL    = time.Minute
	healthTTL      = 30 * time.Second
)

// Client es el cliente HTTP del servicio interno de IA.
type Client struct {
	enabled bool
	baseURL string
	token   string
	http    *http.Client
	cache   *cache.Store
	clock   func() time.Time

	mu              sync.Mutex
	healthCheckedAt time.Time
	healthy         bool
}

// NewClient crea el cliente del servicio de IA.
func NewClient(cfg config.AI, store *cache.Store, clock func() time.Time) *Client {
	return &Client{
		enabled: cfg.Enabled,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		token:   cfg.Token,
		http:    &http.Client{Timeout: cfg.Timeout},
		cache:   store,
		clock:   clock,
	}
}

// IsEnabled indica si el asistente está habilitado.
func (c *Client) IsEnabled() bool { return c.enabled }

// Insights evalúa un cultivo.
func (c *Client) Insights(ctx context.Context, req InsightRequest) (*InsightResponse, error) {
	if err := c.requireEnabled(); err != nil {
		return nil, err
	}
	var resp InsightResponse
	if err := c.do(ctx, http.MethodPost, "/v1/insights", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Fleet analiza todos los cultivos de una cuenta.
func (c *Client) Fleet(ctx context.Context, req FleetRequest) (*FleetResponse, error) {
	if err := c.requireEnabled(); err != nil {
		return nil, err
	}
	var resp FleetResponse
	if err := c.do(ctx, http.MethodPost, "/v1/fleet", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Learn envía un lote de lecturas reales para el aprendizaje continuo.
func (c *Client) Learn(ctx context.Context, batch LearningBatch) error {
	if err := c.requireEnabled(); err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, "/v1/learning/readings", batch, nil)
}

// Forget borra del aprendizaje las lecturas de un cultivo eliminado (best-effort).
func (c *Client) Forget(ctx context.Context, cropID string) {
	if !c.enabled {
		return
	}
	if err := c.do(ctx, http.MethodDelete, "/v1/learning/crops/"+cropID, nil, nil); err != nil {
		slog.Warn("no se pudieron olvidar las lecturas del cultivo", "cropId", cropID, "err", err)
	}
}

// LearningStatusJSON devuelve lo aprendido (agregado por especie), cacheado 1 min.
func (c *Client) LearningStatusJSON(ctx context.Context) (string, error) {
	if err := c.requireEnabled(); err != nil {
		return "", err
	}
	return c.cachedGet(ctx, "ai:learning-status", "/v1/learning/status", learningTTL)
}

// CropProfilesJSON devuelve los rangos óptimos por especie, cacheados 1 h.
func (c *Client) CropProfilesJSON(ctx context.Context) (string, error) {
	if err := c.requireEnabled(); err != nil {
		return "", err
	}
	return c.cachedGet(ctx, "ai:crop-profiles", "/v1/crop-profiles", profilesTTL)
}

// IsHealthy consulta /health del servicio de IA, cacheando el resultado 30 s.
func (c *Client) IsHealthy() bool {
	if !c.enabled {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clock()
	if now.Before(c.healthCheckedAt.Add(healthTTL)) {
		return c.healthy
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.http.Timeout)
	defer cancel()
	c.healthy = c.do(ctx, http.MethodGet, "/health", nil, nil) == nil
	c.healthCheckedAt = now
	return c.healthy
}

func (c *Client) cachedGet(ctx context.Context, key, path string, ttl time.Duration) (string, error) {
	if cached, ok := c.cache.Get(key); ok {
		return cached, nil
	}
	body, err := c.getRaw(ctx, path)
	if err != nil {
		return "", err
	}
	c.cache.Put(key, body, ttl)
	return body, nil
}

func (c *Client) getRaw(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return "", apperr.Unavailable(unavailableMsg)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		slog.Warn("falló la consulta al servicio de IA", "path", path, "err", err)
		return "", apperr.Unavailable(unavailableMsg)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", apperr.Unavailable(unavailableMsg)
	}
	return string(body), nil
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return apperr.Unavailable(unavailableMsg)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return apperr.Unavailable(unavailableMsg)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		slog.Warn("falló la consulta al servicio de IA", "path", path, "err", err)
		return apperr.Unavailable(unavailableMsg)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apperr.Unavailable(unavailableMsg)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return apperr.Unavailable(unavailableMsg)
		}
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	return nil
}

func (c *Client) requireEnabled() error {
	if !c.enabled {
		return apperr.Unavailable(disabledMsg)
	}
	return nil
}
