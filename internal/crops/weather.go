package crops

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/cache"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/virtualdevices"
)

const (
	weatherTTL        = 10 * time.Minute
	weatherRetryAfter = 2 * time.Minute
	weatherNone       = "none"
)

// WeatherSource obtiene el clima de un lugar (lo implementa el simulador).
type WeatherSource interface {
	IsAvailable() bool
	Weather(ctx context.Context, latitude, longitude float64) (*virtualdevices.Weather, bool)
}

// WeatherService devuelve el clima actual del lugar de un cultivo. Lo consulta
// el simulador (único con salida al servicio de clima) y lo guarda 10 minutos
// por lugar; si falla, no reintenta en 2 minutos.
type WeatherService struct {
	source WeatherSource
	cache  cache.Cache
}

// NewWeatherService crea el servicio de clima.
func NewWeatherService(source WeatherSource, store cache.Cache) *WeatherService {
	return &WeatherService{source: source, cache: store}
}

// Current devuelve el clima del lugar del cultivo (false si no hay lugar o clima).
func (s *WeatherService) Current(ctx context.Context, crop *Crop) (*virtualdevices.Weather, bool) {
	if crop.Placement == nil || crop.Placement.Location == nil || !s.source.IsAvailable() {
		return nil, false
	}
	loc := crop.Placement.Location
	key := fmt.Sprintf("weather:%.2f:%.2f", loc.Latitude, loc.Longitude)
	if cached, ok := s.cache.Get(key); ok {
		if cached == weatherNone {
			return nil, false
		}
		var w virtualdevices.Weather
		if err := json.Unmarshal([]byte(cached), &w); err != nil {
			return nil, false
		}
		return &w, true
	}
	weather, ok := s.source.Weather(ctx, loc.Latitude, loc.Longitude)
	if ok {
		if data, err := json.Marshal(weather); err == nil {
			s.cache.Put(key, string(data), weatherTTL)
		}
		return weather, true
	}
	s.cache.Put(key, weatherNone, weatherRetryAfter)
	return nil, false
}
