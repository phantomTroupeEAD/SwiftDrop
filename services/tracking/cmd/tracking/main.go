package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/swiftdrop/tracking/internal/auth"
	"github.com/swiftdrop/tracking/internal/config"
	"github.com/swiftdrop/tracking/internal/domain"
	"github.com/swiftdrop/tracking/internal/dto"
	"github.com/swiftdrop/tracking/internal/events"
	"github.com/swiftdrop/tracking/internal/health"
	trackinghttp "github.com/swiftdrop/tracking/internal/http"
	"github.com/swiftdrop/tracking/internal/logging"
	"github.com/swiftdrop/tracking/internal/repository"
	"github.com/swiftdrop/tracking/internal/service"
)

type localHubPublisher struct {
	hub *events.EventHub
}

func (l *localHubPublisher) Publish(_ context.Context, ev events.RedisTrackingEvent) error {
	if l.hub == nil {
		return nil
	}
	l.hub.Broadcast(dto.PublicStatusUpdateEvent{
		TrackingNumber:  ev.TrackingNumber,
		Status:          ev.Status,
		Description:     domain.SafeStatusDescription(ev.Status),
		Timestamp:       ev.OccurredAt.UTC(),
		StatusHistoryID: ev.StatusHistoryID,
	})
	return nil
}

