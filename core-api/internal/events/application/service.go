package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/swiftdrop/core-api/internal/platform/httpx"
)

// EventsService manages admin operations and merchant delivery inquiries.
type EventsService struct {
	repo   EventsRepository
	logger *slog.Logger
}

// NewEventsService constructs a new EventsService.
func NewEventsService(repo EventsRepository, logger *slog.Logger) *EventsService {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventsService{
		repo:   repo,
		logger: logger,
	}
}

// ListOutbox retrieves paginated outbox events.
func (s *EventsService) ListOutbox(ctx context.Context, filter OutboxFilter, cursor httpx.Cursor, limit int) ([]OutboxItem, *httpx.Cursor, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	return s.repo.ListOutbox(ctx, filter, cursor, limit)
}

// GetOutboxStats returns summary statistics for the outbox.
func (s *EventsService) GetOutboxStats(ctx context.Context) (*OutboxStats, error) {
	return s.repo.GetOutboxStats(ctx)
}

// RetryOutbox un-stalls a FAILED outbox event and resets it to PENDING.
func (s *EventsService) RetryOutbox(ctx context.Context, id string) error {
	if err := s.repo.RetryOutbox(ctx, id); err != nil {
		return err
	}
	s.logger.Info("admin manual retry executed on outbox event", "outbox_id", id)
	return nil
}

// ListDeliveries retrieves unified delivery logs.
func (s *EventsService) ListDeliveries(ctx context.Context, filter DeliveryFilter, cursor httpx.Cursor, limit int) ([]UnifiedDeliveryItem, *httpx.Cursor, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	return s.repo.ListDeliveries(ctx, filter, cursor, limit)
}

// RetryWebhookDelivery resets a FAILED webhook delivery to PENDING.
func (s *EventsService) RetryWebhookDelivery(ctx context.Context, id string) error {
	if err := s.repo.RetryWebhookDelivery(ctx, id); err != nil {
		return err
	}
	s.logger.Info("admin manual retry executed on webhook delivery", "delivery_id", id)
	return nil
}

// ListMerchantWebhookDeliveries returns deliveries for endpoints owned by merchant.
func (s *EventsService) ListMerchantWebhookDeliveries(ctx context.Context, merchantID string, cursor httpx.Cursor, limit int) ([]MerchantDeliveryItem, *httpx.Cursor, error) {
	if merchantID == "" {
		return nil, nil, fmt.Errorf("%w: missing merchant id", ErrTenantUnauthorized)
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	return s.repo.ListMerchantWebhookDeliveries(ctx, merchantID, cursor, limit)
}

// ListMerchantWebhookAttempts returns attempt history for an owned delivery.
func (s *EventsService) ListMerchantWebhookAttempts(ctx context.Context, merchantID, deliveryID string) ([]WebhookAttemptItem, error) {
	if merchantID == "" {
		return nil, fmt.Errorf("%w: missing merchant id", ErrTenantUnauthorized)
	}
	return s.repo.ListMerchantWebhookAttempts(ctx, merchantID, deliveryID)
}
