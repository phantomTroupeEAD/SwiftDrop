package repository

import (
	"context"
	"time"

	"github.com/swiftdrop/tracking/internal/domain"
)

// TrackingRepository defines read-only projection queries for parcel tracking.
type TrackingRepository interface {
	FindParcelByTrackingNumber(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, error)
	FindStatusHistoryByParcelID(ctx context.Context, parcelID string) ([]domain.MilestoneRecord, error)
	FindTrackingSummary(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error)
	CheckReadiness(ctx context.Context) error
	Ping(ctx context.Context) error
	Close() error
}

// PoolConfig encapsulates PostgreSQL connection pool and timeout settings.
// In accordance with Agreement D8, connection demand is bounded at max 5 open connections per pod replica.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	QueryTimeout    time.Duration
}

// DefaultPoolConfig returns ratified connection pool defaults (Agreement D8).
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 15 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
		QueryTimeout:    3 * time.Second,
	}
}
