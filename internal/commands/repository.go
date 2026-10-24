package commands

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository accede a la colección "commands".
type Repository struct {
	coll *mongo.Collection
}

// NewRepository crea el repositorio sobre la base dada.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("commands")}
}

// EnsureIndexes crea los índices compuestos y el TTL de 180 días sobre createdAt.
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "cropId", Value: 1}, {Key: "createdAt", Value: -1}}, Options: options.Index().SetName("crop_created")},
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "sentAt", Value: 1}}, Options: options.Index().SetName("status_sent")},
		{Keys: bson.D{{Key: "createdAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(180 * 24 * 60 * 60)},
	})
	return err
}

// Insert guarda un comando nuevo y fija su id generado.
func (r *Repository) Insert(ctx context.Context, c *Command) error {
	res, err := r.coll.InsertOne(ctx, c)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		c.ID = oid
	}
	return nil
}

// Save reemplaza el documento del comando.
func (r *Repository) Save(ctx context.Context, c *Command) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": c.ID}, c)
	return err
}

// ListByCrop devuelve el historial del cultivo, más recientes primero.
func (r *Repository) ListByCrop(ctx context.Context, cropID primitive.ObjectID, limit int) ([]Command, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit))
	return r.find(ctx, bson.M{"cropId": cropID}, opts)
}

// ListByCrops devuelve el historial de varios cultivos, más recientes primero.
func (r *Repository) ListByCrops(ctx context.Context, cropIDs []primitive.ObjectID, limit int) ([]Command, error) {
	if len(cropIDs) == 0 {
		return nil, nil
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit))
	return r.find(ctx, bson.M{"cropId": bson.M{"$in": cropIDs}}, opts)
}

// CountByCropsSince cuenta los comandos de varios cultivos desde un instante.
func (r *Repository) CountByCropsSince(ctx context.Context, cropIDs []primitive.ObjectID, since time.Time) (int64, error) {
	if len(cropIDs) == 0 {
		return 0, nil
	}
	return r.coll.CountDocuments(ctx, bson.M{"cropId": bson.M{"$in": cropIDs}, "createdAt": bson.M{"$gt": since}})
}

// FindByIDAndCrop busca un comando del cultivo.
func (r *Repository) FindByIDAndCrop(ctx context.Context, id, cropID primitive.ObjectID) (*Command, error) {
	var c Command
	err := r.coll.FindOne(ctx, bson.M{"_id": id, "cropId": cropID}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// FindStaleSent devuelve los comandos SENT enviados antes del límite.
func (r *Repository) FindStaleSent(ctx context.Context, before time.Time) ([]Command, error) {
	filter := bson.M{"status": StatusSent, "sentAt": bson.M{"$lt": before}}
	return r.find(ctx, filter, nil)
}

// ExistsInFlight indica si el actuador tiene un comando PENDING o SENT.
func (r *Repository) ExistsInFlight(ctx context.Context, actuatorID primitive.ObjectID) (bool, error) {
	err := r.coll.FindOne(ctx, bson.M{
		"actuatorId": actuatorID,
		"status":     bson.M{"$in": []Status{StatusPending, StatusSent}},
	}, options.FindOne().SetProjection(bson.M{"_id": 1})).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// DeleteByCrop borra los comandos de un cultivo.
func (r *Repository) DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"cropId": cropID})
	return err
}

func (r *Repository) find(ctx context.Context, filter any, opts *options.FindOptions) ([]Command, error) {
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	out := make([]Command, 0)
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
