package main

import (
	"context"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/crops"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/geo"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/virtualdevices"
)

// virtualCropsAdapter conecta el servicio de cultivos con lo que el servicio de
// dispositivos virtuales necesita (virtualdevices.Crops), sin que virtualdevices
// importe el paquete crops.
type virtualCropsAdapter struct {
	crops *crops.Service
}

func (a virtualCropsAdapter) GetOwned(ctx context.Context, ownerID, cropID string) (*virtualdevices.CropView, error) {
	crop, err := a.crops.GetOwned(ctx, ownerID, cropID)
	if err != nil {
		return nil, err
	}
	return cropView(crop), nil
}

func (a virtualCropsAdapter) Find(ctx context.Context, cropID string) (*virtualdevices.CropView, bool) {
	crop, err := a.crops.Find(ctx, cropID)
	if err != nil || crop == nil {
		return nil, false
	}
	return cropView(crop), true
}

func (a virtualCropsAdapter) DeviceKey(ctx context.Context, cropID string) (string, bool) {
	crop, err := a.crops.Find(ctx, cropID)
	if err != nil || crop == nil {
		return "", false
	}
	return a.crops.DeviceKey(crop)
}

func (a virtualCropsAdapter) SetLocation(ctx context.Context, cropID string, loc geo.Location) (*virtualdevices.CropView, error) {
	crop, err := a.crops.Find(ctx, cropID)
	if err != nil || crop == nil {
		return nil, err
	}
	updated, err := a.crops.SetLocation(ctx, crop, loc)
	if err != nil {
		return nil, err
	}
	return cropView(updated), nil
}

func cropView(c *crops.Crop) *virtualdevices.CropView {
	return &virtualdevices.CropView{
		ID:        c.HexID(),
		OwnerID:   c.OwnerID.Hex(),
		Name:      c.Name,
		Type:      string(c.Type),
		Virtual:   c.IsVirtual(),
		Placement: c.Placement,
	}
}
