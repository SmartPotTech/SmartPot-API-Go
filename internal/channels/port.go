package channels

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LinkRepository es el puerto de almacenamiento de los vínculos de cuenta con un canal (channel_links). Lo
// implementa el adaptador de PostgreSQL (internal/storage/postgres).
type LinkRepository interface {
	FindByUser(ctx context.Context, userID primitive.ObjectID) ([]ChannelLink, error)
	FindByUserEnabled(ctx context.Context, userID primitive.ObjectID) ([]ChannelLink, error)
	FindByIDAndUser(ctx context.Context, id, userID primitive.ObjectID) (*ChannelLink, error)
	FindByUserAndType(ctx context.Context, userID primitive.ObjectID, t Type) (*ChannelLink, error)
	FindByTypeAndAddress(ctx context.Context, t Type, address string) (*ChannelLink, error)
	Save(ctx context.Context, link *ChannelLink) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	DeleteByUser(ctx context.Context, userID primitive.ObjectID) error
}

// CropRepository es el puerto de almacenamiento de los avisos de un cultivo por un canal (crop_channels). Lo
// implementa el adaptador de PostgreSQL (internal/storage/postgres).
type CropRepository interface {
	FindByCropAndType(ctx context.Context, cropID primitive.ObjectID, t Type) (*CropChannel, error)
	FindEnabled(ctx context.Context) ([]CropChannel, error)
	FindByTypeAndRecipientAddress(ctx context.Context, t Type, address string) ([]CropChannel, error)
	Save(ctx context.Context, cc *CropChannel) error
	DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error
	DeleteByOwner(ctx context.Context, ownerID primitive.ObjectID) error
}
