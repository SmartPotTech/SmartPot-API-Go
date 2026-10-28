// Package httpserver arma el motor Echo con el mismo middleware y las mismas
// respuestas de error que la cadena de seguridad y el manejador global de la
// versión en Java.
package httpserver

import (
	"log/slog"

	"github.com/labstack/echo/v4"
)

// New crea el motor Echo con el middleware base. Las rutas se registran aparte.
func New(logger *slog.Logger) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Extractor seguro de IP confiando en X-Forwarded-For de redes internas
	e.IPExtractor = echo.ExtractIPFromXFFHeader()

	// Validador intercambiable desacoplado de bibliotecas externas de etiquetas
	e.Validator = NewValidator()

	// Manejador centralizado de errores: transforma errores de dominio, validación y HTTP en respuestas homogéneas
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		RespondError(c, err)
	}

	// Cadena de middlewares por composición de funciones lineales
	e.Use(contextMiddleware(), requestLogger(logger), recovery(logger), securityHeaders(), etagMiddleware())

	return e
}
