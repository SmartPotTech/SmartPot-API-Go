package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/channels"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ChannelLinkRepository implementa channels.LinkRepository sobre PostgreSQL (tabla channel_links).
type ChannelLinkRepository struct {
	pool *pgxpool.Pool
}

// NewChannelLinkRepository crea el adaptador de vínculos de canal.
func NewChannelLinkRepository(pool *pgxpool.Pool) *ChannelLinkRepository {
	return &ChannelLinkRepository{pool: pool}
}

const channelLinkColumns = "id, user_id, type, address, display_name, enabled, events, linked_at, last_delivered_at, failures"

func scanChannelLink(row pgx.Row) (*channels.ChannelLink, error) {
	var (
		l          channels.ChannelLink
		id, userID string
		typ        string
		eventsRaw  []byte
	)
	if err := row.Scan(&id, &userID, &typ, &l.Address, &l.DisplayName, &l.Enabled, &eventsRaw,
		&l.LinkedAt, &l.LastDeliveredAt, &l.Failures); err != nil {
		return nil, err
	}
	var err error
	if l.ID, err = parseOID(id); err != nil {
		return nil, err
	}
	if l.UserID, err = parseOID(userID); err != nil {
		return nil, err
	}
	l.Type = channels.Type(typ)
	if err := scanJSONB(eventsRaw, &l.Events); err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *ChannelLinkRepository) queryLinks(ctx context.Context, where string, args ...any) ([]channels.ChannelLink, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+channelLinkColumns+" FROM channel_links WHERE "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]channels.ChannelLink, 0)
	for rows.Next() {
		l, err := scanChannelLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	return out, rows.Err()
}

func (r *ChannelLinkRepository) findLink(ctx context.Context, where string, args ...any) (*channels.ChannelLink, error) {
	l, err := scanChannelLink(r.pool.QueryRow(ctx, "SELECT "+channelLinkColumns+" FROM channel_links WHERE "+where, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return l, err
}

// FindByUser devuelve los vínculos de una cuenta.
func (r *ChannelLinkRepository) FindByUser(ctx context.Context, userID primitive.ObjectID) ([]channels.ChannelLink, error) {
	return r.queryLinks(ctx, "user_id = $1", hexOf(userID))
}

// FindByUserEnabled devuelve los vínculos activos de una cuenta.
func (r *ChannelLinkRepository) FindByUserEnabled(ctx context.Context, userID primitive.ObjectID) ([]channels.ChannelLink, error) {
	return r.queryLinks(ctx, "user_id = $1 AND enabled = true", hexOf(userID))
}

// FindByIDAndUser busca un vínculo de la cuenta.
func (r *ChannelLinkRepository) FindByIDAndUser(ctx context.Context, id, userID primitive.ObjectID) (*channels.ChannelLink, error) {
	return r.findLink(ctx, "id = $1 AND user_id = $2", hexOf(id), hexOf(userID))
}

// FindByUserAndType busca el vínculo de la cuenta con un canal.
func (r *ChannelLinkRepository) FindByUserAndType(ctx context.Context, userID primitive.ObjectID, t channels.Type) (*channels.ChannelLink, error) {
	return r.findLink(ctx, "user_id = $1 AND type = $2", hexOf(userID), string(t))
}

// FindByTypeAndAddress busca el vínculo de una dirección en un canal.
func (r *ChannelLinkRepository) FindByTypeAndAddress(ctx context.Context, t channels.Type, address string) (*channels.ChannelLink, error) {
	return r.findLink(ctx, "type = $1 AND address = $2", string(t), address)
}

// Save inserta o reemplaza un vínculo.
func (r *ChannelLinkRepository) Save(ctx context.Context, link *channels.ChannelLink) error {
	events, err := jsonArrayArg(link.Events)
	if err != nil {
		return err
	}
	if link.ID.IsZero() {
		link.ID = primitive.NewObjectID()
	}
	_, err = r.pool.Exec(ctx,
		"INSERT INTO channel_links ("+channelLinkColumns+") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) "+
			"ON CONFLICT (id) DO UPDATE SET user_id=$2, type=$3, address=$4, display_name=$5, enabled=$6, events=$7, linked_at=$8, last_delivered_at=$9, failures=$10",
		hexOf(link.ID), hexOf(link.UserID), string(link.Type), link.Address, link.DisplayName, link.Enabled,
		events, link.LinkedAt, link.LastDeliveredAt, link.Failures)
	return err
}

// Delete borra un vínculo por id.
func (r *ChannelLinkRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM channel_links WHERE id = $1", hexOf(id))
	return err
}

// DeleteByUser borra todos los vínculos de una cuenta.
func (r *ChannelLinkRepository) DeleteByUser(ctx context.Context, userID primitive.ObjectID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM channel_links WHERE user_id = $1", hexOf(userID))
	return err
}

// CropChannelRepository implementa channels.CropRepository sobre PostgreSQL (tabla crop_channels).
type CropChannelRepository struct {
	pool *pgxpool.Pool
}

// NewCropChannelRepository crea el adaptador de avisos por cultivo.
func NewCropChannelRepository(pool *pgxpool.Pool) *CropChannelRepository {
	return &CropChannelRepository{pool: pool}
}

const cropChannelColumns = "id, crop_id, owner_id, type, enabled, events, delivery, digest_hours, daily_summary_at, recipients, last_digest_at, last_summary_at, updated_at"

func scanCropChannel(row pgx.Row) (*channels.CropChannel, error) {
	var (
		cc                      channels.CropChannel
		id, cropID, ownerID     string
		typ                     string
		delivery, dailySummary  *string
		digestHours             *int
		eventsRaw, recipientRaw []byte
	)
	if err := row.Scan(&id, &cropID, &ownerID, &typ, &cc.Enabled, &eventsRaw, &delivery, &digestHours,
		&dailySummary, &recipientRaw, &cc.LastDigestAt, &cc.LastSummaryAt, &cc.UpdatedAt); err != nil {
		return nil, err
	}
	var err error
	if cc.ID, err = parseOID(id); err != nil {
		return nil, err
	}
	if cc.CropID, err = parseOID(cropID); err != nil {
		return nil, err
	}
	if cc.OwnerID, err = parseOID(ownerID); err != nil {
		return nil, err
	}
	cc.Type = channels.Type(typ)
	if delivery != nil {
		cc.Delivery = channels.Delivery(*delivery)
	}
	if digestHours != nil {
		cc.DigestHours = *digestHours
	}
	if dailySummary != nil {
		cc.DailySummaryAt = *dailySummary
	}
	if err := scanJSONB(eventsRaw, &cc.Events); err != nil {
		return nil, err
	}
	if err := scanJSONB(recipientRaw, &cc.Recipients); err != nil {
		return nil, err
	}
	return &cc, nil
}

func (r *CropChannelRepository) queryCrops(ctx context.Context, where string, args ...any) ([]channels.CropChannel, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+cropChannelColumns+" FROM crop_channels WHERE "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]channels.CropChannel, 0)
	for rows.Next() {
		cc, err := scanCropChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *cc)
	}
	return out, rows.Err()
}

