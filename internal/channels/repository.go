package channels

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// LinkRepository accede a la colección "channel_links".
type LinkRepository struct {
	coll *mongo.Collection
}

// NewLinkRepository crea el repositorio de vínculos.
func NewLinkRepository(db *mongo.Database) *LinkRepository {
	return &LinkRepository{coll: db.Collection("channel_links")}
}

// EnsureIndexes crea los índices únicos por (usuario, tipo) y (tipo, dirección).
func (r *LinkRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "type", Value: 1}}, Options: options.Index().SetName("user_type").SetUnique(true)},
		{Keys: bson.D{{Key: "type", Value: 1}, {Key: "address", Value: 1}}, Options: options.Index().SetName("type_address").SetUnique(true)},
	})
	return err
}

// FindByUser devuelve los vínculos de una cuenta.
func (r *LinkRepository) FindByUser(ctx context.Context, userID primitive.ObjectID) ([]ChannelLink, error) {
	return r.findLinks(ctx, bson.M{"userId": userID})
}

// FindByUserEnabled devuelve los vínculos activos de una cuenta.
func (r *LinkRepository) FindByUserEnabled(ctx context.Context, userID primitive.ObjectID) ([]ChannelLink, error) {
	return r.findLinks(ctx, bson.M{"userId": userID, "enabled": true})
}

// FindByIDAndUser busca un vínculo de la cuenta.
func (r *LinkRepository) FindByIDAndUser(ctx context.Context, id, userID primitive.ObjectID) (*ChannelLink, error) {
	return r.findLink(ctx, bson.M{"_id": id, "userId": userID})
}

// FindByUserAndType busca el vínculo de la cuenta con un canal.
func (r *LinkRepository) FindByUserAndType(ctx context.Context, userID primitive.ObjectID, t Type) (*ChannelLink, error) {
	return r.findLink(ctx, bson.M{"userId": userID, "type": t})
}

// FindByTypeAndAddress busca el vínculo de una dirección en un canal.
func (r *LinkRepository) FindByTypeAndAddress(ctx context.Context, t Type, address string) (*ChannelLink, error) {
	return r.findLink(ctx, bson.M{"type": t, "address": address})
}

// Save inserta o reemplaza un vínculo.
func (r *LinkRepository) Save(ctx context.Context, link *ChannelLink) error {
	if link.ID.IsZero() {
		res, err := r.coll.InsertOne(ctx, link)
		if err != nil {
			return err
		}
		if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
			link.ID = oid
		}
		return nil
	}
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": link.ID}, link)
	return err
}

// Delete borra un vínculo por id.
func (r *LinkRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// DeleteByUser borra todos los vínculos de una cuenta.
func (r *LinkRepository) DeleteByUser(ctx context.Context, userID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"userId": userID})
	return err
}

func (r *LinkRepository) findLink(ctx context.Context, filter bson.M) (*ChannelLink, error) {
	var link ChannelLink
	err := r.coll.FindOne(ctx, filter).Decode(&link)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *LinkRepository) findLinks(ctx context.Context, filter bson.M) ([]ChannelLink, error) {
	cur, err := r.coll.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]ChannelLink, 0)
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CropRepository accede a la colección "crop_channels".
type CropRepository struct {
	coll *mongo.Collection
}

// NewCropRepository crea el repositorio de avisos por cultivo.
func NewCropRepository(db *mongo.Database) *CropRepository {
	return &CropRepository{coll: db.Collection("crop_channels")}
}

// EnsureIndexes crea el índice único por cultivo y tipo.
func (r *CropRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "cropId", Value: 1}, {Key: "type", Value: 1}},
		Options: options.Index().SetName("crop_type").SetUnique(true),
	})
	return err
}

// FindByCropAndType busca los avisos de un cultivo por canal.
func (r *CropRepository) FindByCropAndType(ctx context.Context, cropID primitive.ObjectID, t Type) (*CropChannel, error) {
	var cc CropChannel
	err := r.coll.FindOne(ctx, bson.M{"cropId": cropID, "type": t}).Decode(&cc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cc, nil
}

// FindEnabled devuelve los avisos por cultivo activos (para los resúmenes).
func (r *CropRepository) FindEnabled(ctx context.Context) ([]CropChannel, error) {
	return r.findCrops(ctx, bson.M{"enabled": true})
}

// FindByTypeAndRecipientAddress devuelve los cultivos compartidos con un chat.
func (r *CropRepository) FindByTypeAndRecipientAddress(ctx context.Context, t Type, address string) ([]CropChannel, error) {
	return r.findCrops(ctx, bson.M{"type": t, "recipients.address": address})
}

// Save inserta o reemplaza los avisos de un cultivo.
func (r *CropRepository) Save(ctx context.Context, cc *CropChannel) error {
	if cc.ID.IsZero() {
		res, err := r.coll.InsertOne(ctx, cc)
		if err != nil {
			return err
		}
		if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
			cc.ID = oid
		}
		return nil
	}
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": cc.ID}, cc)
	return err
}

// DeleteByCrop borra los avisos de un cultivo.
func (r *CropRepository) DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"cropId": cropID})
	return err
}

// DeleteByOwner borra los avisos de todos los cultivos de una cuenta.
func (r *CropRepository) DeleteByOwner(ctx context.Context, ownerID primitive.ObjectID) error {
	_, err := r.coll.DeleteMany(ctx, bson.M{"ownerId": ownerID})
	return err
}

func (r *CropRepository) findCrops(ctx context.Context, filter bson.M) ([]CropChannel, error) {
	cur, err := r.coll.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]CropChannel, 0)
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
