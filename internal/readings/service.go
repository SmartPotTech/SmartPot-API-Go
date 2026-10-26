package readings

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/cache"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/events"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/objectid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const exportLimit = 50000

// Crops es lo que el servicio de lecturas necesita de los cultivos, sin importar
// el paquete crops (lo implementa CropService).
type Crops interface {
	Exists(ctx context.Context, cropID string) bool
	RequireOwned(ctx context.Context, ownerID, cropID string) error
	MarkDeviceSeen(ctx context.Context, cropID string, online *bool)
}

// Service implementa la lógica de lecturas.
type Service struct {
	repo        Repository
	crops       Crops
	cache       cache.Cache
	bus         *events.Bus
	clock       func() time.Time
	minInterval time.Duration
	maxPageSize int
}

// NewService crea el servicio de lecturas.
func NewService(repo Repository, crops Crops, store cache.Cache, bus *events.Bus, clock func() time.Time,
	minInterval time.Duration, maxPageSize int) *Service {
	return &Service{repo: repo, crops: crops, cache: store, bus: bus, clock: clock,
		minInterval: minInterval, maxPageSize: maxPageSize}
}

// RecordFromDevice guarda la telemetría del dispositivo. Se descarta si el
// cultivo no existe o si llega más rápido que el intervalo mínimo.
func (s *Service) RecordFromDevice(ctx context.Context, cropID string, measures Measures) (*Reading, error) {
	if !s.crops.Exists(ctx, cropID) {
		slog.Debug("telemetría descartada: el cultivo no existe", "cropId", cropID)
		return nil, nil
	}
	online := true
	s.crops.MarkDeviceSeen(ctx, cropID, &online)
	if !s.cache.SetIfAbsent("reading:"+cropID, s.minInterval) {
		slog.Debug("telemetría descartada por frecuencia", "cropId", cropID)
		return nil, nil
	}
	reading, err := s.store(ctx, cropID, measures, SourceMQTT)
	if err != nil {
		slog.Debug("telemetría descartada por rango", "cropId", cropID, "err", err)
		return nil, nil
	}
	return reading, nil
}

// RecordManual guarda una lectura manual (dispositivos sin MQTT o pruebas).
func (s *Service) RecordManual(ctx context.Context, ownerID, cropID string, measures Measures) (*Reading, error) {
	if err := s.crops.RequireOwned(ctx, ownerID, cropID); err != nil {
		return nil, err
	}
	reading, err := s.store(ctx, cropID, measures, SourceHTTP)
	if err != nil {
		return nil, apperr.BadRequest(err.Error())
	}
	return reading, nil
}

// Latest devuelve la última lectura del cultivo (sin comprobar dueño).
func (s *Service) Latest(ctx context.Context, cropID string) (*Reading, error) {
	oid, ok := objectid.Parse(cropID)
	if !ok {
		return nil, nil
	}
	return s.repo.Latest(ctx, oid)
}

// LatestOwned devuelve la última lectura comprobando el dueño.
func (s *Service) LatestOwned(ctx context.Context, ownerID, cropID string) (*Reading, error) {
	if err := s.crops.RequireOwned(ctx, ownerID, cropID); err != nil {
		return nil, err
	}
	return s.Latest(ctx, cropID)
}

// Recent devuelve las últimas lecturas del cultivo, más recientes primero.
func (s *Service) Recent(ctx context.Context, cropID string, count int) ([]Reading, error) {
	oid, ok := objectid.Parse(cropID)
	if !ok {
		return nil, nil
	}
	return s.repo.Recent(ctx, oid, count)
}

// List devuelve las lecturas en orden cronológico; por defecto, las últimas 24 horas.
func (s *Service) List(ctx context.Context, ownerID, cropID string, from, to *time.Time, limit *int) ([]Reading, error) {
	if err := s.crops.RequireOwned(ctx, ownerID, cropID); err != nil {
		return nil, err
	}
	start, end, err := s.window(from, to)
	if err != nil {
		return nil, err
	}
	size := s.maxPageSize
	if limit != nil {
		size = clamp(*limit, 1, s.maxPageSize)
	}
	oid, _ := objectid.Parse(cropID)
	readings, err := s.repo.Between(ctx, oid, start, end, size)
	if err != nil {
		return nil, err
	}
	return reversed(readings), nil
}