// FindByCropAndType busca los avisos de un cultivo por canal.
func (r *CropChannelRepository) FindByCropAndType(ctx context.Context, cropID primitive.ObjectID, t channels.Type) (*channels.CropChannel, error) {
	cc, err := scanCropChannel(r.pool.QueryRow(ctx,
		"SELECT "+cropChannelColumns+" FROM crop_channels WHERE crop_id = $1 AND type = $2", hexOf(cropID), string(t)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return cc, err
}

// FindEnabled devuelve los avisos por cultivo activos (para los resúmenes).
func (r *CropChannelRepository) FindEnabled(ctx context.Context) ([]channels.CropChannel, error) {
	return r.queryCrops(ctx, "enabled = true")
}

// FindByTypeAndRecipientAddress devuelve los cultivos compartidos con un chat (contención en el arreglo JSONB).
func (r *CropChannelRepository) FindByTypeAndRecipientAddress(ctx context.Context, t channels.Type, address string) ([]channels.CropChannel, error) {
	needle, err := json.Marshal([]map[string]string{{"address": address}})
	if err != nil {
		return nil, err
	}
	return r.queryCrops(ctx, "type = $1 AND recipients @> $2::jsonb", string(t), string(needle))
}

// Save inserta o reemplaza los avisos de un cultivo.
func (r *CropChannelRepository) Save(ctx context.Context, cc *channels.CropChannel) error {
	events, err := jsonArrayArg(cc.Events)
	if err != nil {
		return err
	}
	recipients, err := jsonArrayArg(cc.Recipients)
	if err != nil {
		return err
	}
	if cc.ID.IsZero() {
		cc.ID = primitive.NewObjectID()
	}
	_, err = r.pool.Exec(ctx,
		"INSERT INTO crop_channels ("+cropChannelColumns+") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) "+
			"ON CONFLICT (id) DO UPDATE SET crop_id=$2, owner_id=$3, type=$4, enabled=$5, events=$6, delivery=$7, digest_hours=$8, daily_summary_at=$9, recipients=$10, last_digest_at=$11, last_summary_at=$12, updated_at=$13",
		hexOf(cc.ID), hexOf(cc.CropID), hexOf(cc.OwnerID), string(cc.Type), cc.Enabled, events,
		nullEnum(string(cc.Delivery)), cc.DigestHours, nullEnum(cc.DailySummaryAt), recipients,
		cc.LastDigestAt, cc.LastSummaryAt, cc.UpdatedAt)
	return err
}

// DeleteByCrop borra los avisos de un cultivo.
func (r *CropChannelRepository) DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM crop_channels WHERE crop_id = $1", hexOf(cropID))
	return err
}

// DeleteByOwner borra los avisos de todos los cultivos de una cuenta.
func (r *CropChannelRepository) DeleteByOwner(ctx context.Context, ownerID primitive.ObjectID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM crop_channels WHERE owner_id = $1", hexOf(ownerID))
	return err
}
