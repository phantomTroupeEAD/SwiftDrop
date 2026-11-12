package transport_test

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

	"github.com/swiftdrop/core-api/internal/delivery/application"
	"github.com/swiftdrop/core-api/internal/delivery/domain"
	"github.com/swiftdrop/core-api/internal/delivery/transport"
	identityApp "github.com/swiftdrop/core-api/internal/identity/application"
	identityDomain "github.com/swiftdrop/core-api/internal/identity/domain"
	identityTransport "github.com/swiftdrop/core-api/internal/identity/transport"
)

type mockDeliveryTxManager struct{}

func (m *mockDeliveryTxManager) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return fn(nil)
}

type mockAssignmentFinder struct {
	assignments map[uuid.UUID]*application.ActiveAssignmentDTO
}

func (m *mockAssignmentFinder) GetActiveAssignment(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*application.ActiveAssignmentDTO, error) {
	return m.assignments[parcelID], nil
}

func (m *mockAssignmentFinder) CompleteAssignment(ctx context.Context, tx *sql.Tx, assignmentID uuid.UUID) error {
	for k, v := range m.assignments {
		if v.AssignmentID == assignmentID {
			v.Status = "COMPLETED"
			delete(m.assignments, k)
		}
	}
	return nil
}

type mockDeliveryAttemptRepo struct {
	attempts map[uuid.UUID][]*domain.DeliveryAttempt
}

func (m *mockDeliveryAttemptRepo) InsertAttempt(ctx context.Context, tx *sql.Tx, attempt *domain.DeliveryAttempt) error {
	m.attempts[attempt.ParcelID] = append(m.attempts[attempt.ParcelID], attempt)
	return nil
}

func (m *mockDeliveryAttemptRepo) ListAttemptsByParcelID(ctx context.Context, parcelID uuid.UUID) ([]*domain.DeliveryAttempt, error) {
	return m.attempts[parcelID], nil
}

