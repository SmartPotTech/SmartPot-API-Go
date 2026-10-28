package security

import (
	"strings"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/labstack/echo/v4"
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
func (s *TokenService) Guard() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				return apperr.Unauthorized("Inicia sesión para continuar")
			}
			claims, err := s.Parse(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
			if err != nil {
				return apperr.Unauthorized("Tu sesión no es válida o expiró. Inicia sesión de nuevo")
			}
			c.Set(ctxUserID, claims.UserID)
			c.Set(ctxEmail, claims.Email)
			c.Set(ctxRole, claims.Role)
			return next(c)
		}
	}
}

// RequireRole exige un rol concreto; responde 403 como la versión en Java.
func RequireRole(role string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if CurrentRole(c) != role {
				return apperr.Forbidden("No tienes permiso para realizar esta acción")
			}
			return next(c)
		}
	}
}

// CurrentUserID devuelve el id del usuario autenticado (vacío si no hay).
func CurrentUserID(c echo.Context) string { return stringValue(c, ctxUserID) }

// CurrentEmail devuelve el correo del usuario autenticado.
func CurrentEmail(c echo.Context) string { return stringValue(c, ctxEmail) }

// CurrentRole devuelve el rol del usuario autenticado.
func CurrentRole(c echo.Context) string { return stringValue(c, ctxRole) }

func stringValue(c echo.Context, key string) string {
	if v := c.Get(key); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
