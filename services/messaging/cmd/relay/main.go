package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/swiftdrop/messaging/internal/broker"
	"github.com/swiftdrop/messaging/internal/config"
	"github.com/swiftdrop/messaging/internal/health"
	"github.com/swiftdrop/messaging/internal/logging"
	"github.com/swiftdrop/messaging/internal/relay"
)

func main() {
	cfg, err := config.LoadConfig(8082)
	if err != nil {
		slog.Error("failed to load configuration", "error", err.Error())
		os.Exit(1)
	}

	logger := logging.NewLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	logger.Info("starting SwiftDrop outbox relay daemon",
		"env", cfg.AppEnv,
		"probe_port", cfg.ProbePort,
		"poll_interval", cfg.OutboxPollInterval,
	)

	// Root context with graceful termination signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// 1. Connect to PostgreSQL
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open postgres connection", "error", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	// Configure pool budgets per Kubernetes workload rules
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(15 * time.Minute)

	// 2. Initialize RabbitMQ Manager
	brokerMgr := broker.NewManager(cfg.RabbitMQURL, logger)
	defer brokerMgr.Close()

	// 3. Start HTTP Health & Metrics Probe Server with registered readiness checks
	healthServer := health.NewServer(cfg.ProbePort)
	healthServer.AddReadyCheck("database", func(checkCtx context.Context) error {
		return db.PingContext(checkCtx)
	})
	healthServer.AddReadyCheck("broker", func(checkCtx context.Context) error {
		return brokerMgr.CheckHealth(checkCtx)
	})
	if err := healthServer.Start(); err != nil {
		logger.Error("failed to start health probe server", "error", err.Error())
		os.Exit(1)
	}
	logger.Info("health and metrics server listening", "port", cfg.ProbePort)

	// 4. Connect to RabbitMQ & Declare Topology (with backoff & background retry)
	connectCtx, cancelConnect := context.WithTimeout(ctx, 10*time.Second)
	if err := brokerMgr.Connect(connectCtx); err != nil {
		cancelConnect()
		logger.Warn("initial broker connection deferred or failed, manager will retry", "error", err.Error())
	} else {
		cancelConnect()
	}

	// 5. Initialize Outbox Relay
	outboxRepo := relay.NewPostgresOutboxRepository(db)
	outboxRelay := relay.NewRelay(outboxRepo, brokerMgr, cfg, logger)

	// Run relay in separate goroutine
	relayErrCh := make(chan error, 1)
	go func() {
		if err := brokerMgr.WaitForConnection(ctx); err != nil {
			relayErrCh <- err
			return
		}
		relayErrCh <- outboxRelay.Run(ctx)
	}()

	// Wait for shutdown signal or fatal runner error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received, initiating graceful shutdown")
	case err := <-relayErrCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("outbox relay encountered fatal error", "error", err.Error())
		}
	}

	// Graceful teardown with 20-second deadline
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer shutdownCancel()

	if err := healthServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Warn("health server shutdown error", "error", err.Error())
	}

	if err := brokerMgr.Close(); err != nil {
		logger.Warn("broker manager close error", "error", err.Error())
	}

	if err := db.Close(); err != nil {
		logger.Warn("database close error", "error", err.Error())
	}

	logger.Info("outbox relay stopped cleanly")
}
