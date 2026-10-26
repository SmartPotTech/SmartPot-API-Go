package commands

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Repository es el puerto de almacenamiento de las órdenes: lo que el servicio necesita de la capa de datos. Lo
// implementa el adaptador de PostgreSQL (internal/storage/postgres).
type Repository interface {
	Insert(ctx context.Context, c *Command) error
	Save(ctx context.Context, c *Command) error
	ListByCrop(ctx context.Context, cropID primitive.ObjectID, limit int) ([]Command, error)
	ListByCrops(ctx context.Context, cropIDs []primitive.ObjectID, limit int) ([]Command, error)
	CountByCropsSince(ctx context.Context, cropIDs []primitive.ObjectID, since time.Time) (int64, error)
	FindByIDAndCrop(ctx context.Context, id, cropID primitive.ObjectID) (*Command, error)
	FindStaleSent(ctx context.Context, before time.Time) ([]Command, error)
	ExistsInFlight(ctx context.Context, actuatorID primitive.ObjectID) (bool, error)
	DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error
	// DeleteOlderThan borra las órdenes anteriores al corte (reemplaza al índice TTL anterior).
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}
