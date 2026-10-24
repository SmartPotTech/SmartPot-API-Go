package security

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS reproduce la configuración de CORS de la versión en Java: orígenes
// permitidos por configuración, métodos y cabeceras fijos, sin credenciales,
// con Retry-After expuesto y caché de preflight de una hora.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Retry-After"},
		AllowCredentials: false,
		MaxAge:           time.Hour,
	})
}
