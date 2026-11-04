package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryApp "github.com/swiftdrop/core-api/internal/delivery/application"
	deliveryDomain "github.com/swiftdrop/core-api/internal/delivery/domain"
	deliveryTransport "github.com/swiftdrop/core-api/internal/delivery/transport"
	identityApp "github.com/swiftdrop/core-api/internal/identity/application"
	identityDomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityTransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

type ceilingMockStorage struct {
	assignments map[uuid.UUID]*deliveryApp.ActiveAssignmentDTO
	attempts    map[uuid.UUID][]*deliveryDomain.DeliveryAttempt
	statuses    map[uuid.UUID]string
	attemptNums map[uuid.UUID]int
}

func newCeilingMockStorage() *ceilingMockStorage {
	return &ceilingMockStorage{
		assignments: make(map[uuid.UUID]*deliveryApp.ActiveAssignmentDTO),
		attempts:    make(map[uuid.UUID][]*deliveryDomain.DeliveryAttempt),
		statuses:    make(map[uuid.UUID]string),
		attemptNums: make(map[uuid.UUID]int),
	}
}

func (s *ceilingMockStorage) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return fn(nil)
}

func (s *ceilingMockStorage) GetActiveAssignment(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*deliveryApp.ActiveAssignmentDTO, error) {
	return s.assignments[parcelID], nil
}

func (s *ceilingMockStorage) CompleteAssignment(ctx context.Context, tx *sql.Tx, assignmentID uuid.UUID) error {
	for k, v := range s.assignments {
		if v.AssignmentID == assignmentID {
			v.Status = "COMPLETED"
			delete(s.assignments, k)
		}
	}
	return nil
}

func (s *ceilingMockStorage) InsertAttempt(ctx context.Context, tx *sql.Tx, attempt *deliveryDomain.DeliveryAttempt) error {
	s.attempts[attempt.ParcelID] = append(s.attempts[attempt.ParcelID], attempt)
	s.attemptNums[attempt.ParcelID] = attempt.AttemptNumber
	return nil
}

func (s *ceilingMockStorage) ListAttemptsByParcelID(ctx context.Context, parcelID uuid.UUID) ([]*deliveryDomain.DeliveryAttempt, error) {
	return s.attempts[parcelID], nil
}

