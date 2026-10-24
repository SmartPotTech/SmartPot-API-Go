package actuators

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository accede a la colección "actuators".
type Repository struct {
	coll *mongo.Collection
}

// NewRepository crea el repositorio sobre la base dada.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("actuators")}
}

// EnsureIndexes crea el índice único por cultivo y tipo.
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "cropId", Value: 1}, {Key: "type", Value: 1}},
		Options: options.Index().SetName("crop_type").SetUnique(true),
	})
	return err
}

// ListByCrop devuelve los actuadores del cultivo, más antiguos primero.
func (r *Repository) ListByCrop(ctx context.Context, cropID primitive.ObjectID) ([]Actuator, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}})
	cur, err := r.coll.Find(ctx, bson.M{"cropId": cropID}, opts)
	if err != nil {
		return nil, err
	}
	out := make([]Actuator, 0)
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindByIDAndCrop busca un actuador del cultivo.
func (r *Repository) FindByIDAndCrop(ctx context.Context, id, cropID primitive.ObjectID) (*Actuator, error) {
	return r.findOne(ctx, bson.M{"_id": id, "cropId": cropID})
}

// FindByCropAndType busca un actuador del cultivo por tipo.
func (r *Repository) FindByCropAndType(ctx context.Context, cropID primitive.ObjectID, t Type) (*Actuator, error) {
	return r.findOne(ctx, bson.M{"cropId": cropID, "type": t})
}

// ExistsByCropAndType indica si el cultivo ya tiene un actuador de ese tipo.
func (r *Repository) ExistsByCropAndType(ctx context.Context, cropID primitive.ObjectID, t Type) (bool, error) {
	err := r.coll.FindOne(ctx, bson.M{"cropId": cropID, "type": t},
		options.FindOne().SetProjection(bson.M{"_id": 1})).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Insert guarda un actuador nuevo y fija su id generado.
func (r *Repository) Insert(ctx context.Context, a *Actuator) error {
	res, err := r.coll.InsertOne(ctx, a)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		a.ID = oid
	}
	return nil
}

// UpdateState fija el estado confirmado del actuador.
func (r *Repository) UpdateState(ctx context.Context, id primitive.ObjectID, active bool, runningUntil *time.Time, now time.Time) error {
	update := bson.M{"$set": bson.M{"active": active, "lastChangedAt": now}}
	if runningUntil != nil {
		update["$set"].(bson.M)["runningUntil"] = *runningUntil
	} else {
		update["$unset"] = bson.M{"runningUntil": ""}
	}
	_, err := r.coll.UpdateByID(ctx, id, update)
	return err
}

// Delete borra un actuador por id.
func (r *Repository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// DeleteByCrop borra todos los actuadores de un cultivo.
func (r *Repository) DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"cropId": cropID})
	return err
}

func (r *Repository) findOne(ctx context.Context, filter bson.M) (*Actuator, error) {
	var a Actuator
	err := r.coll.FindOne(ctx, filter).Decode(&a)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
