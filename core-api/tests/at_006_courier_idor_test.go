package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
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

type idorMockStorage struct {
	assignments map[uuid.UUID]*deliveryApp.ActiveAssignmentDTO
	attempts    map[uuid.UUID][]*deliveryDomain.DeliveryAttempt
	statuses    map[uuid.UUID]string
	attemptNums map[uuid.UUID]int
	photos      map[string][]byte
}

func newIDORMockStorage() *idorMockStorage {
	return &idorMockStorage{
		assignments: make(map[uuid.UUID]*deliveryApp.ActiveAssignmentDTO),
		attempts:    make(map[uuid.UUID][]*deliveryDomain.DeliveryAttempt),
		statuses:    make(map[uuid.UUID]string),
		attemptNums: make(map[uuid.UUID]int),
		photos:      make(map[string][]byte),
	}
}

func (s *idorMockStorage) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return fn(nil)
}

func (s *idorMockStorage) GetActiveAssignment(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*deliveryApp.ActiveAssignmentDTO, error) {
	return s.assignments[parcelID], nil
}

func (s *idorMockStorage) CompleteAssignment(ctx context.Context, tx *sql.Tx, assignmentID uuid.UUID) error {
	for k, v := range s.assignments {
		if v.AssignmentID == assignmentID {
			v.Status = "COMPLETED"
			delete(s.assignments, k)
		}
	}
	return nil
}

func (s *idorMockStorage) InsertAttempt(ctx context.Context, tx *sql.Tx, attempt *deliveryDomain.DeliveryAttempt) error {
	s.attempts[attempt.ParcelID] = append(s.attempts[attempt.ParcelID], attempt)
	s.attemptNums[attempt.ParcelID] = attempt.AttemptNumber
	return nil
}

func (s *idorMockStorage) ListAttemptsByParcelID(ctx context.Context, parcelID uuid.UUID) ([]*deliveryDomain.DeliveryAttempt, error) {
	return s.attempts[parcelID], nil
}

