package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/crops"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/geo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CropRepository implementa crops.Repository sobre PostgreSQL (tabla crops).
type CropRepository struct {
	pool *pgxpool.Pool
}

// NewCropRepository crea el adaptador de cultivos.
func NewCropRepository(pool *pgxpool.Pool) *CropRepository {
	return &CropRepository{pool: pool}
}

const cropColumns = "id, owner_id, name, type, kind, form, placement, automation_enabled, device, health, created_at, updated_at"

func scanCrop(row pgx.Row) (*crops.Crop, error) {
	var (
		c                       crops.Crop
		id, ownerID, cropType   string
		kind, form              *string
		placementRaw, deviceRaw []byte
		healthRaw               []byte
		updatedAt               *time.Time
	)
	err := row.Scan(&id, &ownerID, &c.Name, &cropType, &kind, &form, &placementRaw,
		&c.AutomationEnabled, &deviceRaw, &healthRaw, &c.CreatedAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if c.ID, err = parseOID(id); err != nil {
		return nil, err
	}
	if c.OwnerID, err = parseOID(ownerID); err != nil {
		return nil, err
	}
	c.Type = crops.CropType(cropType)
	if kind != nil {
		c.Kind = crops.CropKind(*kind)
	}
	if form != nil {
		c.Form = crops.CropForm(*form)
	}
	if len(placementRaw) > 0 && string(placementRaw) != "null" {
		var p geo.Placement
		if err := json.Unmarshal(placementRaw, &p); err != nil {
			return nil, err
		}
		c.Placement = &p
	}
	if len(deviceRaw) > 0 && string(deviceRaw) != "null" {
		var d crops.Device
		if err := json.Unmarshal(deviceRaw, &d); err != nil {
			return nil, err
		}
		c.Device = &d
	}
	if len(healthRaw) > 0 && string(healthRaw) != "null" {
		var h crops.CropHealth
		if err := json.Unmarshal(healthRaw, &h); err != nil {
			return nil, err
		}
		c.Health = &h
	}
	if updatedAt != nil {
		c.UpdatedAt = *updatedAt
	}
	return &c, nil
}

func (r *CropRepository) queryCrops(ctx context.Context, sql string, args ...any) ([]crops.Crop, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]crops.Crop, 0)
	for rows.Next() {
		c, err := scanCrop(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (r *CropRepository) findOne(ctx context.Context, where string, args ...any) (*crops.Crop, error) {
	c, err := scanCrop(r.pool.QueryRow(ctx, "SELECT "+cropColumns+" FROM crops WHERE "+where, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

// ListByOwner devuelve los cultivos del dueño, más antiguos primero.
func (r *CropRepository) ListByOwner(ctx context.Context, ownerID primitive.ObjectID) ([]crops.Crop, error) {
	return r.queryCrops(ctx, "SELECT "+cropColumns+" FROM crops WHERE owner_id = $1 ORDER BY created_at ASC", hexOf(ownerID))
}

// FindByIDAndOwner busca un cultivo del dueño.
func (r *CropRepository) FindByIDAndOwner(ctx context.Context, id, ownerID primitive.ObjectID) (*crops.Crop, error) {
	return r.findOne(ctx, "id = $1 AND owner_id = $2", hexOf(id), hexOf(ownerID))
}

// FindByID busca un cultivo por id.
func (r *CropRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*crops.Crop, error) {
	return r.findOne(ctx, "id = $1", hexOf(id))
}

// FindWithDeviceKey devuelve los cultivos con clave de dispositivo (para sincronizar el broker al reconectar).
func (r *CropRepository) FindWithDeviceKey(ctx context.Context) ([]crops.Crop, error) {
	return r.queryCrops(ctx, "SELECT "+cropColumns+" FROM crops WHERE device ->> 'keyCiphertext' IS NOT NULL")
}

// CountByOwner cuenta los cultivos del dueño.
func (r *CropRepository) CountByOwner(ctx context.Context, ownerID primitive.ObjectID) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, "SELECT count(*) FROM crops WHERE owner_id = $1", hexOf(ownerID)).Scan(&n)
	return n, err
}

// Insert guarda un cultivo nuevo y fija su id generado.
func (r *CropRepository) Insert(ctx context.Context, crop *crops.Crop) error {
	if crop.ID.IsZero() {
		crop.ID = primitive.NewObjectID()
	}
	placement, err := jsonbArg(crop.Placement)
	if err != nil {
		return err
	}
	device, err := jsonbArg(crop.Device)
	if err != nil {
		return err
	}
	health, err := jsonbArg(crop.Health)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		"INSERT INTO crops ("+cropColumns+") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)",
		hexOf(crop.ID), hexOf(crop.OwnerID), crop.Name, string(crop.Type), nullEnum(string(crop.Kind)),
		nullEnum(string(crop.Form)), placement, crop.AutomationEnabled, device, health, crop.CreatedAt, nullTime(crop.UpdatedAt))
	return err
}

// Save reemplaza el cultivo.
func (r *CropRepository) Save(ctx context.Context, crop *crops.Crop) error {
	placement, err := jsonbArg(crop.Placement)
	if err != nil {
		return err
	}
	device, err := jsonbArg(crop.Device)
	if err != nil {
		return err
	}
	health, err := jsonbArg(crop.Health)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		"UPDATE crops SET owner_id=$2, name=$3, type=$4, kind=$5, form=$6, placement=$7, automation_enabled=$8, device=$9, health=$10, created_at=$11, updated_at=$12 WHERE id=$1",
		hexOf(crop.ID), hexOf(crop.OwnerID), crop.Name, string(crop.Type), nullEnum(string(crop.Kind)),
		nullEnum(string(crop.Form)), placement, crop.AutomationEnabled, device, health, crop.CreatedAt, nullTime(crop.UpdatedAt))
	return err
}

// Delete borra el cultivo por id.
func (r *CropRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM crops WHERE id = $1", hexOf(id))
	return err
}

// UpdatePlacement cambia solo el lugar y la fecha de actualización.
func (r *CropRepository) UpdatePlacement(ctx context.Context, id primitive.ObjectID, placement *geo.Placement, now time.Time) error {
	p, err := jsonbArg(placement)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, "UPDATE crops SET placement = $2, updated_at = $3 WHERE id = $1", hexOf(id), p, now)
	return err
}

