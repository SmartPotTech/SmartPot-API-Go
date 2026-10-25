// Command api es el punto de entrada de SmartPot-API en Go: carga la
// configuración, cablea los componentes y expone la API HTTP.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/actuators"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/ai"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/auth"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/cache"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/commands"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/config"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/crops"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/health"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/httpserver"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/mail"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/mongodb"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/mqtt"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/notifications"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/events"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/logging"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/readings"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/security"
	"github.com/SmartPotTech/SmartPot-API-Go/internal/users"
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

	clock := func() time.Time { return time.Now().UTC() }

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelConnect()
	mongoClient, err := mongodb.Connect(connectCtx, cfg.Mongo.URI)
	if err != nil {
		return err
	}
	db := mongoClient.DB

	redisClient := cache.NewRedisClient(
		fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		cfg.Redis.Username, cfg.Redis.Password, cfg.Redis.Database, cfg.Redis.Timeout)
	cacheStore := cache.NewStore(redisClient)

	bus := events.NewBus()

	tokenService := security.NewTokenService(cfg.Security.JWTSecret, cfg.Security.JWTExpiration)
	encryptor, err := security.NewEncryptor(cfg.Security.AESKey)
	if err != nil {
		return err
	}

	mailer, err := mail.NewMailer(cfg.Mail, cfg.Server.WebBaseURL, logger)
	if err != nil {
		return err
	}

	// Repositorios.
	userRepo := users.NewRepository(db)
	tokenRepo := auth.NewTokenRepository(db)
	notificationRepo := notifications.NewRepository(db)
	cropRepo := crops.NewRepository(db)
	readingRepo := readings.NewRepository(db)
	actuatorRepo := actuators.NewRepository(db)
	commandRepo := commands.NewRepository(db)
	ensureIndexes(logger, userRepo.EnsureIndexes, tokenRepo.EnsureIndexes, notificationRepo.EnsureIndexes,
		cropRepo.EnsureIndexes, readingRepo.EnsureIndexes, actuatorRepo.EnsureIndexes, commandRepo.EnsureIndexes)

	// Servicios.
	notificationService := notifications.NewService(notificationRepo, cacheStore, bus, clock)
	topics := mqtt.NewTopicResolver(cfg.MQTT.TopicPrefix)
	mqttPublic := crops.MQTTPublic{Host: cfg.MQTT.PublicHost, Port: cfg.MQTT.PublicPort, TLS: cfg.MQTT.PublicTLS,
		WebSocketURL: cfg.MQTT.WebSocketURL, TopicPrefix: cfg.MQTT.TopicPrefix}
	cropService := crops.NewService(cropRepo, encryptor, mqttPublic, noopProvisioner{}, noopActuatorSeeder{},
		notificationService, bus, clock)
	readingService := readings.NewService(readingRepo, cropService, cacheStore, bus, clock,
		cfg.Readings.MinInterval, cfg.Readings.MaxPageSize)
	weatherService := crops.NewWeatherService(noopWeatherSource{}, cacheStore)
	actuatorService := actuators.NewService(actuatorRepo, cropService, clock)
	cropService.SetActuatorSeeder(actuatorService)
	commandService := commands.NewService(commandRepo, cropService, actuatorService, notificationService, clock,
		cfg.MQTT.CommandTimeout)

	// MQTT: gateway, aprovisionamiento en el broker y enrutado de mensajes.
	gateway := mqtt.NewGateway(cfg.MQTT, topics, logger)
	provisioner := mqtt.NewProvisioner(gateway, topics, cropService, logger)
	mqttHandler := mqtt.NewMessageHandler(topics, readingService, commandService, cropService, provisioner, logger)
	gateway.SetRouter(mqttHandler.Route)
	gateway.SetOnConnect(provisioner.OnConnected)
	cropService.SetProvisioner(provisioner)
	commandService.SetPublisher(gateway)
	gateway.Start()

	// Asistente de IA: evaluación, agente de automatización y aprendizaje continuo.
	timezone, err := time.LoadLocation(cfg.AI.Timezone)
	if err != nil {
		logger.Warn("zona horaria de la IA inválida; se usa UTC", "timezone", cfg.AI.Timezone, "err", err)
		timezone = time.UTC
	}
	aiClient := ai.NewClient(cfg.AI, cacheStore, clock)
	insightService := ai.NewInsightService(aiClient, cropService, readingService, actuatorService, weatherService,
		clock, cfg.AI.HistorySize, timezone)
	agent := ai.NewAutomationAgent(insightService, commandService, notificationService, cacheStore, aiClient,
		cropService, cfg.AI.EvaluationInterval, cfg.AI.AutomationCooldown)
	learningFeed := ai.NewLearningFeed(aiClient, cropService, cfg.AI.LearningEnabled, timezone)
	events.Subscribe(bus, func(e readings.ReadingRecordedEvent) { go agent.OnReading(e) })
	events.Subscribe(bus, func(e readings.ReadingRecordedEvent) { learningFeed.OnReading(e) })
	events.Subscribe(bus, func(e crops.DeletedEvent) { go learningFeed.OnCropDeleted(e) })

	userService := users.NewService(userRepo, clock)
	userService.AddAccountDeleter(notificationService.DeleteAllForUser)
	userService.AddAccountDeleter(tokenRepo.DeleteByUserID)
	userService.AddAccountDeleter(cropService.DeleteAllOwnedBy)
	authService := auth.NewService(userRepo, tokenRepo, tokenService, mailer, notificationService, clock,
		cfg.Security.PasswordResetExpiration)

	// Borrado en cascada de los datos de un cultivo.
	events.Subscribe(bus, func(e crops.DeletedEvent) {
		ctx := context.Background()
		if err := readingService.DeleteByCrop(ctx, e.CropID); err != nil {
			logger.Warn("no se pudieron borrar las lecturas del cultivo", "cropId", e.CropID, "err", err)
		}
		if err := commandService.DeleteByCrop(ctx, e.CropID); err != nil {
			logger.Warn("no se pudieron borrar los comandos del cultivo", "cropId", e.CropID, "err", err)
		}
		if err := actuatorService.DeleteByCrop(ctx, e.CropID); err != nil {
			logger.Warn("no se pudieron borrar los actuadores del cultivo", "cropId", e.CropID, "err", err)
		}
		if err := notificationService.DeleteAllForCrop(ctx, e.CropID); err != nil {
			logger.Warn("no se pudieron borrar las notificaciones del cultivo", "cropId", e.CropID, "err", err)
		}
	})

	// Servidor HTTP.
	rateLimiter := security.NewRateLimiter(cacheStore, cfg.Security.RateLimitPerMinute, cfg.Security.AuthRateLimitPerMinute)
	engine := httpserver.New(logger)
	engine.Use(security.CORS(cfg.Security.CORSAllowedOrigins), rateLimiter.Middleware())

	checker := health.Checker{
		Database: func() bool {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return mongoClient.Ping(ctx) == nil
		},
		Cache:  cacheStore.RedisAvailable,
		Broker: func() (bool, bool) { return gateway.IsEnabled(), gateway.IsConnected() },
		AI:     func() (bool, bool) { return aiClient.IsEnabled(), aiClient.IsHealthy() },
		// El simulador se cablea en su fase.
	}
	engine.GET("/health", checker.Handler())

	// Rutas públicas y autenticadas bajo /api/v1.
	v1 := engine.Group("/api/v1")
	auth.NewHandler(authService).Register(v1)
	aiHandler := ai.NewHandler(insightService, aiClient)
	aiHandler.RegisterPublic(v1)

	authed := v1.Group("")
	authed.Use(tokenService.Guard())
	users.NewHandler(userService).Register(authed)
	notifications.NewHandler(notificationService).Register(authed)
	crops.NewHandler(cropService, readingService, noopVirtualManager{}, weatherService).Register(authed)
	readings.NewHandler(readingService).Register(authed)
	actuators.NewHandler(actuatorService, clock).Register(authed)
	commands.NewHandler(commandService).Register(authed)
	aiHandler.RegisterAuthed(authed)

	// Trabajos en segundo plano (sin cron en el servidor).
	jobsCtx, stopJobs := context.WithCancel(context.Background())
	defer stopJobs()
	go tick(jobsCtx, time.Minute, cacheStore.PurgeExpired)
	go tick(jobsCtx, 30*time.Second, func() { commandService.ExpireStale(context.Background()) })
	go tick(jobsCtx, cfg.AI.LearningFlush, func() { learningFeed.Flush(context.Background()) })

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
	stopJobs()
	gateway.Close()
	if err := cacheStore.Close(); err != nil {
		logger.Error("cierre de Redis con error", "err", err)
	}
	if err := mongoClient.Disconnect(shutdownCtx); err != nil {
		logger.Error("desconexión de Mongo con error", "err", err)
	}
	logger.Info("apagado completo")
	return nil
}

// tick ejecuta fn cada intervalo hasta que se cancela el contexto. Reemplaza a
// los @Scheduled de Spring sin usar cron en el servidor.
func tick(ctx context.Context, interval time.Duration, fn func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn()
		}
	}
}

// ensureIndexes crea los índices de cada repositorio al arrancar; los errores se
// registran pero no impiden el arranque (la base puede estar aún iniciando).
func ensureIndexes(logger *slog.Logger, ensurers ...func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, ensure := range ensurers {
		if err := ensure(ctx); err != nil {
			logger.Warn("no se pudieron crear índices", "err", err)
		}
	}
}
