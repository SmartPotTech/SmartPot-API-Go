package security

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// CORS reproduce la configuración de CORS de la versión en Java: orígenes
// permitidos por configuración, métodos y cabeceras fijos, sin credenciales,
// con Retry-After expuesto y caché de preflight de una hora.
func CORS(allowedOrigins []string) echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Retry-After"},
		AllowCredentials: false,
		MaxAge:           int(time.Hour / time.Second),
	})
}
