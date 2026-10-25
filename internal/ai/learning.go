package ai

import (
	"context"
	"sync"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/crops"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/readings"
)

const (
	maxBatch   = 1000
	maxPending = 20000
)

// LearningFeed lleva las lecturas reales al servicio de IA para el aprendizaje
// continuo (las de los cultivos virtuales son sintéticas y no se envían). Se
// acumulan y se envían por lotes; si la IA no responde, el lote vuelve a la cola.
type LearningFeed struct {
	client   *Client
	crops    *crops.Service
	enabled  bool
	timezone *time.Location

	mu      sync.Mutex
	pending []LearningReading
}

// NewLearningFeed crea el feed de aprendizaje.
func NewLearningFeed(client *Client, cropsSvc *crops.Service, learningEnabled bool, timezone *time.Location) *LearningFeed {
	return &LearningFeed{client: client, crops: cropsSvc, enabled: client.IsEnabled() && learningEnabled, timezone: timezone}
}

// OnReading encola una lectura real para aprender.
func (f *LearningFeed) OnReading(event readings.ReadingRecordedEvent) {
	if !f.enabled {
		return
	}
	crop, err := f.crops.Find(context.Background(), event.CropID)
	if err != nil || crop == nil || crop.IsVirtual() {
		return
	}
	localHour := event.Reading.MeasuredAt.In(f.timezone).Hour()
	item := LearningReading{
		CropID:     crop.HexID(),
		CropType:   string(crop.Type),
		MeasuredAt: event.Reading.MeasuredAt,
		LocalHour:  &localHour,
		Measures:   event.Reading.Measures,
	}
	f.mu.Lock()
	f.pending = append(f.pending, item)
	if len(f.pending) > maxPending {
		f.pending = f.pending[len(f.pending)-maxPending:]
	}
	f.mu.Unlock()
}

// Flush envía los lotes pendientes; si la IA no responde, el lote vuelve a la cola.
func (f *LearningFeed) Flush(ctx context.Context) {
	for {
		f.mu.Lock()
		if len(f.pending) == 0 {
			f.mu.Unlock()
			return
		}
		n := maxBatch
		if n > len(f.pending) {
			n = len(f.pending)
		}
		batch := make([]LearningReading, n)
		copy(batch, f.pending[:n])
		f.pending = f.pending[n:]
		f.mu.Unlock()

		if err := f.client.Learn(ctx, LearningBatch{Readings: batch}); err != nil {
			f.mu.Lock()
			f.pending = append(batch, f.pending...)
			f.mu.Unlock()
			return
		}
	}
}

// OnCropDeleted olvida las lecturas de un cultivo eliminado. Corre en su goroutine.
func (f *LearningFeed) OnCropDeleted(event crops.DeletedEvent) {
	f.mu.Lock()
	kept := f.pending[:0]
	for _, r := range f.pending {
		if r.CropID != event.CropID {
			kept = append(kept, r)
		}
	}
	f.pending = kept
	f.mu.Unlock()
	f.client.Forget(context.Background(), event.CropID)
}
