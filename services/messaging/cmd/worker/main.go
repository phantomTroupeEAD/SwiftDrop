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
	"github.com/swiftdrop/messaging/internal/consumer"
	"github.com/swiftdrop/messaging/internal/delivery"
	"github.com/swiftdrop/messaging/internal/health"
	"github.com/swiftdrop/messaging/internal/idempotency"
	"github.com/swiftdrop/messaging/internal/logging"
	"github.com/swiftdrop/messaging/internal/notification"
	"github.com/swiftdrop/messaging/internal/webhook"
)

func main() {
	cfg, err := config.LoadConfig(8083)
	if err != nil {
		slog.Error("failed to load configuration", "error", err.Error())
		os.Exit(1)
	}

	logger := logging.SetupLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	logger.Info("starting SwiftDrop event worker daemon",
		"env", cfg.AppEnv,
		"probe_port", cfg.ProbePort,
		"concurrency", cfg.WorkerConcurrency,
	)

	// 1. Start HTTP Health & Metrics Probe Server First
	healthServer := health.NewServer(cfg.ProbePort)
	if err := healthServer.Start(); err != nil {
		logger.Error("failed to start health probe server", "error", err.Error())
		os.Exit(1)
	}
	logger.Info("health and metrics server listening", "port", cfg.ProbePort)

	// Root context with termination signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// 2. Connect to PostgreSQL
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open database connection", "error", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(cfg.WorkerConcurrency)
	db.SetMaxIdleConns(cfg.WorkerConcurrency)
	db.SetConnMaxLifetime(15 * time.Minute)

	// 3. Connect to RabbitMQ & Declare Topology
	brokerMgr := broker.NewManager(cfg.RabbitMQURL, logger)
	defer brokerMgr.Close()

	connectCtx, cancelConnect := context.WithTimeout(ctx, 10*time.Second)
	if err := brokerMgr.Connect(connectCtx); err != nil {
		cancelConnect()
		logger.Warn("initial broker connection deferred or failed, manager will retry", "error", err.Error())
	} else {
		cancelConnect()
	}

	// 4. Register Readiness Probes
	healthServer.AddReadyCheck("database", func(checkCtx context.Context) error {
		return db.PingContext(checkCtx)
	})
	healthServer.AddReadyCheck("broker", func(checkCtx context.Context) error {
		return brokerMgr.CheckHealth(checkCtx)
	})

	// 5. Initialize Repositories
	idempStore := idempotency.NewPostgresStore(db)
	deliveryRepo := delivery.NewPostgresDeliveryRepository(db, idempStore)
	backoffCalc := delivery.NewBackoffCalculator(cfg.WebhookRetrySchedule, nil)

	// 6. Start AMQP Queue Consumers
	notifCh, err := brokerMgr.OpenConsumerChannel(cfg.WorkerConcurrency)
	if err != nil {
		logger.Error("failed to open notification consumer channel", "error", err.Error())
		os.Exit(1)
	}
	defer notifCh.Close()

	notifDeliveries, err := notifCh.Consume(
		broker.QueueNotificationsWorker,
		"notifications-worker",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		logger.Error("failed to start consuming notifications queue", "error", err.Error())
		os.Exit(1)
	}

	notifHandler := consumer.NewNotificationHandler(deliveryRepo, idempStore, logger)
	notifRunner := consumer.NewRunner("notifications", notifHandler, logger)
	go func() {
		_ = notifRunner.Start(ctx, notifDeliveries)
	}()

	webhookCh, err := brokerMgr.OpenConsumerChannel(cfg.WorkerConcurrency)
	if err != nil {
		logger.Error("failed to open webhook consumer channel", "error", err.Error())
		os.Exit(1)
	}
	defer webhookCh.Close()

	webhookDeliveries, err := webhookCh.Consume(
		broker.QueueWebhooksDispatcher,
		"webhooks-dispatcher",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		logger.Error("failed to start consuming webhooks queue", "error", err.Error())
		os.Exit(1)
	}

	webhookFanoutHandler := consumer.NewWebhookFanoutHandler(deliveryRepo, idempStore, logger)
	webhookRunner := consumer.NewRunner("webhooks", webhookFanoutHandler, logger)
	go func() {
		_ = webhookRunner.Start(ctx, webhookDeliveries)
	}()

	// 7. Start Outbound Delivery Pollers (SMTP & Webhooks)
	smtpSender := notification.NewSMTPSender(cfg)
	notifPoller := notification.NewPoller(deliveryRepo, smtpSender, backoffCalc, cfg, logger)
	go func() {
		_ = notifPoller.Run(ctx)
	}()

	decryptor, err := webhook.NewDecryptor(cfg.WebhookSecretEncryptionKey)
	if err != nil {
		logger.Error("failed to initialize secret decryptor", "error", err.Error())
		os.Exit(1)
	}

	safeClient := webhook.NewSafeClient(cfg.WebhookHTTPTimeout, cfg.WebhookAllowPrivateHosts, nil)
	webhookDispatcher := webhook.NewDispatcher(deliveryRepo, safeClient, decryptor, backoffCalc, cfg, logger)
	go func() {
		_ = webhookDispatcher.Run(ctx)
	}()

	logger.Info("event worker daemon and dispatchers fully initialized")

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutdown signal received, initiating graceful teardown")

	// Graceful shutdown with 20s deadline
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelShutdown()

	if err := healthServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Warn("health server shutdown error", "error", err.Error())
	}

	if err := notifCh.Close(); err != nil {
		logger.Debug("notif channel close note", "error", err.Error())
	}
	if err := webhookCh.Close(); err != nil {
		logger.Debug("webhook channel close note", "error", err.Error())
	}

	_ = brokerMgr.Close()
	_ = db.Close()

	logger.Info("event worker stopped cleanly")
}