func main() {
	// 1. Load and validate configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[FATAL] Configuration validation error: %v", err)
	}

	// 2. Initialize structured logger
	logger := logging.NewLogger(cfg.LogLevel)

	// 3. Log service startup metadata (with all connection credentials masked)
	dbURL := "(unconfigured)"
	if cfg.DatabaseURL != "" {
		dbURL = logging.MaskURL(cfg.DatabaseURL)
	}

	redisURL := "(unconfigured)"
	if cfg.RedisURL != "" {
		redisURL = logging.MaskURL(cfg.RedisURL)
	}

	rabbitmqURL := "(unconfigured)"
	if cfg.RabbitMQURL != "" {
		rabbitmqURL = logging.MaskURL(cfg.RabbitMQURL)
	}

	fmt.Println("==================================================================")
	fmt.Println("   SwiftDrop Tracking API — Phase 5: Production Event Pipeline")
	fmt.Printf("   Service Address: %s | Environment: %s\n", cfg.HTTPAddr, cfg.AppEnv)
	fmt.Println("==================================================================")

	logger.Info("Starting SwiftDrop Tracking API service",
		"http_addr", cfg.HTTPAddr,
		"env", cfg.AppEnv,
		"log_level", cfg.LogLevel,
		"shutdown_timeout", cfg.ShutdownTimeout.String(),
		"read_timeout", cfg.ReadTimeout.String(),
		"write_timeout", cfg.WriteTimeout.String(),
		"database_url", dbURL,
		"redis_url", redisURL,
		"rabbitmq_url", rabbitmqURL,
		"rabbitmq_queue", cfg.RabbitMQQueue,
		"redis_channel", cfg.RedisChannel,
		"db_max_open_conns", cfg.DBMaxOpenConns,
		"db_max_idle_conns", cfg.DBMaxIdleConns,
		"db_query_timeout", cfg.DBQueryTimeout.String(),
	)

	// 4. Initialize health probe handler
	healthHandler := health.NewHandler()

	// 5. Initialize PostgreSQL persistence (if configured)
	var (
		sqlDB        *sql.DB
		trackingRepo repository.TrackingRepository
	)

	poolCfg := repository.PoolConfig{
		MaxOpenConns:    cfg.DBMaxOpenConns,
		MaxIdleConns:    cfg.DBMaxIdleConns,
		ConnMaxLifetime: cfg.DBConnMaxLifetime,
		ConnMaxIdleTime: cfg.DBConnMaxIdleTime,
		QueryTimeout:    cfg.DBQueryTimeout,
	}

	if cfg.DatabaseURL != "" {
		dbConn, err := repository.OpenPostgresDB(cfg.DatabaseURL, poolCfg)
		if err != nil {
			logger.Error("Failed to initialize PostgreSQL connection pool", "error", err)
			healthHandler.AddCheck("postgres", func(ctx context.Context) error {
				return fmt.Errorf("postgres connection pool initialization failed: %w", err)
			})
		} else {
			sqlDB = dbConn
			repo := repository.NewPostgresTrackingRepository(sqlDB, poolCfg)
			trackingRepo = repo

			// Register truthful readiness probe check for PostgreSQL
			healthHandler.AddCheck("postgres", func(ctx context.Context) error {
				return repo.CheckReadiness(ctx)
			})
			logger.Info("PostgreSQL connection pool initialized with read-only tracking adapter")
		}
	} else {
		logger.Warn("TRACKING_DATABASE_URL is not configured; PostgreSQL repository will report dependency unavailable")
		healthHandler.AddCheck("postgres", func(ctx context.Context) error {
			return errors.New("database unconfigured (TRACKING_DATABASE_URL not set)")
		})
	}

	// 6. Initialize in-memory EventHub
	eventHub := events.NewEventHub(events.WithLogger(logger))

	// 7. Initialize Redis Pub/Sub (Phase 5: horizontal cross-pod fan-out)
	var (
		redisPubSub *events.RedisPubSub
		publisher   events.TrackingEventPublisher
	)

	if strings.TrimSpace(cfg.RedisURL) != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			logger.Error("Failed to parse REDIS_URL", "error", err)
			healthHandler.AddCheck("redis", func(ctx context.Context) error {
				return fmt.Errorf("malformed REDIS_URL: %w", err)
			})
		} else {
			opt.DialTimeout = cfg.RedisDialTimeout
			opt.ReadTimeout = cfg.RedisReadTimeout
			opt.WriteTimeout = cfg.RedisWriteTimeout

			redisClient := redis.NewClient(opt)
			redisPubSub = events.NewRedisPubSub(redisClient, cfg.RedisChannel, eventHub, logger)
			publisher = redisPubSub

			// Register truthful readiness probe check for Redis
			healthHandler.AddCheck("redis", func(ctx context.Context) error {
				return redisPubSub.Ping(ctx)
			})

			// Launch background Redis subscription loop
			_ = redisPubSub.StartListening(context.Background())
			logger.Info("Redis Pub/Sub distributor initialized", "channel", cfg.RedisChannel)
		}
	} else {
		publisher = &localHubPublisher{hub: eventHub}
		if cfg.IsProduction() {
			logger.Warn("REDIS_URL is unconfigured in production; multi-pod fan-out disabled")
			healthHandler.AddCheck("redis", func(ctx context.Context) error {
				return errors.New("redis unconfigured in production (REDIS_URL not set)")
			})
		} else {
			logger.Info("REDIS_URL unconfigured; running in single-replica local hub mode")
		}
	}

	// 8. Initialize RabbitMQ Consumer (Phase 5: production event ingestion)
	var rabbitConsumer *events.RabbitMQConsumer

	if strings.TrimSpace(cfg.RabbitMQURL) != "" {
		rabbitConsumer = events.NewRabbitMQConsumer(
			cfg.RabbitMQURL,
			cfg.RabbitMQQueue,
			cfg.RabbitMQPrefetch,
			cfg.RabbitMQReconnectDelay,
			trackingRepo,
			publisher,
			logger,
		)

		// Register truthful readiness probe check for RabbitMQ
		healthHandler.AddCheck("rabbitmq", func(ctx context.Context) error {
			return rabbitConsumer.Ping(ctx)
		})

		// Launch background RabbitMQ consumption loop
		_ = rabbitConsumer.Start(context.Background())
		logger.Info("RabbitMQ event consumer initialized",
			"queue", cfg.RabbitMQQueue,
			"prefetch", cfg.RabbitMQPrefetch,
		)
	} else {
		if cfg.IsProduction() {
			logger.Warn("RABBITMQ_URL is unconfigured in production; real-time event pipeline disabled")
			healthHandler.AddCheck("rabbitmq", func(ctx context.Context) error {
				return errors.New("rabbitmq unconfigured in production (RABBITMQ_URL not set)")
			})
		} else {
			logger.Info("RABBITMQ_URL unconfigured; consumer idle (awaiting configuration)")
		}
	}

	// 9. Initialize application service and HTTP handlers
	publicTrackingSvc := service.NewPublicTrackingService(trackingRepo)
	publicTrackingHandler := trackinghttp.NewPublicTrackingHandler(publicTrackingSvc, logger)

	// Phase 5: Wire EventHub into SSEHandler for real-time streaming
	sseHandler := trackinghttp.NewSSEHandler(
		publicTrackingSvc,
		eventHub,
		time.Duration(cfg.SSEHeartbeatSeconds)*time.Second,
		logger,
	)

	// Authenticated Tracking REST Endpoint (Phase 3)
	jwtVerifier := auth.NewJWTVerifier(cfg.JWTSecret, cfg.JWTIssuer)
	authTrackingSvc := service.NewAuthenticatedTrackingService(trackingRepo)
	authTrackingHandler := trackinghttp.NewAuthenticatedTrackingHandler(authTrackingSvc, jwtVerifier, logger)

	// 10. Construct HTTP router with middleware pipeline
	router := trackinghttp.NewRouter(cfg, logger, healthHandler, publicTrackingHandler, sseHandler, authTrackingHandler)

	// 11. Initialize HTTP server
	srv := trackinghttp.NewServer(cfg, router, logger)

	// 12. Setup graceful shutdown listener for SIGINT and SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// 13. Start HTTP listener
	if err := srv.Start(); err != nil {
		logger.Error("Failed to start HTTP server", "error", err)
		os.Exit(1)
	}

	logger.Info("Tracking API listening for incoming traffic", "addr", cfg.HTTPAddr)

	// 14. Block until termination signal
	sig := <-stop
	logger.Info("Shutdown signal received, initiating graceful teardown", "signal", sig.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown encountered an error", "error", err)
	} else {
		logger.Info("HTTP server gracefully terminated")
	}

	// 15. Gracefully terminate RabbitMQ consumer and Redis Pub/Sub
	if rabbitConsumer != nil {
		if err := rabbitConsumer.Close(); err != nil {
			logger.Error("Error closing RabbitMQ consumer", "error", err)
		} else {
			logger.Info("RabbitMQ consumer closed cleanly")
		}
	}

	if redisPubSub != nil {
		if err := redisPubSub.Close(); err != nil {
			logger.Error("Error closing Redis Pub/Sub distributor", "error", err)
		} else {
			logger.Info("Redis Pub/Sub distributor closed cleanly")
		}
	}

	if err := eventHub.Close(); err != nil {
		logger.Error("Error closing EventHub", "error", err)
	}

	// 16. Close database connection pool
	if trackingRepo != nil {
		if err := trackingRepo.Close(); err != nil {
			logger.Error("Error closing PostgreSQL connection pool", "error", err)
		} else {
			logger.Info("PostgreSQL connection pool closed cleanly")
		}
	}

	logger.Info("SwiftDrop Tracking API stopped cleanly")
}
