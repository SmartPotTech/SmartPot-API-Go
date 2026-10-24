package security

import (
	"strings"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/gin-gonic/gin"
)

const (
	ctxUserID = "smartpot_user_id"
	ctxEmail  = "smartpot_email"
	ctxRole   = "smartpot_role"

	// RoleAdmin y RoleUser son los roles de la cuenta.
	RoleAdmin = "ADMIN"
	RoleUser  = "USER"
)

// Guard devuelve el middleware que exige un token JWT válido. Distingue, como
// la versión en Java, entre "no hay sesión" y "la sesión no es válida/expiró".
func (s *TokenService) Guard() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			_ = c.Error(apperr.Unauthorized("Inicia sesión para continuar"))
			c.Abort()
			return
		}
		claims, err := s.Parse(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			_ = c.Error(apperr.Unauthorized("Tu sesión no es válida o expiró. Inicia sesión de nuevo"))
			c.Abort()
			return
		}
		c.Set(ctxUserID, claims.UserID)
		c.Set(ctxEmail, claims.Email)
		c.Set(ctxRole, claims.Role)
		c.Next()
	}
}

// RequireRole exige un rol concreto; responde 403 como la versión en Java.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if CurrentRole(c) != role {
			_ = c.Error(apperr.Forbidden("No tienes permiso para realizar esta acción"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// CurrentUserID devuelve el id del usuario autenticado (vacío si no hay).
func CurrentUserID(c *gin.Context) string { return stringValue(c, ctxUserID) }

// CurrentEmail devuelve el correo del usuario autenticado.
func CurrentEmail(c *gin.Context) string { return stringValue(c, ctxEmail) }

// CurrentRole devuelve el rol del usuario autenticado.
func CurrentRole(c *gin.Context) string { return stringValue(c, ctxRole) }

func stringValue(c *gin.Context, key string) string {
	if v, ok := c.Get(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
