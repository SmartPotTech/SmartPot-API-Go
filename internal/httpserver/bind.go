package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/validation"
	"github.com/labstack/echo/v4"
)

// Validatable es una solicitud que sabe validarse. Devuelve el mapa de errores
// por campo (vacío si es válida).
type Validatable interface {
	Validate() validation.Fields
}

// DecodeJSON decodifica el cuerpo JSON en dst sin validarlo. Si el JSON es
// inválido responde 400 y devuelve false (el llamador debe detenerse).
func DecodeJSON(c echo.Context, dst any) bool {
	if err := json.NewDecoder(c.Request().Body).Decode(dst); err != nil {
		writeError(c, http.StatusBadRequest, "El cuerpo de la solicitud no es un JSON válido")
		return false
	}
	return true
}

// Bind decodifica el cuerpo JSON en req y lo valida. Si el JSON es inválido
// responde 400 con el mismo mensaje que la versión en Java; si falla la
// validación responde 400 con el detalle por campo. Devuelve false si ya
// respondió (el llamador debe detenerse).
func Bind(c echo.Context, req Validatable) bool {
	decoder := json.NewDecoder(c.Request().Body)
	if err := decoder.Decode(req); err != nil {
		writeError(c, http.StatusBadRequest, "El cuerpo de la solicitud no es un JSON válido")
		return false
	}
	if fields := req.Validate(); len(fields) > 0 {
		_ = c.JSON(http.StatusBadRequest,
			apperr.NewValidationResponse("Revisa los datos enviados", c.Request().URL.Path, fields))
		return false
	}
	return true
}
