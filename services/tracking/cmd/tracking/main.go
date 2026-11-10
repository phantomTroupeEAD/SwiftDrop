package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/swiftdrop/tracking/internal/config"
	"github.com/swiftdrop/tracking/internal/health"
	trackinghttp "github.com/swiftdrop/tracking/internal/http"
	"github.com/swiftdrop/tracking/internal/logging"
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
	dbURL := "(unconfigured - Phase 1 standalone)"
	if cfg.DatabaseURL != "" {
		dbURL = logging.MaskURL(cfg.DatabaseURL)
	}

	redisURL := "(unconfigured - Phase 1 standalone)"
	if cfg.RedisURL != "" {
		redisURL = logging.MaskURL(cfg.RedisURL)
	}

	rabbitmqURL := "(unconfigured - Phase 1 standalone)"
	if cfg.RabbitMQURL != "" {
		rabbitmqURL = logging.MaskURL(cfg.RabbitMQURL)
	}

	fmt.Println("==================================================================")
	fmt.Println("   SwiftDrop Tracking API — Standalone Foundation (Phase 1)")
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
	)

	// 4. Initialize health probe handler
	healthHandler := health.NewHandler()

	// 5. Construct HTTP router with middleware pipeline
	router := trackinghttp.NewRouter(cfg, logger, healthHandler)

	// 6. Initialize HTTP server
	srv := trackinghttp.NewServer(cfg, router, logger)

	// 7. Setup graceful shutdown listener for SIGINT and SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// 8. Start HTTP listener
	if err := srv.Start(); err != nil {
		logger.Error("Failed to start HTTP server", "error", err)
		os.Exit(1)
	}

	logger.Info("Tracking API listening for incoming traffic", "addr", cfg.HTTPAddr)

	// 9. Block until termination signal
	sig := <-stop
	logger.Info("Shutdown signal received, initiating graceful teardown", "signal", sig.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown encountered an error", "error", err)
	} else {
		logger.Info("HTTP server gracefully terminated")
	}

	logger.Info("SwiftDrop Tracking API stopped cleanly")
}
