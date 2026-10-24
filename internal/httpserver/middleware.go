package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/gin-gonic/gin"
)

// requestLogger registra cada petición con su método, ruta, estado y latencia.
func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("peticion",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"ms", time.Since(start).Milliseconds(),
		)
	}
}

// recovery atrapa pánicos, los registra y responde un 500 en el formato de error.
func recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panico no controlado",
					"method", c.Request.Method, "path", c.Request.URL.Path, "panic", r)
				if !c.Writer.Written() {
					writeError(c, http.StatusInternalServerError,
						"Ocurrió un error inesperado. Intenta de nuevo más tarde")
				}
			}
		}()
		c.Next()
	}
}

// securityHeaders fija las mismas cabeceras que la cadena de seguridad en Java.
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Frame-Options", "DENY")
		c.Next()
	}
}

// errorHandler traduce los errores acumulados en el contexto a una respuesta JSON.
func errorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		RespondError(c, c.Errors.Last().Err)
	}
}

// RespondError escribe el error como respuesta JSON con su código HTTP. Un
// *apperr.Error conserva su código y mensaje; cualquier otro error es un 500.
func RespondError(c *gin.Context, err error) {
	var apiErr *apperr.Error
	if errors.As(err, &apiErr) {
		writeError(c, apiErr.Status, apiErr.Message)
		return
	}
	writeError(c, http.StatusInternalServerError,
		"Ocurrió un error inesperado. Intenta de nuevo más tarde")
}

func writeError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, apperr.NewResponse(status, message, c.Request.URL.Path))
}
