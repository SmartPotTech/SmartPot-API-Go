// Package readings gestiona la telemetría de los sensores: su validación de
// rangos, el guardado con intervalo mínimo, el resumen estadístico y la
// exportación. Es un paquete hoja: depende de los cultivos a través de una
// interfaz propia, no del paquete crops.
package readings

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Source indica por dónde llegó la lectura.
type Source string

const (
	SourceMQTT Source = "MQTT"
	SourceHTTP Source = "HTTP"
)

// Measures son los valores de una lectura. Cada sensor es opcional.
// Unidades: °C, % HR, lux, pH, ppm, hPa y % de humedad del sustrato.
type Measures struct {
	Temperature  *float64 `json:"temperature,omitempty" bson:"temperature,omitempty"`
	Humidity     *float64 `json:"humidity,omitempty" bson:"humidity,omitempty"`
	Brightness   *float64 `json:"brightness,omitempty" bson:"brightness,omitempty"`
	Ph           *float64 `json:"ph,omitempty" bson:"ph,omitempty"`
	Tds          *float64 `json:"tds,omitempty" bson:"tds,omitempty"`
	Atmosphere   *float64 `json:"atmosphere,omitempty" bson:"atmosphere,omitempty"`
	SoilMoisture *float64 `json:"soilMoisture,omitempty" bson:"soilMoisture,omitempty"`
}

// IsEmpty indica si la lectura no trae ningún valor.
func (m Measures) IsEmpty() bool {
	return m.Temperature == nil && m.Humidity == nil && m.Brightness == nil && m.Ph == nil &&
		m.Tds == nil && m.Atmosphere == nil && m.SoilMoisture == nil
}

// Reading es el documento de la colección "readings".
type Reading struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	CropID     primitive.ObjectID `bson:"cropId"`
	MeasuredAt time.Time          `bson:"measuredAt"`
	Measures   Measures           `bson:"measures"`
	Source     Source             `bson:"source,omitempty"`
}

// ReadingRecordedEvent anuncia una lectura nueva; el agente de automatización la
// evalúa. Lleva el id del cultivo (quien lo consume carga el cultivo completo).
type ReadingRecordedEvent struct {
	CropID  string
	Reading Reading
}
