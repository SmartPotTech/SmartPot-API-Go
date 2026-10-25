// Package overview arma el panel general de la cuenta: totales, series
// comparables entre cultivos y el análisis de flota del asistente de IA.
package overview

import (
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/crops"
)

// Totals son los totales de la cuenta. needsAttention: cultivos desconectados o
// con salud por debajo de 70.
type Totals struct {
	Crops           int      `json:"crops"`
	Online          int      `json:"online"`
	Automated       int      `json:"automated"`
	AverageHealth   *float64 `json:"averageHealth,omitempty"`
	NeedsAttention  int      `json:"needsAttention"`
	UnreadAlerts    int64    `json:"unreadAlerts"`
	CommandsLast24h int64    `json:"commandsLast24h"`
}

// OverviewResponse es el panel general: totales y cada cultivo con su última lectura.
type OverviewResponse struct {
	Totals Totals               `json:"totals"`
	Crops  []crops.CropResponse `json:"crops"`
}

// Point es un promedio en un instante.
type Point struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

// CropSeries es la serie de un cultivo.
type CropSeries struct {
	CropID string  `json:"cropId"`
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	Points []Point `json:"points"`
}

// MetricSeriesResponse son los promedios de una variable por intervalo para
// comparar cultivos.
type MetricSeriesResponse struct {
	Metric        string       `json:"metric"`
	Hours         int          `json:"hours"`
	BucketMinutes int          `json:"bucketMinutes"`
	Series        []CropSeries `json:"series"`
}
