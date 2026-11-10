package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/swiftdrop/tracking/internal/config"
	"github.com/swiftdrop/tracking/internal/health"
	trackinghttp "github.com/swiftdrop/tracking/internal/http"
	"github.com/swiftdrop/tracking/internal/logging"
	"github.com/swiftdrop/tracking/internal/repository"
	"github.com/swiftdrop/tracking/internal/service"
)

func main() {
	// 1. Load and validate configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[FATAL] Configuration validation error: %v", err)
	}

	// 2. Initialize structured logger
	logger := logging.NewLogger(cfg.LogLevel)

	// 3. Log service startup metadata (with credentials masked)
	dbURL := "(unconfigured)"
	if cfg.DatabaseURL != "" {
		dbURL = logging.MaskURL(cfg.DatabaseURL)
	}

	redisURL := "(unconfigured - Phase 3/4)"
	if cfg.RedisURL != "" {
		redisURL = logging.MaskURL(cfg.RedisURL)
	}

	rabbitmqURL := "(unconfigured - Phase 4)"
	if cfg.RabbitMQURL != "" {
		rabbitmqURL = logging.MaskURL(cfg.RabbitMQURL)
	}

	fmt.Println("==================================================================")
	fmt.Println("   SwiftDrop Tracking API — Phase 2: Public Tracking & PostgreSQL")
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
		} else {
			sqlDB = dbConn
			repo := repository.NewPostgresTrackingRepository(sqlDB, poolCfg)
			trackingRepo = repo

			// Register truthful readiness probe check for PostgreSQL
			// Task 5: CheckReadiness verifies connectivity, table existence, and SELECT permissions.
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

	// 6. Initialize application service and HTTP handler
	publicTrackingSvc := service.NewPublicTrackingService(trackingRepo)
	publicTrackingHandler := trackinghttp.NewPublicTrackingHandler(publicTrackingSvc, logger)

	// 7. Construct HTTP router with middleware pipeline
	router := trackinghttp.NewRouter(cfg, logger, healthHandler, publicTrackingHandler)

	// 8. Initialize HTTP server
	srv := trackinghttp.NewServer(cfg, router, logger)

	// 9. Setup graceful shutdown listener for SIGINT and SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// 10. Start HTTP listener
	if err := srv.Start(); err != nil {
		logger.Error("Failed to start HTTP server", "error", err)
		os.Exit(1)
	}

	logger.Info("Tracking API listening for incoming traffic", "addr", cfg.HTTPAddr)

	// 11. Block until termination signal
	sig := <-stop
	logger.Info("Shutdown signal received, initiating graceful teardown", "signal", sig.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown encountered an error", "error", err)
	} else {
		logger.Info("HTTP server gracefully terminated")
	}

	// 12. Close database connection pool
	if trackingRepo != nil {
		if err := trackingRepo.Close(); err != nil {
			logger.Error("Error closing PostgreSQL connection pool", "error", err)
		} else {
			logger.Info("PostgreSQL connection pool closed cleanly")
		}
	}

	logger.Info("SwiftDrop Tracking API stopped cleanly")
}