func (m *mockDeliveryAttemptRepo) GetAttemptCount(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (int, error) {
	return len(m.attempts[parcelID]), nil
}

type mockDeliveryParcelUpdater struct {
	statuses map[uuid.UUID]string
	attempts map[uuid.UUID]int
}

func (m *mockDeliveryParcelUpdater) TransitionStatus(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, fromStatus, toStatus string, actorID uuid.UUID, actorRole, note string) error {
	m.statuses[parcelID] = toStatus
	return nil
}

func (m *mockDeliveryParcelUpdater) GetStatusWithLock(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (string, int, error) {
	s, ok := m.statuses[parcelID]
	if !ok {
		s = "OUT_FOR_DELIVERY"
	}
	return s, m.attempts[parcelID], nil
}

type mockDeliveryOutboxWriter struct{}

func (m *mockDeliveryOutboxWriter) Write(ctx context.Context, tx *sql.Tx, aggregateType string, aggregateID uuid.UUID, aggregateVersion int, eventType string, payload any, correlationID string) error {
	return nil
}

type mockManifestReader struct {
	items []application.CourierManifestItemDTO
}

func (m *mockManifestReader) GetActiveManifest(ctx context.Context, courierID uuid.UUID) ([]application.CourierManifestItemDTO, error) {
	return m.items, nil
}

type mockStorageClient struct{}

func (m *mockStorageClient) UploadProofPhoto(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	return key, nil
}

func (m *mockStorageClient) GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "https://cdn.swiftdrop.io/proofs/" + key, nil
}

func setupTestDeliveryHandler() (*transport.DeliveryHandler, *transport.ManifestHandler, uuid.UUID, uuid.UUID) {
	return setupTestDeliveryHandlerWithStatus("OUT_FOR_DELIVERY")
}

func setupTestDeliveryHandlerWithStatus(status string) (*transport.DeliveryHandler, *transport.ManifestHandler, uuid.UUID, uuid.UUID) {
	txMgr := &mockDeliveryTxManager{}
	finder := &mockAssignmentFinder{assignments: make(map[uuid.UUID]*application.ActiveAssignmentDTO)}
	attemptRepo := &mockDeliveryAttemptRepo{attempts: make(map[uuid.UUID][]*domain.DeliveryAttempt)}
	parcelUpdater := &mockDeliveryParcelUpdater{statuses: make(map[uuid.UUID]string), attempts: make(map[uuid.UUID]int)}
	outbox := &mockDeliveryOutboxWriter{}
	manifestReader := &mockManifestReader{}
	storage := &mockStorageClient{}

	courierID := uuid.New()
	parcelID := uuid.New()
	asgnID := uuid.New()

	finder.assignments[parcelID] = &application.ActiveAssignmentDTO{
		AssignmentID: asgnID,
		ParcelID:     parcelID,
		CourierID:    courierID,
		Status:       "ACTIVE",
	}
	parcelUpdater.statuses[parcelID] = status
	parcelUpdater.attempts[parcelID] = 0

	manifestReader.items = []application.CourierManifestItemDTO{
		{
			ParcelID:       parcelID.String(),
			TrackingNumber: "SD-20261102-0001",
			CurrentStatus:  status,
			RecipientName:  "Sunil Silva",
			DeliveryStreet: "123 Galle Road",
			DeliveryCity:   "Colombo",
		},
	}

	uploadUC := application.NewUploadProofPhotoUseCase(storage, finder)
	completeUC := application.NewCompleteDeliveryUseCase(txMgr, finder, attemptRepo, parcelUpdater, outbox)
	failedUC := application.NewRecordFailedAttemptUseCase(txMgr, finder, attemptRepo, parcelUpdater, outbox)
	historyQ := application.NewGetDeliveryHistoryQueryHandler(attemptRepo, parcelUpdater, storage)
	manifestQ := application.NewGetCourierManifestQueryHandler(manifestReader)
	startRunUC := application.NewStartDeliveryRunUseCase(txMgr, finder, parcelUpdater, outbox)

	dh := transport.NewDeliveryHandler(uploadUC, completeUC, failedUC, historyQ)
	mh := transport.NewManifestHandler(manifestQ, startRunUC)

	return dh, mh, courierID, parcelID
}

func withDeliveryAuth(r *http.Request, userID string, role identityDomain.Role) *http.Request {
	p := &identityApp.Principal{
		UserID: userID,
		Email:  "courier@swiftdrop.io",
		Role:   role,
	}
	return r.WithContext(identityTransport.WithPrincipal(r.Context(), p))
}

func TestDeliveryHandler_CompleteDelivery(t *testing.T) {
	t.Run("Success returns 200 OK", func(t *testing.T) {
		dh, _, courierID, parcelID := setupTestDeliveryHandler()
		reqBody := map[string]any{
			"proof_type":       "PHOTO",
			"proof_url":        "https://cdn.swiftdrop.io/proofs/sample.jpg",
			"received_by_name": "Kasun Perera",
		}
		b, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+parcelID.String()+"/complete", bytes.NewReader(b))
		req.SetPathValue("id", parcelID.String())
		req = withDeliveryAuth(req, courierID.String(), identityDomain.RoleCourier)
		rec := httptest.NewRecorder()

		dh.CompleteDelivery(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "DELIVERED")
	})

	t.Run("AT-006 IDOR defense: foreign courier rejected with 403 Forbidden", func(t *testing.T) {
		dh, _, _, parcelID := setupTestDeliveryHandler()
		reqBody := map[string]any{
			"proof_type":       "PHOTO",
			"proof_url":        "https://cdn.swiftdrop.io/proofs/sample.jpg",
			"received_by_name": "Kasun Perera",
		}
		b, _ := json.Marshal(reqBody)
		foreignCourierID := uuid.NewString()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+parcelID.String()+"/complete", bytes.NewReader(b))
		req.SetPathValue("id", parcelID.String())
		req = withDeliveryAuth(req, foreignCourierID, identityDomain.RoleCourier)
		rec := httptest.NewRecorder()

		dh.CompleteDelivery(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "COURIER_NOT_ASSIGNED")
	})
}

