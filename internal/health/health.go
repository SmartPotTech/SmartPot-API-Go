// Package health expone /health para balanceadores y despliegues. Solo la base
// de datos hace fallar el chequeo (503); sin broker, caché, IA o simulador la
// API sigue atendiendo con funciones reducidas.
package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Checker reúne las sondas de cada dependencia. Los componentes que todavía no
// existen se inyectan como nil y se informan como DOWN o DISABLED.
type Checker struct {
	Database  func() bool // true = UP (controla el 503)
	Broker    func() (enabled, up bool)
	Cache     func() bool // true = UP
	AI        func() (enabled, healthy bool)
	Simulator func() (available, healthy bool)
}

type response struct {
	Status    string `json:"status"`
	Database  string `json:"database"`
	Broker    string `json:"broker"`
	Cache     string `json:"cache"`
	AI        string `json:"ai"`
	Simulator string `json:"simulator"`
}

// Handler devuelve el manejador Gin de /health.
func (h Checker) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		database := call(h.Database)

		body := response{
			Status:    upDown(database),
			Database:  upDown(database),
			Broker:    gated(h.Broker),
			Cache:     upDown(call(h.Cache)),
			AI:        gated(h.AI),
			Simulator: gated(h.Simulator),
		}

		status := http.StatusOK
		if !database {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, body)
	}
}

func call(probe func() bool) bool {
	return probe != nil && probe()
}

// gated resuelve el estado de un componente opcional: DISABLED si no está
// habilitado/disponible, si no UP o DOWN según su salud.
func gated(probe func() (bool, bool)) string {
	if probe == nil {
		return "DISABLED"
	}
	enabled, up := probe()
	if !enabled {
		return "DISABLED"
	}
	return upDown(up)
}

func upDown(up bool) string {
	if up {
		return "UP"
	}
	return "DOWN"
}
