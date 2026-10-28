package httpserver

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/labstack/echo/v4"
)

// requestLogger registra cada petición con su método, ruta, estado y latencia.
func requestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			req := c.Request()
			res := c.Response()
			logger.Info("peticion",
				"method", req.Method,
				"path", req.URL.Path,
				"status", res.Status,
				"ms", time.Since(start).Milliseconds(),
			)
			return err
		}
	}
}

// recovery atrapa pánicos, los registra y responde un 500 en el formato de error.
func recovery(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					req := c.Request()
					logger.Error("panico no controlado",
						"method", req.Method, "path", req.URL.Path, "panic", r)
					if !c.Response().Committed {
						writeError(c, http.StatusInternalServerError,
							"Ocurrió un error inesperado. Intenta de nuevo más tarde")
					}
				}
			}()
			return next(c)
		}
	}
}

// securityHeaders fija las mismas cabeceras que la cadena de seguridad en Java.
func securityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Referrer-Policy", "no-referrer")
			c.Response().Header().Set("X-Frame-Options", "DENY")
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")
			return next(c)
		}
	}
}

// RespondError escribe el error como respuesta JSON con su código HTTP. Un
// *apperr.Error conserva su código y mensaje; cualquier otro error es un 500.
func RespondError(c echo.Context, err error) {
	if c.Response().Committed {
		return
	}
	var apiErr *apperr.Error
	if errors.As(err, &apiErr) {
		writeError(c, apiErr.Status, apiErr.Message)
		return
	}
	var he *echo.HTTPError
	if errors.As(err, &he) {
		if he.Code == http.StatusNotFound {
			writeError(c, http.StatusNotFound, "La ruta solicitada no existe")
			return
		}
		if he.Code == http.StatusMethodNotAllowed {
			writeError(c, http.StatusMethodNotAllowed, "Método no permitido para esta ruta")
			return
		}
		msg := fmt.Sprintf("%v", he.Message)
		writeError(c, he.Code, msg)
		return
	}
	writeError(c, http.StatusInternalServerError,
		"Ocurrió un error inesperado. Intenta de nuevo más tarde")
}

func writeError(c echo.Context, status int, message string) {
	_ = c.JSON(status, apperr.NewResponse(status, message, c.Request().URL.Path))
}