func TestDeliveryHandler_RecordFailedAttempt(t *testing.T) {
	dh, _, courierID, parcelID := setupTestDeliveryHandler()

	t.Run("Valid failed attempt returns 200 OK", func(t *testing.T) {
		reqBody := map[string]any{
			"failure_reason_code": "RECIPIENT_UNAVAILABLE",
			"notes":               "Customer not home; gate locked.",
		}
		b, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+parcelID.String()+"/failed", bytes.NewReader(b))
		req.SetPathValue("id", parcelID.String())
		req = withDeliveryAuth(req, courierID.String(), identityDomain.RoleCourier)
		rec := httptest.NewRecorder()

		dh.RecordFailedAttempt(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "ATTEMPT_FAILED")
	})

	t.Run("Invalid reason code returns 400 Bad Request", func(t *testing.T) {
		reqBody := map[string]any{
			"failure_reason_code": "INVALID_REASON_CODE_XYZ",
			"notes":               "Notes here",
		}
		b, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+parcelID.String()+"/failed", bytes.NewReader(b))
		req.SetPathValue("id", parcelID.String())
		req = withDeliveryAuth(req, courierID.String(), identityDomain.RoleCourier)
		rec := httptest.NewRecorder()

		dh.RecordFailedAttempt(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestDeliveryHandler_UploadProofPhoto(t *testing.T) {
	dh, _, courierID, parcelID := setupTestDeliveryHandler()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "proof.jpg")
	require.NoError(t, err)
	_, _ = io.WriteString(part, "fake-image-bytes-content")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+parcelID.String()+"/proof-photo", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetPathValue("id", parcelID.String())
	req = withDeliveryAuth(req, courierID.String(), identityDomain.RoleCourier)
	rec := httptest.NewRecorder()

	dh.UploadProofPhoto(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "photo_url")
}

func TestManifestHandler_GetManifest(t *testing.T) {
	_, mh, courierID, _ := setupTestDeliveryHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/courier/parcels", nil)
	req = withDeliveryAuth(req, courierID.String(), identityDomain.RoleCourier)
	rec := httptest.NewRecorder()

	mh.GetManifest(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Sunil Silva")
}

func TestManifestHandler_StartBatchRun(t *testing.T) {
	t.Run("Empty parcel list rejected with 422 Unprocessable Entity", func(t *testing.T) {
		_, mh, courierID, _ := setupTestDeliveryHandler()
		reqBody := map[string]any{
			"parcel_ids": []string{},
		}
		b, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/courier/runs/start-batch", bytes.NewReader(b))
		req = withDeliveryAuth(req, courierID.String(), identityDomain.RoleCourier)
		rec := httptest.NewRecorder()

		mh.StartBatchRun(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "VALIDATION_FAILED")
	})

	t.Run("Over 100 parcels rejected with 422 Unprocessable Entity", func(t *testing.T) {
		_, mh, courierID, _ := setupTestDeliveryHandler()
		excessiveIDs := make([]string, 101)
		for i := 0; i < 101; i++ {
			excessiveIDs[i] = uuid.NewString()
		}
		reqBody := map[string]any{
			"parcel_ids": excessiveIDs,
		}
		b, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/courier/runs/start-batch", bytes.NewReader(b))
		req = withDeliveryAuth(req, courierID.String(), identityDomain.RoleCourier)
		rec := httptest.NewRecorder()

		mh.StartBatchRun(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "VALIDATION_FAILED")
	})

	t.Run("Malformed JSON rejected with 400 Bad Request", func(t *testing.T) {
		_, mh, courierID, _ := setupTestDeliveryHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/courier/runs/start-batch", bytes.NewReader([]byte("{malformed-json")))
		req = withDeliveryAuth(req, courierID.String(), identityDomain.RoleCourier)
		rec := httptest.NewRecorder()

		mh.StartBatchRun(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "INVALID_REQUEST_BODY")
	})

	t.Run("Valid batch run succeeds with 200 OK", func(t *testing.T) {
		_, mh, courierID, parcelID := setupTestDeliveryHandlerWithStatus("ASSIGNED")
		reqBody := map[string]any{
			"parcel_ids": []string{parcelID.String()},
		}
		b, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/courier/runs/start-batch", bytes.NewReader(b))
		req = withDeliveryAuth(req, courierID.String(), identityDomain.RoleCourier)
		rec := httptest.NewRecorder()

		mh.StartBatchRun(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "started_parcels")
	})
}

func TestDeliveryHandler_CompleteDelivery_Signature(t *testing.T) {
	t.Run("Valid signature POD succeeds with 200 OK", func(t *testing.T) {
		dh, _, courierID, parcelID := setupTestDeliveryHandler()
		reqBody := map[string]any{
			"proof_type":       "SIGNATURE",
			"signature_data":   "M10,20 L30,40 L50,60 L70,80 L90,100 L110,120 L130,140 L150,160 L170,180",
			"received_by_name": "Kasun Perera",
		}
		b, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+parcelID.String()+"/complete", bytes.NewReader(b))
		req.SetPathValue("id", parcelID.String())
		req = withDeliveryAuth(req, courierID.String(), identityDomain.RoleCourier)
		rec := httptest.NewRecorder()

		dh.CompleteDelivery(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "DELIVERED")
	})

	t.Run("Short signature stroke rejected with 422 Unprocessable Entity", func(t *testing.T) {
		dh, _, courierID, parcelID := setupTestDeliveryHandler()
		reqBody := map[string]any{
			"proof_type":       "SIGNATURE",
			"signature_data":   "M10,20", // Under 50 chars
			"received_by_name": "Kasun Perera",
		}
		b, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/deliveries/"+parcelID.String()+"/complete", bytes.NewReader(b))
		req.SetPathValue("id", parcelID.String())
		req = withDeliveryAuth(req, courierID.String(), identityDomain.RoleCourier)
		rec := httptest.NewRecorder()

		dh.CompleteDelivery(rec, req)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "INVALID_SIGNATURE_DATA")
	})
}

func TestManifestHandler_DataMinimization(t *testing.T) {
	_, mh, courierID, _ := setupTestDeliveryHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/courier/parcels", nil)
	req = withDeliveryAuth(req, courierID.String(), identityDomain.RoleCourier)
	rec := httptest.NewRecorder()

	mh.GetManifest(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	// Security / Privacy: Data Minimization verifies no sensitive billing or merchant fee fields
	assert.NotContains(t, body, "delivery_fee")
	assert.NotContains(t, body, "wholesale_tariff")
	assert.NotContains(t, body, "merchant_profit")
	assert.NotContains(t, body, "credit_card")
}
