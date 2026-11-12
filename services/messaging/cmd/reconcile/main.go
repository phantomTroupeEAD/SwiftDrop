package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/swiftdrop/messaging/internal/config"
	"github.com/swiftdrop/messaging/internal/logging"
	"github.com/swiftdrop/messaging/internal/reconciler"
)

func main() {
	var (
		staleThresholdStr  string
		outboxThresholdStr string
		deliveryTimeoutStr string
		batchLimit         int
		dlqLimit           int
		dlqQueue           string
		targetExchange     string
	)

	// Base flags
	flag.StringVar(&staleThresholdStr, "stale-threshold", "5m", "Threshold to consider pending outbox events stale")
	flag.StringVar(&outboxThresholdStr, "outbox-threshold", "", "Alias for --stale-threshold")
	flag.StringVar(&deliveryTimeoutStr, "delivery-timeout", "30m", "Threshold to consider deliveries overdue")
	flag.IntVar(&batchLimit, "batch-limit", 1000, "Maximum records to delete per retention batch")
	flag.IntVar(&dlqLimit, "limit", 100, "Maximum messages to replay from DLQ")
	flag.StringVar(&dlqQueue, "dlq-queue", "swiftdrop.dlq", "Dead letter queue to drain")
	flag.StringVar(&targetExchange, "target-exchange", "swiftdrop.events.topic", "Exchange to republish DLQ messages to")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [subcommand] [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  (none)      Run standard reconciliation (audit stale outbox, reset leases, check overdue)\n")
		fmt.Fprintf(os.Stderr, "  retention   Prune old published events and completed delivery logs\n")
		fmt.Fprintf(os.Stderr, "  dlq-replay  Drain dead-letter queue and republish to topic exchange\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	subcommand := ""
	if len(os.Args) > 1 && os.Args[1] != "" && os.Args[1][0] != '-' {
		subcommand = os.Args[1]
		_ = flag.CommandLine.Parse(os.Args[2:])
	} else {
		_ = flag.CommandLine.Parse(os.Args[1:])
	}

	logger := logging.SetupLogger("info")

	cfg, err := config.LoadConfig(8084)
	if err != nil {
		logger.Error("configuration error", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch subcommand {
	case "dlq-replay":
		logger.Info("starting dlq-replay workflow",
			"queue", dlqQueue,
			"target_exchange", targetExchange,
			"limit", dlqLimit,
		)
		replayed, err := reconciler.ReplayDLQ(ctx, cfg.RabbitMQURL, dlqQueue, targetExchange, dlqLimit, logger)
		if err != nil {
			logger.Error("dlq-replay encountered an error", "error", err, "replayed", replayed)
			os.Exit(1)
		}
		logger.Info("dlq-replay completed successfully", "replayed_count", replayed)
		os.Exit(0)

	case "retention":
		db, err := sql.Open("postgres", cfg.ReconcilerDatabaseURL)
		if err != nil {
			logger.Error("failed to connect to database for retention", "error", err)
			os.Exit(1)
		}
		defer db.Close()

		rec := reconciler.NewReconciler(db, logger, reconciler.Options{
			BatchLimit: batchLimit,
		})

		logger.Info("starting retention cleanup pass", "batch_limit", batchLimit)
		report, err := rec.RunRetention(ctx, batchLimit)
		if err != nil {
			logger.Error("retention run failed", "error", err)
			os.Exit(1)
		}
		logger.Info("retention completed successfully",
			"deleted_outbox", report.DeletedOutbox,
			"deleted_notifications", report.DeletedNotifications,
			"deleted_webhooks", report.DeletedWebhooks,
			"deleted_events", report.DeletedEvents,
		)
		os.Exit(0)

	case "":
		// Default reconciliation pass
		threshold := reconciler.DefaultStaleThreshold
		if outboxThresholdStr != "" {
			staleThresholdStr = outboxThresholdStr
		}
		if d, err := time.ParseDuration(staleThresholdStr); err == nil {
			threshold = d
		}

		deliveryTimeout := reconciler.DefaultDeliveryTimeout
		if d, err := time.ParseDuration(deliveryTimeoutStr); err == nil {
			deliveryTimeout = d
		}

		db, err := sql.Open("postgres", cfg.ReconcilerDatabaseURL)
		if err != nil {
			logger.Error("failed to connect to database for reconciliation", "error", err)
			os.Exit(1)
		}
		defer db.Close()

		rec := reconciler.NewReconciler(db, logger, reconciler.Options{
			StaleThreshold:  threshold,
			DeliveryTimeout: deliveryTimeout,
			BatchLimit:      batchLimit,
		})

		logger.Info("starting reconciliation audit pass",
			"stale_threshold", threshold.String(),
			"delivery_timeout", deliveryTimeout.String(),
		)

		summary, err := rec.Run(ctx)
		if err != nil {
			logger.Error("reconciliation audit failed", "error", err)
			os.Exit(1)
		}

		logger.Info("reconciliation completed successfully",
			"stale_outbox", summary.StaleOutboxCount,
			"reset_notification_leases", summary.ResetNotificationLeases,
			"reset_webhook_leases", summary.ResetWebhookLeases,
			"overdue_notifications", summary.OverdueNotifications,
			"overdue_webhooks", summary.OverdueWebhooks,
		)
		os.Exit(0)

	default:
		logger.Error("unknown subcommand", "subcommand", subcommand)
		flag.Usage()
		os.Exit(1)
	}
}
