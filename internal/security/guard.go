package security

import (
	"strings"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/labstack/echo/v4"
)

const (
	// RoleAdmin y RoleUser son los roles de la cuenta.
	RoleAdmin = "ADMIN"
	RoleUser  = "USER"
)

// Guard devuelve el middleware que exige un token JWT válido e inyecta la
// identidad y las credenciales verificadas en el RequestContext tipado en
// tiempo de compilación (sin usar mapas de strings ni conversiones any).
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
			rc := httpserver.FromContext(c)
			rc.SetIdentity(httpserver.Identity{
				UserID:        claims.UserID,
				Email:         claims.Email,
				Role:          claims.Role,
				IssuedAt:      claims.IssuedAt,
				ExpiresAt:     claims.ExpiresAt,
				Authenticated: true,
			})
			return next(rc)
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

// CurrentIdentity devuelve la estructura tipada con la identidad y credenciales JWT del usuario.
func CurrentIdentity(c echo.Context) httpserver.Identity {
	return httpserver.FromContext(c).Identity()
}

// CurrentUserID devuelve el id del usuario autenticado (vacío si no hay).
func CurrentUserID(c echo.Context) string {
	return httpserver.FromContext(c).UserID()
}

// CurrentEmail devuelve el correo del usuario autenticado.
func CurrentEmail(c echo.Context) string {
	return httpserver.FromContext(c).Email()
}

// CurrentRole devuelve el rol del usuario autenticado.
func CurrentRole(c echo.Context) string {
	return httpserver.FromContext(c).Role()
}
