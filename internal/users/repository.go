package users

import (
	"context"
	"errors"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/objectid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository accede a la colección "users".
type Repository struct {
	coll *mongo.Collection
}

// NewRepository crea el repositorio sobre la base dada.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("users")}
}

// EnsureIndexes crea el índice único de correo (como @Indexed(unique=true)).
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

// FindByEmail devuelve el usuario con ese correo, o nil si no existe.
func (r *Repository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.coll.FindOne(ctx, bson.M{"email": email}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ExistsByEmail indica si ya hay una cuenta con ese correo.
func (r *Repository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	err := r.coll.FindOne(ctx, bson.M{"email": email},
		options.FindOne().SetProjection(bson.M{"_id": 1})).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// FindByID devuelve el usuario por id, o nil si el id es inválido o no existe.
func (r *Repository) FindByID(ctx context.Context, id string) (*User, error) {
	oid, ok := objectid.Parse(id)
	if !ok {
		return nil, nil
	}
	var u User
	err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Insert guarda un usuario nuevo y fija su id generado.
func (r *Repository) Insert(ctx context.Context, u *User) error {
	res, err := r.coll.InsertOne(ctx, u)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		u.ID = oid
	}
	return nil
}

// Update reemplaza el documento del usuario.
func (r *Repository) Update(ctx context.Context, u *User) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": u.ID}, u)
	return err
}

// Delete borra el usuario por id.
func (r *Repository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// IsDuplicate indica si el error es por clave duplicada (correo ya existente).
func IsDuplicate(err error) bool {
	return mongo.IsDuplicateKeyError(err)
}
