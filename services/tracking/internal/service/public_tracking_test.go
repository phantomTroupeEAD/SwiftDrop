package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/domain"
	"github.com/swiftdrop/tracking/internal/repository"
)

type mockTrackingRepo struct {
	findSummaryFunc func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error)
}

func (m *mockTrackingRepo) FindParcelByTrackingNumber(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, error) {
	return nil, nil
}

func (m *mockTrackingRepo) FindStatusHistoryByParcelID(ctx context.Context, parcelID string) ([]domain.MilestoneRecord, error) {
	return nil, nil
}

func (m *mockTrackingRepo) FindTrackingSummary(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
	if m.findSummaryFunc != nil {
		return m.findSummaryFunc(ctx, trackingNumber)
	}
	return nil, nil, errors.New("not implemented")
}

func (m *mockTrackingRepo) Ping(ctx context.Context) error {
	return nil
}

func (m *mockTrackingRepo) CheckReadiness(ctx context.Context) error {
	return nil
}

func (m *mockTrackingRepo) Close() error {
	return nil
}

var _ repository.TrackingRepository = (*mockTrackingRepo)(nil)

func TestPublicTrackingService_GetPublicTracking_Success(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	tn, _ := domain.NewTrackingNumber("SD-20261106-000001")

	repo := &mockTrackingRepo{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			parcel := &domain.ParcelTrackingInfo{
				ID:              "parcel-uuid-1",
				TrackingNumber:  tn,
				Status:          domain.StatusInTransit,
				DestinationCity: "Galle",
				CreatedAt:       now.Add(-2 * time.Hour),
				UpdatedAt:       now,
			}
			history := []domain.MilestoneRecord{
				{
					ID:        "h-1",
					ParcelID:  "parcel-uuid-1",
					ToStatus:  "CREATED",
					Timestamp: now.Add(-2 * time.Hour),
				},
				{
					ID:        "h-2",
					ParcelID:  "parcel-uuid-1",
					ToStatus:  "PICKED_UP",
					Timestamp: now.Add(-1 * time.Hour),
				},
				{
					ID:        "h-3",
					ParcelID:  "parcel-uuid-1",
					ToStatus:  "IN_TRANSIT",
					Timestamp: now,
				},
			}
			return parcel, history, nil
		},
	}

	svc := NewPublicTrackingService(repo)
	res, err := svc.GetPublicTracking(context.Background(), "SD-20261106-000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.TrackingNumber != "SD-20261106-000001" {
		t.Errorf("expected tracking number SD-20261106-000001, got %s", res.TrackingNumber)
	}
	if res.Status != "IN_TRANSIT" {
		t.Errorf("expected status IN_TRANSIT, got %s", res.Status)
	}
	if res.StatusDescription != "Parcel sorted and in transit between hubs." {
		t.Errorf("expected safe status description, got %q", res.StatusDescription)
	}
	if res.Destination.City != "Galle" {
		t.Errorf("expected destination city Galle, got %s", res.Destination.City)
	}
	if res.EstimatedDelivery != nil {
		t.Errorf("expected nil estimated delivery, got %v", res.EstimatedDelivery)
	}
	if len(res.Milestones) != 3 {
		t.Fatalf("expected 3 milestones, got %d", len(res.Milestones))
	}

	// Verify chronological ordering of milestones
	if res.Milestones[0].Status != "CREATED" || res.Milestones[1].Status != "PICKED_UP" || res.Milestones[2].Status != "IN_TRANSIT" {
		t.Errorf("unexpected milestone sequence: %v", res.Milestones)
	}
	if res.Milestones[0].Description != "Shipment booked and delivery label generated." {
		t.Errorf("expected safe milestone 1 description, got %q", res.Milestones[0].Description)
	}
}

func TestPublicTrackingService_GetPublicTracking_InvalidTrackingNumber(t *testing.T) {
	svc := NewPublicTrackingService(&mockTrackingRepo{})
	_, err := svc.GetPublicTracking(context.Background(), "INVALID-TN-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidTrackingNumber) {
		t.Errorf("expected ErrInvalidTrackingNumber, got %v", err)
	}
}

func TestPublicTrackingService_GetPublicTracking_NotFound(t *testing.T) {
	repo := &mockTrackingRepo{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return nil, nil, domain.ErrTrackingNotFound
		},
	}
	svc := NewPublicTrackingService(repo)
	_, err := svc.GetPublicTracking(context.Background(), "SD-20261106-000001")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrTrackingNotFound) {
		t.Errorf("expected ErrTrackingNotFound, got %v", err)
	}
}

func TestPublicTrackingService_GetPublicTracking_EmptyHistoryFallback(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	tn, _ := domain.NewTrackingNumber("SD-20261106-000001")

	repo := &mockTrackingRepo{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			parcel := &domain.ParcelTrackingInfo{
				ID:              "parcel-uuid-1",
				TrackingNumber:  tn,
				Status:          domain.StatusCreated,
				DestinationCity: "Negombo",
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			return parcel, nil, nil // Empty history
		},
	}

	svc := NewPublicTrackingService(repo)
	res, err := svc.GetPublicTracking(context.Background(), "SD-20261106-000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Milestones) != 1 {
		t.Fatalf("expected 1 fallback milestone, got %d", len(res.Milestones))
	}
	if res.Milestones[0].Status != "CREATED" {
		t.Errorf("expected fallback milestone CREATED, got %s", res.Milestones[0].Status)
	}
}
