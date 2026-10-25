package mqtt

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/commands"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/readings"
)

const maxPayload = 4096

// ParseTelemetry lee la telemetría del dispositivo. Acepta números o textos
// numéricos, ignora campos desconocidos y valida los rangos físicos.
func ParseTelemetry(payload string) (readings.Measures, error) {
	root, err := readObject(payload)
	if err != nil {
		return readings.Measures{}, err
	}
	m := readings.Measures{}
	for field, target := range map[string]**float64{
		"temperature":  &m.Temperature,
		"humidity":     &m.Humidity,
		"brightness":   &m.Brightness,
		"ph":           &m.Ph,
		"tds":          &m.Tds,
		"atmosphere":   &m.Atmosphere,
		"soilMoisture": &m.SoilMoisture,
	} {
		value, err := number(root, field)
		if err != nil {
			return readings.Measures{}, err
		}
		*target = value
	}
	if err := readings.ValidateRanges(m); err != nil {
		return readings.Measures{}, err
	}
	return m, nil
}

// ParseAck lee la confirmación del dispositivo. status: EXECUTED o FAILED.
func ParseAck(payload string) (commands.AckMessage, error) {
	root, err := readObject(payload)
	if err != nil {
		return commands.AckMessage{}, err
	}
	status := strings.ToUpper(stringField(root, "status"))
	if status != "EXECUTED" && status != "FAILED" {
		return commands.AckMessage{}, fmt.Errorf("estado de ACK desconocido: %s", status)
	}
	return commands.AckMessage{ID: stringField(root, "id"), Status: status, Message: stringField(root, "message")}, nil
}

func readObject(payload string) (map[string]any, error) {
	if strings.TrimSpace(payload) == "" || len(payload) > maxPayload {
		return nil, errors.New("mensaje vacío o demasiado grande")
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(payload), &root); err != nil {
		return nil, errors.New("el mensaje no es un objeto JSON válido")
	}
	return root, nil
}

func number(root map[string]any, field string) (*float64, error) {
	value, ok := root[field]
	if !ok || value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case float64:
		return &v, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return nil, fmt.Errorf("el campo %s no es numérico", field)
		}
		return &f, nil
	default:
		return nil, fmt.Errorf("el campo %s no es numérico", field)
	}
}

func stringField(root map[string]any, field string) string {
	if v, ok := root[field].(string); ok {
		return v
	}
	return ""
}
