package auth

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PasswordResetToken guarda solo el SHA-256 del token; Mongo borra el documento
// al vencer (índice TTL sobre expiresAt). Colección "password_reset_tokens".
type PasswordResetToken struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	TokenHash string             `bson:"tokenHash"`
	UserID    primitive.ObjectID `bson:"userId"`
	ExpiresAt time.Time          `bson:"expiresAt"`
	CreatedAt time.Time          `bson:"createdAt"`
}
