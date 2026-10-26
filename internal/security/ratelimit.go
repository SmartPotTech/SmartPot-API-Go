package security

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/cache"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/gin-gonic/gin"
)

// RateLimiter aplica una ventana fija de un minuto por IP. Las rutas de
// autenticación tienen un cupo menor. OPTIONS y /health quedan exentos.
type RateLimiter struct {
	store        cache.Cache
	generalLimit int
	authLimit    int
	now          func() time.Time
}

// NewRateLimiter crea el limitador con los cupos general y de autenticación.
func NewRateLimiter(store cache.Cache, generalLimit, authLimit int) *RateLimiter {
	return &RateLimiter{store: store, generalLimit: generalLimit, authLimit: authLimit, now: time.Now}
}

// Middleware devuelve el middleware Gin del limitador.
func (r *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if c.Request.Method == http.MethodOptions || path == "/health" {
			c.Next()
			return
		}
		authRoute := strings.HasPrefix(path, "/api/v1/auth/")
		limit := r.generalLimit
		scope := "all:"
		if authRoute {
			limit = r.authLimit
			scope = "auth:"
		}
		epoch := r.now().Unix()
		key := fmt.Sprintf("rate:%s%s:%d", scope, c.ClientIP(), epoch/60)

		if r.store.Increment(key, time.Minute) > int64(limit) {
			retryAfter := 60 - epoch%60
			c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
			_ = c.Error(apperr.New(http.StatusTooManyRequests,
				fmt.Sprintf("Enviaste demasiadas solicitudes. Intenta de nuevo en %d segundos", retryAfter)))
			c.Abort()
			return
		}
		c.Next()
	}
}
