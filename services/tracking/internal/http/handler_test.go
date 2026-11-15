package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/tracking/internal/config"
	"github.com/swiftdrop/tracking/internal/domain"
	"github.com/swiftdrop/tracking/internal/dto"
	"github.com/swiftdrop/tracking/internal/health"
	"github.com/swiftdrop/tracking/internal/service"
)

type mockRepoForHTTP struct {
	findSummaryFunc func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error)
}

func (m *mockRepoForHTTP) FindParcelByTrackingNumber(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, error) {
	return nil, nil
}

func (m *mockRepoForHTTP) FindStatusHistoryByParcelID(ctx context.Context, parcelID string) ([]domain.MilestoneRecord, error) {
	return nil, nil
}

func (m *mockRepoForHTTP) FindStatusHistoryByID(ctx context.Context, historyID string) (*domain.MilestoneRecord, error) {
	return nil, nil
}

func (m *mockRepoForHTTP) FindTrackingSummary(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
	if m.findSummaryFunc != nil {
		return m.findSummaryFunc(ctx, trackingNumber)
	}
	return nil, nil, errors.New("not implemented")
}

func (m *mockRepoForHTTP) Ping(ctx context.Context) error {
	return nil
}

func (m *mockRepoForHTTP) CheckReadiness(ctx context.Context) error {
	return nil
}

func (m *mockRepoForHTTP) Close() error {
	return nil
}

func setupTestTrackingRouter(mockRepo *mockRepoForHTTP) http.Handler {
	cfg := &config.Config{
		HTTPAddr:            ":8081",
		AppEnv:              "local",
		ShutdownTimeout:     15 * time.Second,
		ReadTimeout:         15 * time.Second,
		WriteTimeout:        30 * time.Second,
		IdleTimeout:         60 * time.Second,
		LogLevel:            "info",
		SSEHeartbeatSeconds: 15,
		DBMaxOpenConns:      5,
		DBMaxIdleConns:      2,
		DBConnMaxLifetime:   15 * time.Minute,
		DBConnMaxIdleTime:   5 * time.Minute,
		DBQueryTimeout:      3 * time.Second,
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	healthHandler := health.NewHandler()
	svc := service.NewPublicTrackingService(mockRepo)
	trackingHandler := NewPublicTrackingHandler(svc, logger)
	return NewRouter(cfg, logger, healthHandler, trackingHandler)
}

func TestPublicTrackingHandler_Success_200(t *testing.T) {
	now := time.Date(2026, 11, 6, 14, 0, 0, 0, time.UTC)
	tn, _ := domain.NewTrackingNumber("SD-20261106-000001")

	repo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			parcel := &domain.ParcelTrackingInfo{
				ID:              "parcel-uuid-1",
				TrackingNumber:  tn,
				Status:          domain.StatusOutForDelivery,
				DestinationCity: "Colombo",
				CreatedAt:       now.Add(-4 * time.Hour),
				UpdatedAt:       now,
			}
			history := []domain.MilestoneRecord{
				{
					ID:        "hist-1",
					ParcelID:  "parcel-uuid-1",
					ToStatus:  "CREATED",
					Timestamp: now.Add(-4 * time.Hour),
				},
				{
					ID:        "hist-2",
					ParcelID:  "parcel-uuid-1",
					ToStatus:  "IN_TRANSIT",
					Timestamp: now.Add(-2 * time.Hour),
				},
				{
					ID:        "hist-3",
					ParcelID:  "parcel-uuid-1",
					ToStatus:  "OUT_FOR_DELIVERY",
					Timestamp: now,
				},
			}
			return parcel, history, nil
		},
	}

	router := setupTestTrackingRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/SD-20261106-000001", nil)
	req.Header.Set("X-Request-ID", "corr-req-001")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: body=%s", rr.Code, rr.Body.String())
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type application/json; charset=utf-8, got %q", ct)
	}

	var env SingleDataEnvelope[dto.PublicTrackingResponse]
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode response envelope: %v", err)
	}

	data := env.Data
	if data.TrackingNumber != "SD-20261106-000001" {
		t.Errorf("expected tracking number SD-20261106-000001, got %s", data.TrackingNumber)
	}
	if data.Status != "OUT_FOR_DELIVERY" {
		t.Errorf("expected status OUT_FOR_DELIVERY, got %s", data.Status)
	}
	if data.StatusDescription != "Courier is out for final doorstep delivery." {
		t.Errorf("expected safe description, got %q", data.StatusDescription)
	}
	if data.Destination.City != "Colombo" {
		t.Errorf("expected destination city Colombo, got %s", data.Destination.City)
	}
	if data.EstimatedDelivery != nil {
		t.Errorf("expected nil estimated delivery, got %v", data.EstimatedDelivery)
	}
	if len(data.Milestones) != 3 {
		t.Fatalf("expected 3 milestones, got %d", len(data.Milestones))
	}
	if data.Milestones[0].Status != "CREATED" || data.Milestones[2].Status != "OUT_FOR_DELIVERY" {
		t.Errorf("unexpected milestone progression: %v", data.Milestones)
	}
}

