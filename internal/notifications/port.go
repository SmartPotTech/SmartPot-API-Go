package notifications

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Repository es el puerto de almacenamiento de las notificaciones: lo que el servicio y los resúmenes necesitan de
// la capa de datos. Lo implementa el adaptador de PostgreSQL (internal/storage/postgres).
type Repository interface {
	Insert(ctx context.Context, n *Notification) error
	List(ctx context.Context, userID primitive.ObjectID, unreadOnly bool, limit int) ([]Notification, error)
	CountUnread(ctx context.Context, userID primitive.ObjectID) (int64, error)
	FindByIDAndUser(ctx context.Context, id, userID primitive.ObjectID) (*Notification, error)
	SetRead(ctx context.Context, id primitive.ObjectID) error
	MarkAllRead(ctx context.Context, userID primitive.ObjectID) error
	DeleteByIDAndUser(ctx context.Context, id, userID primitive.ObjectID) (int64, error)
	DeleteByUser(ctx context.Context, userID primitive.ObjectID) error
	DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error
	FindByCropCreatedAfter(ctx context.Context, cropID primitive.ObjectID, after time.Time) ([]Notification, error)
	CountByCropTypeCreatedAfter(ctx context.Context, cropID primitive.ObjectID, t Type, after time.Time) (int64, error)
	// DeleteOlderThan borra las notificaciones anteriores al corte (reemplaza al índice TTL anterior).
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}
