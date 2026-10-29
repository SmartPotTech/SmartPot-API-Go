// Package httpserver arma el motor Echo con el mismo middleware y las mismas
// respuestas de error que la cadena de seguridad y el manejador global de la
// versión en Java.
package httpserver

import (
	"log/slog"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

// New crea el motor Echo con el middleware base. Las rutas se registran aparte.
func New(logger *slog.Logger) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Extractor seguro de IP contra spoofing vía cabeceras X-Forwarded-For / X-Real-IP (CVE-2026-118653).
	// Por defecto extrae directamente la IP de la conexión de red (RemoteAddr).
	// Si se configura TRUST_PROXY=true o SMARTPOT_TRUST_PROXY=true, se habilita la extracción
	// restringida exclusivamente a subredes privadas de proxy inverso de confianza.
	if strings.EqualFold(os.Getenv("TRUST_PROXY"), "true") || strings.EqualFold(os.Getenv("SMARTPOT_TRUST_PROXY"), "true") {
		e.IPExtractor = echo.ExtractIPFromXFFHeader(
			echo.TrustLoopback(true),
			echo.TrustLinkLocal(true),
			echo.TrustPrivateNet(true),
		)
	} else {
		e.IPExtractor = echo.ExtractIPDirect()
	}

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
