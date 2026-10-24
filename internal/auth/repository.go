package auth

import (
	"context"
	"errors"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/objectid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TokenRepository accede a la colección "password_reset_tokens".
type TokenRepository struct {
	coll *mongo.Collection
}

// NewTokenRepository crea el repositorio sobre la base dada.
func NewTokenRepository(db *mongo.Database) *TokenRepository {
	return &TokenRepository{coll: db.Collection("password_reset_tokens")}
}

// EnsureIndexes crea el índice único de tokenHash, el de userId y el TTL que
// borra el documento al llegar a expiresAt.
func (r *TokenRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "tokenHash", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "userId", Value: 1}}},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	return err
}

// FindByTokenHash busca un token por su hash, o nil si no existe.
func (r *TokenRepository) FindByTokenHash(ctx context.Context, hash string) (*PasswordResetToken, error) {
	var t PasswordResetToken
	err := r.coll.FindOne(ctx, bson.M{"tokenHash": hash}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Insert guarda un token nuevo.
func (r *TokenRepository) Insert(ctx context.Context, t *PasswordResetToken) error {
	_, err := r.coll.InsertOne(ctx, t)
	return err
}

// DeleteByUserID borra los tokens de un usuario (también sirve de borrador de
// cuenta en el borrado en cascada).
func (r *TokenRepository) DeleteByUserID(ctx context.Context, userID string) error {
	oid, ok := objectid.Parse(userID)
	if !ok {
		return nil
	}
	_, err := r.coll.DeleteMany(ctx, bson.M{"userId": oid})
	return err
}
