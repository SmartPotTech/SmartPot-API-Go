// Package httpserver arma el motor Gin con el mismo middleware y las mismas
// respuestas de error que la cadena de seguridad y el manejador global de la
// versión en Java.
package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/gin-gonic/gin"
)

// New crea el motor Gin con el middleware base. Las rutas se registran aparte.
func New(logger *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.HandleMethodNotAllowed = true
	e.Use(requestLogger(logger), recovery(logger), securityHeaders(), errorHandler())
	e.NoRoute(func(c *gin.Context) {
		RespondError(c, apperr.NotFound("La ruta solicitada no existe"))
	})
	e.NoMethod(func(c *gin.Context) {
		RespondError(c, apperr.New(http.StatusMethodNotAllowed, "Método no permitido para esta ruta"))
	})
	return e
}
