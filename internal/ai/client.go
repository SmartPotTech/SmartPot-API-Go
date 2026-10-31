package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/cache"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/config"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	smartpotv1 "github.com/SmartPotTech/SmartPot-API-Go/internal/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	unavailableMsg = "El asistente de IA no está disponible en este momento"
	disabledMsg    = "El asistente de IA está deshabilitado en este servidor"
	profilesTTL    = time.Hour
	learningTTL    = time.Minute
	healthTTL      = 30 * time.Second
)

// Client es el cliente HTTP y gRPC del servicio interno de IA con Circuit Breaker, Retry exponencial y Singleflight.
type Client struct {
	enabled bool
	baseURL string
	token   string
	http    *http.Client
	cache   cache.Cache
	clock   func() time.Time

	cb *CircuitBreaker
	sf *Singleflight

	grpcConn   *grpc.ClientConn
	grpcClient smartpotv1.SmartPotAIServiceClient

	mu              sync.Mutex
	healthCheckedAt time.Time
	healthy         bool
}

// NewClient crea el cliente del servicio de IA.
// Intenta conectar automáticamente via gRPC (por socket Unix o TCP) con degradación a REST.
func NewClient(cfg config.AI, store cache.Cache, clock func() time.Time) *Client {
	c := &Client{
		enabled: cfg.Enabled,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		token:   cfg.Token,
		http:    &http.Client{Timeout: cfg.Timeout},
		cache:   store,
		clock:   clock,
		cb:      NewCircuitBreaker(5, 10*time.Second, clock),
		sf:      NewSingleflight(),
	}

	if cfg.Enabled {
		target := ""
		if sock := os.Getenv("AI_GRPC_SOCKET"); sock != "" {
			target = "unix://" + sock
		} else if _, err := os.Stat("/var/run/smartpot/ai.sock"); err == nil {
			target = "unix:///var/run/smartpot/ai.sock"
		} else if strings.Contains(cfg.BaseURL, ":8000") {
			host := strings.TrimPrefix(cfg.BaseURL, "http://")
			host = strings.TrimPrefix(host, "https://")
			parts := strings.Split(host, ":")
			if len(parts) > 0 && parts[0] != "" {
				target = parts[0] + ":50051"
			}
		}

		if target != "" {
			conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err == nil {
				c.grpcConn = conn
				c.grpcClient = smartpotv1.NewSmartPotAIServiceClient(conn)
			}
		}
	}

	return c
}

// Close libera las conexiones gRPC activas.
func (c *Client) Close() error {
	if c.grpcConn != nil {
		return c.grpcConn.Close()
	}
	return nil
}

// IsEnabled indica si el asistente está habilitado.
func (c *Client) IsEnabled() bool { return c.enabled }

// CircuitState devuelve el estado actual del Circuit Breaker (CLOSED, HALF-OPEN, OPEN).
func (c *Client) CircuitState() CircuitState {
	if c.cb == nil {
		return StateClosed
	}
	return c.cb.State()
}