// MarkDeviceSeen actualiza solo el estado del dispositivo (last_seen y, si se indica, online) sin pisar la clave.
func (r *CropRepository) MarkDeviceSeen(ctx context.Context, id primitive.ObjectID, online *bool, now time.Time) error {
	seen, err := json.Marshal(now)
	if err != nil {
		return err
	}
	if online == nil {
		_, err = r.pool.Exec(ctx,
			"UPDATE crops SET device = jsonb_set(COALESCE(device, '{}'::jsonb), '{lastSeenAt}', $2::jsonb, true) WHERE id = $1",
			hexOf(id), string(seen))
		return err
	}
	on, err := json.Marshal(*online)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		"UPDATE crops SET device = jsonb_set(jsonb_set(COALESCE(device, '{}'::jsonb), '{lastSeenAt}', $2::jsonb, true), '{online}', $3::jsonb, true) WHERE id = $1",
		hexOf(id), string(seen), string(on))
	return err
}

// UpdateHealth actualiza solo la salud del cultivo.
func (r *CropRepository) UpdateHealth(ctx context.Context, id primitive.ObjectID, health *crops.CropHealth) error {
	h, err := jsonbArg(health)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, "UPDATE crops SET health = $2 WHERE id = $1", hexOf(id), h)
	return err
}

// BackfillKind fija el tipo en los cultivos sin tipo cuyo id esté en la lista.
func (r *CropRepository) BackfillKind(ctx context.Context, ids []primitive.ObjectID, kind crops.CropKind) error {
	if len(ids) == 0 {
		return nil
	}
	hexes := make([]string, len(ids))
	for i, id := range ids {
		hexes[i] = id.Hex()
	}
	_, err := r.pool.Exec(ctx, "UPDATE crops SET kind = $1 WHERE kind IS NULL AND id = ANY($2)", string(kind), hexes)
	return err
}

// BackfillKindAll fija el tipo en todos los cultivos que aún no lo tienen.
func (r *CropRepository) BackfillKindAll(ctx context.Context, kind crops.CropKind) error {
	_, err := r.pool.Exec(ctx, "UPDATE crops SET kind = $1 WHERE kind IS NULL", string(kind))
	return err
}

// BackfillFormAll fija la forma en todos los cultivos que aún no la tienen.
func (r *CropRepository) BackfillFormAll(ctx context.Context, form crops.CropForm) error {
	_, err := r.pool.Exec(ctx, "UPDATE crops SET form = $1 WHERE form IS NULL", string(form))
	return err
}

// BackfillPlacement fija el lugar de un cultivo si aún no lo tiene.
func (r *CropRepository) BackfillPlacement(ctx context.Context, id primitive.ObjectID, placement geo.Placement) error {
	p, err := jsonbArg(&placement)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, "UPDATE crops SET placement = $2 WHERE id = $1 AND placement IS NULL", hexOf(id), p)
	return err
}
