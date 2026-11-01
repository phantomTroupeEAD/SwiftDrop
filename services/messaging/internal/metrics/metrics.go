package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Outbox Metrics
	OutboxPendingEvents = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "swiftdrop_outbox_pending_events",
		Help: "Current count of unpublished events remaining in the outbox queue.",
	})

	OutboxOldestPendingAgeSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "swiftdrop_outbox_oldest_pending_age_seconds",
		Help: "Age in seconds of the oldest pending event in the outbox.",
	})

	OutboxPolledTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "swiftdrop_outbox_polled_total",
		Help: "Total count of outbox events polled from the database.",
	})

	OutboxPublishFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "swiftdrop_outbox_publish_failures_total",
		Help: "Total count of failed attempts to publish outbox events to RabbitMQ.",
	})

	OutboxPublishedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "swiftdrop_outbox_published_total",
		Help: "Total count of outbox events successfully published and confirmed by RabbitMQ.",
	})

	// Consumer Metrics
	ConsumerFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "swiftdrop_consumer_failures_total",
		Help: "Total count of unhandled errors encountered by consumer goroutines.",
	}, []string{"consumer"})

	// Webhook Delivery Metrics
	WebhookDeliveryAttemptsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "swiftdrop_webhook_delivery_attempts_total",
		Help: "Total count of outbound HTTP webhook delivery attempts partitioned by outcome.",
	}, []string{"outcome"}) // outcome: success, retry, failed

	// Notification Metrics
	NotificationAttemptsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "swiftdrop_notification_attempts_total",
		Help: "Total count of SMTP notification delivery attempts partitioned by outcome.",
	}, []string{"outcome"}) // outcome: success, retry, failed

	NotificationsSkippedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "swiftdrop_notifications_skipped_total",
		Help: "Total count of events skipped because no customer notification template applies.",
	})

	// Dead-Letter Queue (DLQ) Metrics
	DLQMessagesRoutedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "swiftdrop_dlq_messages_routed_total",
		Help: "Total count of poison or rejected messages routed to the dead-letter queue.",
	})

	DLQDepthGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "swiftdrop_dlq_depth_gauge",
		Help: "Current message depth of the dead-letter queue swiftdrop.dlq.",
	})
)
