// Package users gestiona el perfil de la cuenta: datos, cambio de contraseña y
// borrado con todos sus datos asociados.
package users

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Role es el rol de la cuenta.
type Role string

const (
	RoleUser  Role = "USER"
	RoleAdmin Role = "ADMIN"
)

// User es el documento de la colección "users". Los nombres de campo coinciden
// con los de la versión en Java para mantener la compatibilidad del esquema.
type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Name         string             `bson:"name"`
	LastName     string             `bson:"lastName"`
	Email        string             `bson:"email"`
	PasswordHash string             `bson:"passwordHash"`
	Role         Role               `bson:"role"`
	CreatedAt    time.Time          `bson:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt"`
}

// HexID devuelve el id como cadena hexadecimal (como lo expone la API en JSON).
func (u *User) HexID() string {
	return u.ID.Hex()
}
