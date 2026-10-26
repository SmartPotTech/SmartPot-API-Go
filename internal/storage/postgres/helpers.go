package postgres

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// nullTime devuelve nil para un instante cero (columna NULL) o el propio instante.
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// nullEnum devuelve nil para una cadena vacía (columna de enum opcional NULL) o la propia cadena.
func nullEnum(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// hexOf devuelve el id como cadena hexadecimal de 24 caracteres (como se guarda en CHAR(24)).
func hexOf(id primitive.ObjectID) string {
	return id.Hex()
}

// hexSlice convierte una lista de ids en una lista de cadenas hexadecimales (para cláusulas = ANY($1)).
func hexSlice(ids []primitive.ObjectID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.Hex()
	}
	return out
}

// hexPtr devuelve el id como *string, o nil si el puntero es nil (para columnas opcionales).
func hexPtr(id *primitive.ObjectID) *string {
	if id == nil {
		return nil
	}
	s := id.Hex()
	return &s
}

// parseOID convierte una cadena de la base en ObjectId. CHAR(24) no rellena con espacios a 24, pero se recorta por
// seguridad.
func parseOID(s string) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(strings.TrimSpace(s))
}

// parseOIDPtr convierte un *string de la base en *ObjectId (nil si la columna era NULL).
func parseOIDPtr(s *string) (*primitive.ObjectID, error) {
	if s == nil {
		return nil, nil
	}
	oid, err := parseOID(*s)
	if err != nil {
		return nil, err
	}
	return &oid, nil
}

// jsonbArg prepara un valor para una columna JSONB. Un puntero nil (subdocumento ausente) se guarda como NULL; el
// resto se serializa con encoding/json y se envía como texto, que PostgreSQL interpreta como jsonb.
func jsonbArg(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Ptr && rv.IsNil() {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// jsonArrayArg prepara un arreglo para una columna JSONB NOT NULL: una lista nil se guarda como '[]' (no 'null').
func jsonArrayArg(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if string(b) == "null" {
		return "[]", nil
	}
	return string(b), nil
}

// scanJSONB deserializa una columna JSONB (leída como []byte) en dest. Un valor NULL o vacío no toca dest.
func scanJSONB(data []byte, dest any) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, dest)
}

// isUniqueViolation indica si el error es una violación de restricción única (código 23505 de PostgreSQL).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
