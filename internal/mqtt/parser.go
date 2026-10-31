package mqtt

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/commands"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/readings"
)

// CompactBinaryMagic es el byte identificador 'P' (0x50) de paquete binario compacto SmartPot.
const CompactBinaryMagic byte = 0x50

// CRC8 calcula el checksum CRC-8 ATM (polinomio 0x07) sobre el buffer.
func CRC8(data []byte) byte {
	var crc byte = 0x00
	for _, b := range data {
		crc ^= b
		for range 8 {
			if (crc & 0x80) != 0 {
				crc = (crc << 1) ^ 0x07
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// ParseCompactBinary decodifica un paquete compacto de 22 bytes con verificación CRC8.
func ParseCompactBinary(data []byte) (readings.Measures, error) {
	if len(data) != 22 {
		return readings.Measures{}, errors.New("longitud inválida para paquete compacto")
	}
	if data[0] != CompactBinaryMagic {
		return readings.Measures{}, errors.New("magic byte inválido en paquete compacto")
	}
	if CRC8(data[:21]) != data[21] {
		return readings.Measures{}, errors.New("fallo de checksum CRC-8 en paquete compacto")
	}

	rawTemp := int16(binary.BigEndian.Uint16(data[8:10]))
	temp := float64(rawTemp) / 100.0

	rawHum := binary.BigEndian.Uint16(data[10:12])
	hum := float64(rawHum) / 100.0

	rawPH := binary.BigEndian.Uint16(data[12:14])
	ph := float64(rawPH) / 100.0

	rawTDS := binary.BigEndian.Uint16(data[14:16])
	tds := float64(rawTDS)

	rawMoist := binary.BigEndian.Uint16(data[16:18])
	moist := float64(rawMoist) / 100.0

	rawBright := binary.BigEndian.Uint16(data[18:20])
	bright := float64(rawBright)

	m := readings.Measures{
		Temperature:  &temp,
		Humidity:     &hum,
		Ph:           &ph,
		Tds:          &tds,
		SoilMoisture: &moist,
		Brightness:   &bright,
	}
	if err := readings.ValidateRanges(m); err != nil {
		return readings.Measures{}, err
	}
	return m, nil
}

const maxPayload = 4096

// Pool para reutilizar mapas de deserialización y reducir la presión sobre el GC en el hot-path MQTT.
var mapPool = sync.Pool{
	New: func() any {
		return make(map[string]any, 16)
	},
}

// ParseTelemetry lee la telemetría del dispositivo (soporta formato JSON y paquete binario compacto de 22 bytes).
func ParseTelemetry(payload string) (readings.Measures, error) {
	bytesPayload := []byte(payload)
	if len(bytesPayload) == 22 && bytesPayload[0] == CompactBinaryMagic {
		return ParseCompactBinary(bytesPayload)
	}

	if strings.TrimSpace(payload) == "" || len(payload) > maxPayload {
		return readings.Measures{}, errors.New("mensaje vacío o demasiado grande")
	}

	root := mapPool.Get().(map[string]any)
	clear(root)
	defer mapPool.Put(root)

	if err := json.Unmarshal(bytesPayload, &root); err != nil {
		return readings.Measures{}, errors.New("el mensaje no es un objeto JSON válido")
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
	if strings.TrimSpace(payload) == "" || len(payload) > maxPayload {
		return commands.AckMessage{}, errors.New("mensaje vacío o demasiado grande")
	}

	root := mapPool.Get().(map[string]any)
	clear(root)
	defer mapPool.Put(root)

	if err := json.Unmarshal([]byte(payload), &root); err != nil {
		return commands.AckMessage{}, errors.New("el mensaje no es un objeto JSON válido")
	}

	status := strings.ToUpper(stringField(root, "status"))
	if status != "EXECUTED" && status != "FAILED" {
		return commands.AckMessage{}, fmt.Errorf("estado de ACK desconocido: %s", status)
	}
	return commands.AckMessage{ID: stringField(root, "id"), Status: status, Message: stringField(root, "message")}, nil
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