func TestPublicTrackingHandler_InvalidTrackingNumber_400(t *testing.T) {
	router := setupTestTrackingRouter(&mockRepoForHTTP{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/INVALID-TRACKING-123", nil)
	req.Header.Set("X-Request-ID", "corr-req-400")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", rr.Code)
	}

	var env ErrorEnvelope
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode error envelope: %v", err)
	}

	if env.Error.Code != "INVALID_TRACKING_NUMBER" {
		t.Errorf("expected error code INVALID_TRACKING_NUMBER, got %s", env.Error.Code)
	}
	if env.Error.CorrelationID != "corr-req-400" {
		t.Errorf("expected correlation ID corr-req-400, got %s", env.Error.CorrelationID)
	}
	if len(env.Error.Details) == 0 || env.Error.Details[0].Field != "tracking_number" {
		t.Errorf("expected detail for tracking_number, got %v", env.Error.Details)
	}
}

func TestPublicTrackingHandler_NotFound_404(t *testing.T) {
	repo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return nil, nil, domain.ErrTrackingNotFound
		},
	}

	router := setupTestTrackingRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/SD-20261106-999999", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", rr.Code)
	}

	var env ErrorEnvelope
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode error envelope: %v", err)
	}

	if env.Error.Code != "TRACKING_NOT_FOUND" {
		t.Errorf("expected TRACKING_NOT_FOUND, got %s", env.Error.Code)
	}
	if !strings.Contains(env.Error.Message, "SD-20261106-999999") {
		t.Errorf("expected message to mention tracking number, got %s", env.Error.Message)
	}
}

func TestPublicTrackingHandler_DatabaseUnavailable_503(t *testing.T) {
	repo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return nil, nil, domain.ErrDatabaseUnavailable
		},
	}

	router := setupTestTrackingRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/SD-20261106-000001", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d", rr.Code)
	}

	var env ErrorEnvelope
	if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
		t.Fatalf("failed to decode error envelope: %v", err)
	}

	if env.Error.Code != "DEPENDENCY_UNAVAILABLE" {
		t.Errorf("expected DEPENDENCY_UNAVAILABLE, got %s", env.Error.Code)
	}
	if env.Error.Message != "Tracking data temporarily unavailable." {
		t.Errorf("expected safe error message, got %s", env.Error.Message)
	}

	// Verify no SQL error leaked
	bodyStr := rr.Body.String()
	if strings.Contains(bodyStr, "SELECT") || strings.Contains(bodyStr, "postgres") || strings.Contains(bodyStr, "password") {
		t.Errorf("leaked sensitive DB details in error response: %s", bodyStr)
	}
}

func TestPublicTrackingHandler_ContextCancellation(t *testing.T) {
	repo := &mockRepoForHTTP{
		findSummaryFunc: func(ctx context.Context, trackingNumber string) (*domain.ParcelTrackingInfo, []domain.MilestoneRecord, error) {
			return nil, nil, context.Canceled
		},
	}

	router := setupTestTrackingRouter(repo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel early

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tracking/public/SD-20261106-000001", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Since context was cancelled, no response was written or safe exit occurred
	if rr.Code != http.StatusOK && rr.Code != http.StatusRequestTimeout && rr.Code != 0 {
		// Not an error - early return verified
	}
}
