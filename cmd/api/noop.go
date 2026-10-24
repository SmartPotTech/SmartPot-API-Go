package main

import (
	"context"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/geo"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/virtualdevices"
)

// Implementaciones vacías de los colaboradores que todavía no existen. Se
// reemplazan por las reales al construir sus fases: aprovisionamiento MQTT
// (broker), siembra de actuadores y gestión de dispositivos virtuales/clima.

type noopProvisioner struct{}

func (noopProvisioner) Provision(context.Context, string, string, bool) error { return nil }
func (noopProvisioner) Deprovision(context.Context, string) error             { return nil }

type noopActuatorSeeder struct{}

func (noopActuatorSeeder) SeedDefaults(context.Context, string, bool) error { return nil }

type noopVirtualManager struct{}

func (noopVirtualManager) CheckCanCreate(context.Context, string, *virtualdevices.VirtualDeviceRequest, *geo.Placement) error {
	return nil
}

func (noopVirtualManager) StartFor(context.Context, string, *virtualdevices.VirtualDeviceRequest) error {
	return nil
}

type noopWeatherSource struct{}

func (noopWeatherSource) IsAvailable() bool { return false }

func (noopWeatherSource) Weather(context.Context, float64, float64) (*virtualdevices.Weather, bool) {
	return nil, false
}