// Summary calcula el resumen estadístico de la ventana pedida.
func (s *Service) Summary(ctx context.Context, ownerID, cropID string, hours int) (ReadingSummaryResponse, error) {
	if err := s.crops.RequireOwned(ctx, ownerID, cropID); err != nil {
		return ReadingSummaryResponse{}, err
	}
	to := s.clock()
	from := to.Add(-time.Duration(clamp(hours, 1, 24*90)) * time.Hour)
	oid, _ := objectid.Parse(cropID)
	count, metrics, err := s.repo.Summary(ctx, oid, from, to)
	if err != nil {
		return ReadingSummaryResponse{}, err
	}
	return ReadingSummaryResponse{From: from, To: to, Count: count, Metrics: metrics}, nil
}

// ExportCSV exporta hasta 50 000 lecturas del rango en CSV.
func (s *Service) ExportCSV(ctx context.Context, ownerID, cropID string, from, to *time.Time) (string, error) {
	if err := s.crops.RequireOwned(ctx, ownerID, cropID); err != nil {
		return "", err
	}
	start, end, err := s.window(from, to)
	if err != nil {
		return "", err
	}
	oid, _ := objectid.Parse(cropID)
	readings, err := s.repo.Between(ctx, oid, start, end, exportLimit)
	if err != nil {
		return "", err
	}
	readings = reversed(readings)
	var b strings.Builder
	b.WriteString("measuredAt," + strings.Join(Metrics, ",") + "\n")
	for _, r := range readings {
		m := r.Measures
		b.WriteString(r.MeasuredAt.UTC().Format(time.RFC3339Nano))
		for _, v := range []*float64{m.Temperature, m.Humidity, m.Brightness, m.Ph, m.Tds, m.Atmosphere, m.SoilMoisture} {
			b.WriteByte(',')
			b.WriteString(cell(v))
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// SeriesAverages calcula promedios por cultivo e intervalo (para el panel general).
func (s *Service) SeriesAverages(ctx context.Context, cropIDs []string, metric string, from time.Time, bucketMinutes int) ([]Bucket, error) {
	oids := make([]primitive.ObjectID, 0, len(cropIDs))
	for _, id := range cropIDs {
		if oid, ok := objectid.Parse(id); ok {
			oids = append(oids, oid)
		}
	}
	return s.repo.SeriesAverages(ctx, oids, metric, from, bucketMinutes)
}

// DeleteByCrop borra las lecturas de un cultivo (borrado en cascada por evento).
func (s *Service) DeleteByCrop(ctx context.Context, cropID string) error {
	oid, ok := objectid.Parse(cropID)
	if !ok {
		return nil
	}
	return s.repo.DeleteByCrop(ctx, oid)
}

func (s *Service) store(ctx context.Context, cropID string, measures Measures, source Source) (*Reading, error) {
	if err := ValidateRanges(measures); err != nil {
		return nil, err
	}
	oid, ok := objectid.Parse(cropID)
	if !ok {
		return nil, apperr.NotFound("El cultivo no existe")
	}
	reading := &Reading{CropID: oid, MeasuredAt: s.clock(), Measures: measures, Source: source}
	if err := s.repo.Insert(ctx, reading); err != nil {
		return nil, err
	}
	s.bus.Publish(ReadingRecordedEvent{CropID: cropID, Reading: *reading})
	return reading, nil
}

func (s *Service) window(from, to *time.Time) (time.Time, time.Time, error) {
	end := s.clock()
	if to != nil {
		end = *to
	}
	start := end.Add(-24 * time.Hour)
	if from != nil {
		start = *from
	}
	if start.After(end) {
		return time.Time{}, time.Time{}, apperr.BadRequest("La fecha inicial debe ser anterior a la final")
	}
	return start, end, nil
}

func reversed(in []Reading) []Reading {
	for i, j := 0, len(in)-1; i < j; i, j = i+1, j-1 {
		in[i], in[j] = in[j], in[i]
	}
	return in
}

func cell(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'g', -1, 64)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