func (s *ceilingMockStorage) GetAttemptCount(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (int, error) {
	return len(s.attempts[parcelID]), nil
}

func (s *ceilingMockStorage) TransitionStatus(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, fromStatus, toStatus string, actorID uuid.UUID, actorRole, note string) error {
	s.statuses[parcelID] = toStatus
	return nil
}

func (s *ceilingMockStorage) GetStatusWithLock(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (string, int, error) {
	st, ok := s.statuses[parcelID]
	if !ok {
		st = "OUT_FOR_DELIVERY"
	}
	return st, s.attemptNums[parcelID], nil
}

func (s *ceilingMockStorage) Write(ctx context.Context, tx *sql.Tx, aggregateType string, aggregateID uuid.UUID, aggregateVersion int, eventType string, payload any, correlationID string) error {
	return nil
}

func (s *ceilingMockStorage) UploadProofPhoto(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	return key, nil
}

func (s *ceilingMockStorage) GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "/proofs/" + key, nil
}

// TestADR004_ThreeAttemptCeilingAndReturn verifies ADR-004:
// 1. Attempts 1 and 2 transition parcel to ATTEMPT_FAILED.
// 2. Attempt 3 transitions parcel to terminal RETURNED and closes assignment.
// 3. Any 4th attempt is strictly barred.
// 4. Reason REFUSED_BY_RECIPIENT triggers immediate RETURNED on Attempt 1.
func TestADR004_ThreeAttemptCeilingAndReturn(t *testing.T) {
	storage := newCeilingMockStorage()

	courierID := uuid.New()
	parcelID := uuid.New()
	assignmentID := uuid.New()

	storage.assignments[parcelID] = &deliveryApp.ActiveAssignmentDTO{
		AssignmentID: assignmentID,
		ParcelID:     parcelID,
		CourierID:    courierID,
		Status:       "ACTIVE",
	}
	storage.statuses[parcelID] = "OUT_FOR_DELIVERY"

	failedUC := deliveryApp.NewRecordFailedAttemptUseCase(storage, storage, storage, storage, storage)
	uploadUC := deliveryApp.NewUploadProofPhotoUseCase(storage, storage)
	completeUC := deliveryApp.NewCompleteDeliveryUseCase(storage, storage, storage, storage, storage)
	historyQ := deliveryApp.NewGetDeliveryHistoryQueryHandler(storage, storage, storage)

	deliveryHandler := deliveryTransport.NewDeliveryHandler(uploadUC, completeUC, failedUC, historyQ)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/deliveries/{id}/failed", deliveryHandler.RecordFailedAttempt)

	authAsCourier := func(r *http.Request) *http.Request {
		p := &identityApp.Principal{
			UserID: courierID.String(),
			Email:  "courier@swiftdrop.io",
			Role:   identityDomain.RoleCourier,
		}
		return r.WithContext(identityTransport.WithPrincipal(r.Context(), p))
	}

	postFailedAttempt := func(reason, notes string) (*httptest.ResponseRecorder, map[string]any) {
		payload, _ := json.Marshal(map[string]any{
			"failure_reason_code": reason,
			"notes":               notes,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+parcelID.String()+"/failed", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", parcelID.String())
		req = authAsCourier(req)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		var resp map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&resp)
		return rec, resp
	}

	// 1. Attempt 1: Recipient Unavailable -> ATTEMPT_FAILED
	rec1, resp1 := postFailedAttempt("RECIPIENT_UNAVAILABLE", "Premises locked, nobody home")
	assert.Equal(t, http.StatusOK, rec1.Code)
	data1 := resp1["data"].(map[string]any)
	assert.Equal(t, float64(1), data1["attempt_number"])
	assert.Equal(t, "ATTEMPT_FAILED", data1["parcel_status"])
	assert.Equal(t, false, data1["is_terminal"])
	assert.Equal(t, "ATTEMPT_FAILED", storage.statuses[parcelID])

	// Simulate courier resuming run
	storage.statuses[parcelID] = "OUT_FOR_DELIVERY"

	// 2. Attempt 2: Access Restricted -> ATTEMPT_FAILED
	rec2, resp2 := postFailedAttempt("ACCESS_RESTRICTED", "Security gate access refused")
	assert.Equal(t, http.StatusOK, rec2.Code)
	data2 := resp2["data"].(map[string]any)
	assert.Equal(t, float64(2), data2["attempt_number"])
	assert.Equal(t, "ATTEMPT_FAILED", data2["parcel_status"])
	assert.Equal(t, false, data2["is_terminal"])
	assert.Equal(t, "ATTEMPT_FAILED", storage.statuses[parcelID])

	// Simulate courier resuming run
	storage.statuses[parcelID] = "OUT_FOR_DELIVERY"

	// 3. Attempt 3: Customer Reschedule -> RETURNED (Terminal ceiling reached!)
	rec3, resp3 := postFailedAttempt("CUSTOMER_REQUESTED_RESCHEDULE", "Customer requested return to hub")
	assert.Equal(t, http.StatusOK, rec3.Code)
	data3 := resp3["data"].(map[string]any)
	assert.Equal(t, float64(3), data3["attempt_number"])
	assert.Equal(t, "RETURNED", data3["parcel_status"])
	assert.Equal(t, true, data3["is_terminal"])
	assert.Equal(t, "RETURNED", storage.statuses[parcelID])

	// 4. Attempt 4: Forbidden because assignment was closed & parcel is terminal
	rec4, _ := postFailedAttempt("RECIPIENT_UNAVAILABLE", "Attempt after terminal")
	assert.True(t, rec4.Code == http.StatusNotFound || rec4.Code == http.StatusConflict, "Attempt 4 must be rejected")

	// 5. Immediate Return on REFUSED_BY_RECIPIENT
	refusalParcelID := uuid.New()
	refusalAsgnID := uuid.New()
	storage.assignments[refusalParcelID] = &deliveryApp.ActiveAssignmentDTO{
		AssignmentID: refusalAsgnID,
		ParcelID:     refusalParcelID,
		CourierID:    courierID,
		Status:       "ACTIVE",
	}
	storage.statuses[refusalParcelID] = "OUT_FOR_DELIVERY"

	payloadRefusal, _ := json.Marshal(map[string]any{
		"failure_reason_code": "REFUSED_BY_RECIPIENT",
		"notes":               "Customer refused acceptance due to exterior packaging dent",
	})
	reqRefusal := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+refusalParcelID.String()+"/failed", bytes.NewReader(payloadRefusal))
	reqRefusal.Header.Set("Content-Type", "application/json")
	reqRefusal.SetPathValue("id", refusalParcelID.String())
	reqRefusal = authAsCourier(reqRefusal)
	recRefusal := httptest.NewRecorder()

	mux.ServeHTTP(recRefusal, reqRefusal)
	assert.Equal(t, http.StatusOK, recRefusal.Code)

	var respRefusal map[string]any
	_ = json.NewDecoder(recRefusal.Body).Decode(&respRefusal)
	dataRefusal := respRefusal["data"].(map[string]any)
	assert.Equal(t, float64(1), dataRefusal["attempt_number"])
	assert.Equal(t, "RETURNED", dataRefusal["parcel_status"], "Refused parcels must immediately transition to RETURNED")
	assert.Equal(t, true, dataRefusal["is_terminal"])
}