func (s *idorMockStorage) GetAttemptCount(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (int, error) {
	return len(s.attempts[parcelID]), nil
}

func (s *idorMockStorage) TransitionStatus(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, fromStatus, toStatus string, actorID uuid.UUID, actorRole, note string) error {
	s.statuses[parcelID] = toStatus
	return nil
}

func (s *idorMockStorage) GetStatusWithLock(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (string, int, error) {
	st, ok := s.statuses[parcelID]
	if !ok {
		st = "ASSIGNED"
	}
	return st, s.attemptNums[parcelID], nil
}

func (s *idorMockStorage) Write(ctx context.Context, tx *sql.Tx, aggregateType string, aggregateID uuid.UUID, aggregateVersion int, eventType string, payload any, correlationID string) error {
	return nil
}

func (s *idorMockStorage) UploadProofPhoto(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	b, _ := io.ReadAll(data)
	s.photos[key] = b
	return key, nil
}

func (s *idorMockStorage) GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "https://cdn.swiftdrop.io/proofs/" + key, nil
}

func (s *idorMockStorage) GetActiveManifest(ctx context.Context, courierID uuid.UUID) ([]deliveryApp.CourierManifestItemDTO, error) {
	return nil, nil
}

// TestAT006_CourierIDORProtection verifies that only the legitimately assigned courier
// can perform delivery operations on a parcel; foreign couriers are rejected with 403 Forbidden.
func TestAT006_CourierIDORProtection(t *testing.T) {
	storage := newIDORMockStorage()

	courierA := uuid.New()
	courierB := uuid.New()
	parcelID := uuid.New()
	assignmentID := uuid.New()

	// Parcel is assigned exclusively to Courier A
	storage.assignments[parcelID] = &deliveryApp.ActiveAssignmentDTO{
		AssignmentID: assignmentID,
		ParcelID:     parcelID,
		CourierID:    courierA,
		Status:       "ACTIVE",
	}
	storage.statuses[parcelID] = "ASSIGNED"

	// Construct use cases & handlers
	uploadUC := deliveryApp.NewUploadProofPhotoUseCase(storage, storage)
	completeUC := deliveryApp.NewCompleteDeliveryUseCase(storage, storage, storage, storage, storage)
	failedUC := deliveryApp.NewRecordFailedAttemptUseCase(storage, storage, storage, storage, storage)
	historyQ := deliveryApp.NewGetDeliveryHistoryQueryHandler(storage, storage, storage)
	manifestQ := deliveryApp.NewGetCourierManifestQueryHandler(storage)
	startRunUC := deliveryApp.NewStartDeliveryRunUseCase(storage, storage, storage, storage)

	deliveryHandler := deliveryTransport.NewDeliveryHandler(uploadUC, completeUC, failedUC, historyQ)
	manifestHandler := deliveryTransport.NewManifestHandler(manifestQ, startRunUC)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/courier/parcels/{id}/start-run", manifestHandler.StartRun)
	mux.HandleFunc("POST /api/v1/deliveries/{id}/proof-photo", deliveryHandler.UploadProofPhoto)
	mux.HandleFunc("POST /api/v1/deliveries/{id}/complete", deliveryHandler.CompleteDelivery)
	mux.HandleFunc("POST /api/v1/deliveries/{id}/failed", deliveryHandler.RecordFailedAttempt)

	authAs := func(r *http.Request, courierID uuid.UUID) *http.Request {
		p := &identityApp.Principal{
			UserID: courierID.String(),
			Email:  "courier@swiftdrop.io",
			Role:   identityDomain.RoleCourier,
		}
		return r.WithContext(identityTransport.WithPrincipal(r.Context(), p))
	}

	t.Run("AT-006: Courier B cannot start delivery run for Courier A's parcel", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/courier/parcels/"+parcelID.String()+"/start-run", nil)
		req.SetPathValue("id", parcelID.String())
		req = authAs(req, courierB)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "COURIER_NOT_ASSIGNED")
	})

	t.Run("AT-006: Courier A successfully starts delivery run", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/courier/parcels/"+parcelID.String()+"/start-run", nil)
		req.SetPathValue("id", parcelID.String())
		req = authAs(req, courierA)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OUT_FOR_DELIVERY", storage.statuses[parcelID])
	})

	t.Run("AT-006: Courier B cannot upload POD photo for Courier A's parcel", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "proof.jpg")
		_, _ = io.WriteString(part, "fake-photo-data")
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+parcelID.String()+"/proof-photo", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.SetPathValue("id", parcelID.String())
		req = authAs(req, courierB)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "COURIER_NOT_ASSIGNED")
	})

	t.Run("AT-006: Courier B cannot complete delivery for Courier A's parcel", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]any{
			"proof_type":       "PHOTO",
			"proof_url":        "https://cdn.swiftdrop.io/proofs/test.jpg",
			"received_by_name": "Kasun Perera",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+parcelID.String()+"/complete", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", parcelID.String())
		req = authAs(req, courierB)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "COURIER_NOT_ASSIGNED")
	})

	t.Run("AT-006: Courier B cannot record failed attempt for Courier A's parcel", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]any{
			"failure_reason_code": "RECIPIENT_UNAVAILABLE",
			"notes":               "Customer not home",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+parcelID.String()+"/failed", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", parcelID.String())
		req = authAs(req, courierB)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "COURIER_NOT_ASSIGNED")
	})

	t.Run("AT-006: Courier A successfully completes delivery with valid POD", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]any{
			"proof_type":       "PHOTO",
			"proof_url":        "https://cdn.swiftdrop.io/proofs/test.jpg",
			"received_by_name": "Kasun Perera",
			"notes":            "Handed directly to recipient at front gate",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+parcelID.String()+"/complete", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", parcelID.String())
		req = authAs(req, courierA)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "DELIVERED", storage.statuses[parcelID])
	})
}

