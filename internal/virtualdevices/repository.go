package virtualdevices

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository accede a la colección "virtual_devices".
type Repository struct {
	coll *mongo.Collection
}

// NewRepository crea el repositorio sobre la base dada.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("virtual_devices")}
}

// EnsureIndexes crea el índice único por cultivo y el índice por dueño.
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "cropId", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "ownerId", Value: 1}}},
	})
	return err
}

// FindByCropID busca la simulación de un cultivo, o nil si no existe.
func (r *Repository) FindByCropID(ctx context.Context, cropID primitive.ObjectID) (*VirtualDevice, error) {
	var v VirtualDevice
	err := r.coll.FindOne(ctx, bson.M{"cropId": cropID}).Decode(&v)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// CountByOwner cuenta las simulaciones de una cuenta.
func (r *Repository) CountByOwner(ctx context.Context, ownerID primitive.ObjectID) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"ownerId": ownerID})
}

// FindAll devuelve todas las simulaciones (para la reconciliación).
func (r *Repository) FindAll(ctx context.Context) ([]VirtualDevice, error) {
	cur, err := r.coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	out := make([]VirtualDevice, 0)
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Save inserta o reemplaza la simulación.
func (r *Repository) Save(ctx context.Context, v *VirtualDevice) error {
	if v.ID.IsZero() {
		res, err := r.coll.InsertOne(ctx, v)
		if err != nil {
			return err
		}
		if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
			v.ID = oid
		}
		return nil
	}
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": v.ID}, v)
	return err
}

// DeleteByCrop borra la simulación de un cultivo.
func (r *Repository) DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"cropId": cropID})
	return err
}
