package main

import "context"

// Implementaciones vacías de los colaboradores que se fijan por setter después
// de construir el servicio de cultivos: el aprovisionamiento MQTT (broker) y la
// siembra de actuadores se reemplazan por los reales al cablearlos.

type noopProvisioner struct{}

func (noopProvisioner) Provision(context.Context, string, string, bool) error { return nil }
func (noopProvisioner) Deprovision(context.Context, string) error             { return nil }

type noopActuatorSeeder struct{}

func (noopActuatorSeeder) SeedDefaults(context.Context, string, bool) error { return nil }
