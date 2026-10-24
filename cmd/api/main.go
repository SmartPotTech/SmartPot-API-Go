// Command api es el punto de entrada de SmartPot-API en Go: carga la
// configuración, cablea los componentes y expone la API HTTP.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/config"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/health"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/mongodb"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/logging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error fatal al arrancar:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := logging.Setup(cfg.Server.LogLevel)
	logger.Info("iniciando SmartPot-API", "port", cfg.Server.Port)

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelConnect()
	mongoClient, err := mongodb.Connect(connectCtx, cfg.Mongo.URI)
	if err != nil {
		return err
	}

	engine := httpserver.New(logger)

	checker := health.Checker{
		Database: func() bool {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return mongoClient.Ping(ctx) == nil
		},
		// Broker, caché, IA y simulador se cablean en sus fases.
	}
	engine.GET("/health", checker.Handler())

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("API escuchando", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		return fmt.Errorf("el servidor HTTP falló: %w", err)
	case <-ctx.Done():
		logger.Info("apagando (señal recibida)")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("apagado del servidor con error", "err", err)
	}
	if err := mongoClient.Disconnect(shutdownCtx); err != nil {
		logger.Error("desconexión de Mongo con error", "err", err)
	}
	logger.Info("apagado completo")
	return nil
}
