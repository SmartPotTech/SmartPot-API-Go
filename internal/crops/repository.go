package crops

import (
	"context"
	"errors"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/geo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository accede a la colección "crops".
type Repository struct {
	coll *mongo.Collection
}

// NewRepository crea el repositorio sobre la base dada.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("crops")}
}

// EnsureIndexes crea el índice compuesto por dueño y fecha de creación.
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "ownerId", Value: 1}, {Key: "createdAt", Value: -1}},
		Options: options.Index().SetName("owner_created"),
	})
	return err
}

// ListByOwner devuelve los cultivos del dueño, más antiguos primero.
func (r *Repository) ListByOwner(ctx context.Context, ownerID primitive.ObjectID) ([]Crop, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}})
	cur, err := r.coll.Find(ctx, bson.M{"ownerId": ownerID}, opts)
	if err != nil {
		return nil, err
	}
	out := make([]Crop, 0)
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindByIDAndOwner busca un cultivo del dueño.
func (r *Repository) FindByIDAndOwner(ctx context.Context, id, ownerID primitive.ObjectID) (*Crop, error) {
	return r.findOne(ctx, bson.M{"_id": id, "ownerId": ownerID})
}

// FindByID busca un cultivo por id.
func (r *Repository) FindByID(ctx context.Context, id primitive.ObjectID) (*Crop, error) {
	return r.findOne(ctx, bson.M{"_id": id})
}

// CountByOwner cuenta los cultivos del dueño.
func (r *Repository) CountByOwner(ctx context.Context, ownerID primitive.ObjectID) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"ownerId": ownerID})
}

// Insert guarda un cultivo nuevo y fija su id generado.
func (r *Repository) Insert(ctx context.Context, crop *Crop) error {
	res, err := r.coll.InsertOne(ctx, crop)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		crop.ID = oid
	}
	return nil
}

// Save reemplaza el documento del cultivo.
func (r *Repository) Save(ctx context.Context, crop *Crop) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": crop.ID}, crop)
	return err
}

// Delete borra el cultivo por id.
func (r *Repository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// UpdatePlacement cambia solo el lugar y la fecha de actualización.
func (r *Repository) UpdatePlacement(ctx context.Context, id primitive.ObjectID, placement *geo.Placement, now time.Time) error {
	_, err := r.coll.UpdateByID(ctx, id, bson.M{"$set": bson.M{"placement": placement, "updatedAt": now}})
	return err
}

// MarkDeviceSeen actualiza solo el estado del dispositivo, sin pisar otros cambios.
func (r *Repository) MarkDeviceSeen(ctx context.Context, id primitive.ObjectID, online *bool, now time.Time) error {
	set := bson.M{"device.lastSeenAt": now}
	if online != nil {
		set["device.online"] = *online
	}
	_, err := r.coll.UpdateByID(ctx, id, bson.M{"$set": set})
	return err
}

// UpdateHealth actualiza solo la salud del cultivo.
func (r *Repository) UpdateHealth(ctx context.Context, id primitive.ObjectID, health *CropHealth) error {
	_, err := r.coll.UpdateByID(ctx, id, bson.M{"$set": bson.M{"health": health}})
	return err
}

func (r *Repository) findOne(ctx context.Context, filter bson.M) (*Crop, error) {
	var crop Crop
	err := r.coll.FindOne(ctx, filter).Decode(&crop)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &crop, nil
}