// Insights evalúa un cultivo priorizando gRPC de baja latencia con degradación transparente a HTTP REST.
func (c *Client) Insights(ctx context.Context, req InsightRequest) (*InsightResponse, error) {
	if err := c.requireEnabled(); err != nil {
		return nil, err
	}
	if c.grpcClient != nil {
		gResp, err := c.evalInsightGRPC(ctx, req)
		if err == nil && gResp != nil {
			return gResp, nil
		}
	}
	var resp InsightResponse
	if err := c.do(ctx, http.MethodPost, "/v1/insights", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) evalInsightGRPC(ctx context.Context, in InsightRequest) (*InsightResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var place *smartpotv1.Placement
	if in.Placement != nil {
		place = &smartpotv1.Placement{
			Setting:  in.Placement.Setting,
			Exposure: in.Placement.Exposure,
		}
	}

	val := func(p *float64) float64 {
		if p == nil {
			return 0.0
		}
		return *p
	}

	hist := make([]*smartpotv1.HistoryPoint, 0, len(in.History))
	for _, h := range in.History {
		hist = append(hist, &smartpotv1.HistoryPoint{
			MeasuredAtUnixMs: h.MeasuredAt.UnixMilli(),
			Measures: &smartpotv1.Measures{
				Temperature:  val(h.Temperature),
				Humidity:     val(h.Humidity),
				Brightness:   val(h.Brightness),
				Ph:           val(h.Ph),
				Tds:          val(h.Tds),
				Atmosphere:   val(h.Atmosphere),
				SoilMoisture: val(h.SoilMoisture),
			},
		})
	}

	localHour := int32(0)
	if in.LocalHour != nil {
		localHour = int32(*in.LocalHour)
	}

	grpcReq := &smartpotv1.InsightRequest{
		CropType:  in.CropType,
		Placement: place,
		Measures: &smartpotv1.Measures{
			Temperature:  val(in.Measures.Temperature),
			Humidity:     val(in.Measures.Humidity),
			Brightness:   val(in.Measures.Brightness),
			Ph:           val(in.Measures.Ph),
			Tds:          val(in.Measures.Tds),
			Atmosphere:   val(in.Measures.Atmosphere),
			SoilMoisture: val(in.Measures.SoilMoisture),
		},
		History:   hist,
		Actuators: in.Actuators,
		LocalHour: localHour,
	}

	res, err := c.grpcClient.EvaluateInsight(callCtx, grpcReq)
	if err != nil {
		return nil, err
	}

	out := &InsightResponse{
		CropType:    in.CropType,
		Summary:     res.Summary,
		EvaluatedAt: c.clock(),
	}

	if res.Health != nil {
		byParam := make(map[string]float64, len(res.Health.ByParameter))
		for k, v := range res.Health.ByParameter {
			byParam[k] = float64(v)
		}
		out.Health = &Health{
			Index:       float64(res.Health.Index),
			Level:       res.Health.Level,
			Label:       res.Health.Label,
			ByParameter: byParam,
		}
	}

	if res.Diagnosis != "" {
		out.Diagnosis = []Diagnosis{
			{Message: res.Diagnosis},
		}
	}

	for _, cStr := range res.Conclusions {
		out.Conclusions = append(out.Conclusions, Conclusion{Title: cStr})
	}

	for _, a := range res.Actions {
		dur := int(a.DurationSeconds)
		out.Actions = append(out.Actions, Action{
			Actuator:        a.Actuator,
			Action:          a.Action,
			Reason:          a.Reason,
			DurationSeconds: &dur,
		})
	}

	return out, nil
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

// LearningStatusJSON devuelve lo aprendido (agregado por especie), cacheado 1 min y protegido contra Cache Stampede.
func (c *Client) LearningStatusJSON(ctx context.Context) (string, error) {
	if err := c.requireEnabled(); err != nil {
		return "", err
	}
	return c.cachedGet(ctx, "ai:learning-status", "/v1/learning/status", learningTTL)
}

// CropProfilesJSON devuelve los rangos óptimos por especie, cacheados 1 h y protegidos contra Cache Stampede.
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

	// Singleflight: si hay múltiples solicitudes concurrentes, solo una hace la petición HTTP
	val, err := c.sf.Do(key, func() (any, error) {
		if cached, ok := c.cache.Get(key); ok {
			return cached, nil
		}
		body, err := c.getRaw(ctx, path)
		if err != nil {
			return "", err
		}
		c.cache.Put(key, body, ttl)
		return body, nil
	})
	if err != nil {
		return "", err
	}
	return val.(string), nil
}

func (c *Client) getRaw(ctx context.Context, path string) (string, error) {
	var body string
	err := c.do(ctx, http.MethodGet, path, nil, &body)
	return body, err
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	isIdempotent := method == http.MethodGet

	return c.cb.Execute(func() error {
		return RetryWithBackoff(ctx, 3, 50*time.Millisecond, func(err error) bool {
			return isIdempotent && isTransientError(err)
		}, func() error {
			return c.doRaw(ctx, method, path, body, out)
		})
	})
}

func (c *Client) doRaw(ctx context.Context, method, path string, body, out any) error {
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
		if strPtr, ok := out.(*string); ok {
			rawBytes, _ := io.ReadAll(resp.Body)
			*strPtr = string(rawBytes)
			return nil
		}
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
