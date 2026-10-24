// Package objectid valida identificadores de MongoDB (ObjectId) para no revelar
// qué ids existen: un id mal formado se trata como "no encontrado", igual que en
// la versión en Java.
package objectid

import (
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/apperr"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Valid indica si el id es un ObjectId hexadecimal válido (24 caracteres).
func Valid(id string) bool {
	_, err := primitive.ObjectIDFromHex(id)
	return err == nil
}

// Require devuelve un error 404 con el mensaje dado si el id no es válido.
func Require(id, notFoundMessage string) error {
	if !Valid(id) {
		return apperr.NotFound(notFoundMessage)
	}
	return nil
}

// Parse convierte el id a ObjectId; el segundo valor indica si era válido.
func Parse(id string) (primitive.ObjectID, bool) {
	oid, err := primitive.ObjectIDFromHex(id)
	return oid, err == nil
}
