package notifications

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository accede a la colección "notifications".
type Repository struct {
	coll *mongo.Collection
}

// NewRepository crea el repositorio sobre la base dada.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("notifications")}
}

// EnsureIndexes crea el índice compuesto y el TTL de 90 días sobre createdAt.
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("user_created"),
		},
		{
			Keys:    bson.D{{Key: "createdAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(90 * 24 * 60 * 60),
		},
	})
	return err
}

// Insert guarda una notificación y fija su id generado.
func (r *Repository) Insert(ctx context.Context, n *Notification) error {
	res, err := r.coll.InsertOne(ctx, n)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		n.ID = oid
	}
	return nil
}

// List devuelve las notificaciones del usuario, más recientes primero.
func (r *Repository) List(ctx context.Context, userID primitive.ObjectID, unreadOnly bool, limit int) ([]Notification, error) {
	filter := bson.M{"userId": userID}
	if unreadOnly {
		filter["read"] = false
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit))
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	return decodeAll(ctx, cur)
}

// CountUnread cuenta las notificaciones sin leer del usuario.
func (r *Repository) CountUnread(ctx context.Context, userID primitive.ObjectID) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"userId": userID, "read": false})
}

// FindByIDAndUser busca una notificación del usuario.
func (r *Repository) FindByIDAndUser(ctx context.Context, id, userID primitive.ObjectID) (*Notification, error) {
	var n Notification
	err := r.coll.FindOne(ctx, bson.M{"_id": id, "userId": userID}).Decode(&n)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// SetRead marca una notificación como leída.
func (r *Repository) SetRead(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.coll.UpdateByID(ctx, id, bson.M{"$set": bson.M{"read": true}})
	return err
}

// MarkAllRead marca como leídas todas las del usuario.
func (r *Repository) MarkAllRead(ctx context.Context, userID primitive.ObjectID) error {
	_, err := r.coll.UpdateMany(ctx, bson.M{"userId": userID, "read": false}, bson.M{"$set": bson.M{"read": true}})
	return err
}

// DeleteByIDAndUser borra una notificación del usuario; devuelve cuántas borró.
func (r *Repository) DeleteByIDAndUser(ctx context.Context, id, userID primitive.ObjectID) (int64, error) {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id, "userId": userID})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// DeleteByUser borra todas las notificaciones del usuario.
func (r *Repository) DeleteByUser(ctx context.Context, userID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"userId": userID})
	return err
}

// DeleteByCrop borra todas las notificaciones de un cultivo.
func (r *Repository) DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"cropId": cropID})
	return err
}

// FindByCropCreatedAfter devuelve las notificaciones de un cultivo posteriores a
// un instante, más antiguas primero (para los resúmenes por cultivo).
func (r *Repository) FindByCropCreatedAfter(ctx context.Context, cropID primitive.ObjectID, after time.Time) ([]Notification, error) {
	filter := bson.M{"cropId": cropID, "createdAt": bson.M{"$gt": after}}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	return decodeAll(ctx, cur)
}

// CountByCropTypeCreatedAfter cuenta por cultivo, tipo e instante.
func (r *Repository) CountByCropTypeCreatedAfter(ctx context.Context, cropID primitive.ObjectID, t Type, after time.Time) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"cropId": cropID, "type": t, "createdAt": bson.M{"$gt": after}})
}

func decodeAll(ctx context.Context, cur *mongo.Cursor) ([]Notification, error) {
	defer cur.Close(ctx)
	out := make([]Notification, 0)
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
