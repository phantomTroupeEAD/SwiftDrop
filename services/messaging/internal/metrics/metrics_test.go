package metrics

import (
	"testing"
)

func TestMetricsRegistered(_ *testing.T) {
	// Assert metrics can be mutated without panic
	OutboxPendingEvents.Set(15)
	OutboxOldestPendingAgeSeconds.Set(120.5)
	OutboxPublishedTotal.Inc()
	OutboxPublishFailuresTotal.Inc()

	ConsumerFailuresTotal.WithLabelValues("notification_worker").Inc()
	WebhookDeliveryAttemptsTotal.WithLabelValues("success").Inc()
	WebhookDeliveryAttemptsTotal.WithLabelValues("retry").Inc()
	NotificationAttemptsTotal.WithLabelValues("failed").Inc()
	NotificationsSkippedTotal.Inc()
	DLQMessagesRoutedTotal.Inc()
	DLQDepthGauge.Set(3)
}
