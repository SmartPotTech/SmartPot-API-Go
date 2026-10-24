package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/validation"
	"github.com/gin-gonic/gin"
)

// Validatable es una solicitud que sabe validarse. Devuelve el mapa de errores
// por campo (vacío si es válida).
type Validatable interface {
	Validate() validation.Fields
}

// Bind decodifica el cuerpo JSON en req y lo valida. Si el JSON es inválido
// responde 400 con el mismo mensaje que la versión en Java; si falla la
// validación responde 400 con el detalle por campo. Devuelve false si ya
// respondió (el llamador debe detenerse).
func Bind(c *gin.Context, req Validatable) bool {
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(req); err != nil {
		writeError(c, http.StatusBadRequest, "El cuerpo de la solicitud no es un JSON válido")
		return false
	}
	if fields := req.Validate(); len(fields) > 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest,
			apperr.NewValidationResponse("Revisa los datos enviados", c.Request.URL.Path, fields))
		return false
	}
	return true
}
