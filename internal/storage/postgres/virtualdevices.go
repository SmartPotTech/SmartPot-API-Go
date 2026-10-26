package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/readings"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/virtualdevices"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// VirtualDeviceRepository implementa virtualdevices.Repository sobre PostgreSQL (tabla virtual_devices).
type VirtualDeviceRepository struct {
	pool *pgxpool.Pool
}

// NewVirtualDeviceRepository crea el adaptador de simulaciones.
func NewVirtualDeviceRepository(pool *pgxpool.Pool) *VirtualDeviceRepository {
	return &VirtualDeviceRepository{pool: pool}
}

const virtualDeviceColumns = "id, crop_id, owner_id, mode, manual, location, interval_seconds, active, created_at, updated_at"

func scanVirtualDevice(row pgx.Row) (*virtualdevices.VirtualDevice, error) {
	var (
		v                      virtualdevices.VirtualDevice
		id, cropID, ownerID    string
		mode                   string
		manualRaw, locationRaw []byte
	)
	if err := row.Scan(&id, &cropID, &ownerID, &mode, &manualRaw, &locationRaw,
		&v.IntervalSeconds, &v.Active, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return nil, err
	}
	var err error
	if v.ID, err = parseOID(id); err != nil {
		return nil, err
	}
	if v.CropID, err = parseOID(cropID); err != nil {
		return nil, err
	}
	if v.OwnerID, err = parseOID(ownerID); err != nil {
		return nil, err
	}
	v.Mode = virtualdevices.Mode(mode)
	if len(manualRaw) > 0 && string(manualRaw) != "null" {
		var m readings.Measures
		if err := json.Unmarshal(manualRaw, &m); err != nil {
			return nil, err
		}
		v.Manual = &m
	}
	if len(locationRaw) > 0 && string(locationRaw) != "null" {
		var loc virtualdevices.Location
		if err := json.Unmarshal(locationRaw, &loc); err != nil {
			return nil, err
		}
		v.Location = &loc
	}
	return &v, nil
}

// FindByCropID busca la simulación de un cultivo, o nil si no existe.
func (r *VirtualDeviceRepository) FindByCropID(ctx context.Context, cropID primitive.ObjectID) (*virtualdevices.VirtualDevice, error) {
	v, err := scanVirtualDevice(r.pool.QueryRow(ctx,
		"SELECT "+virtualDeviceColumns+" FROM virtual_devices WHERE crop_id = $1", hexOf(cropID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return v, err
}

// CountByOwner cuenta las simulaciones de una cuenta.
func (r *VirtualDeviceRepository) CountByOwner(ctx context.Context, ownerID primitive.ObjectID) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, "SELECT count(*) FROM virtual_devices WHERE owner_id = $1", hexOf(ownerID)).Scan(&n)
	return n, err
}

// FindAll devuelve todas las simulaciones (para la reconciliación).
func (r *VirtualDeviceRepository) FindAll(ctx context.Context) ([]virtualdevices.VirtualDevice, error) {
	rows, err := r.pool.Query(ctx, "SELECT "+virtualDeviceColumns+" FROM virtual_devices")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]virtualdevices.VirtualDevice, 0)
	for rows.Next() {
		v, err := scanVirtualDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// Save inserta o reemplaza la simulación.
func (r *VirtualDeviceRepository) Save(ctx context.Context, v *virtualdevices.VirtualDevice) error {
	manual, err := jsonbArg(v.Manual)
	if err != nil {
		return err
	}
	location, err := jsonbArg(v.Location)
	if err != nil {
		return err
	}
	if v.ID.IsZero() {
		v.ID = primitive.NewObjectID()
	}
	_, err = r.pool.Exec(ctx,
		"INSERT INTO virtual_devices ("+virtualDeviceColumns+") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) "+
			"ON CONFLICT (id) DO UPDATE SET crop_id=$2, owner_id=$3, mode=$4, manual=$5, location=$6, interval_seconds=$7, active=$8, created_at=$9, updated_at=$10",
		hexOf(v.ID), hexOf(v.CropID), hexOf(v.OwnerID), string(v.Mode), manual, location,
		v.IntervalSeconds, v.Active, v.CreatedAt, v.UpdatedAt)
	return err
}

// DeleteByCrop borra la simulación de un cultivo.
func (r *VirtualDeviceRepository) DeleteByCrop(ctx context.Context, cropID primitive.ObjectID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM virtual_devices WHERE crop_id = $1", hexOf(cropID))
	return err
}
