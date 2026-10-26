package virtualdevices

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Repository es el puerto de almacenamiento de las simulaciones de los cultivos virtuales. Lo implementa el
// adaptador de PostgreSQL (internal/storage/postgres).
type Repository interface {
	FindByCropID(ctx context.Context, cropID primitive.ObjectID) (*VirtualDevice, error)
	CountByOwner(ctx context.Context, ownerID primitive.ObjectID) (int64, error)
	FindAll(ctx context.Context) ([]VirtualDevice, error)
	Save(ctx context.Context, v *VirtualDevice) error
	DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error
}
