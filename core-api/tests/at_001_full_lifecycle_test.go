package tests

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryApp "github.com/swiftdrop/core-api/internal/delivery/application"
	deliveryDomain "github.com/swiftdrop/core-api/internal/delivery/domain"
	dispatchDomain "github.com/swiftdrop/core-api/internal/dispatch/domain"
	parcelsdomain "github.com/swiftdrop/core-api/internal/parcels/domain"
)

// lifecycleMockStorage coordinates multi-module state transitions in-memory
// when running unit/offline tests, adhering to the Single Transaction Guarantee.
type lifecycleMockStorage struct {
	parcels     map[string]*parcelsdomain.Parcel
	assignments map[uuid.UUID]*dispatchDomain.CourierAssignment
	attempts    map[uuid.UUID][]*deliveryDomain.DeliveryAttempt
	outbox      []string
}

func newLifecycleMockStorage() *lifecycleMockStorage {
	return &lifecycleMockStorage{
		parcels:     make(map[string]*parcelsdomain.Parcel),
		assignments: make(map[uuid.UUID]*dispatchDomain.CourierAssignment),
		attempts:    make(map[uuid.UUID][]*deliveryDomain.DeliveryAttempt),
	}
}

func (s *lifecycleMockStorage) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return fn(nil)
}

func (s *lifecycleMockStorage) RecordEvent(eventType string) {
	s.outbox = append(s.outbox, eventType)
}

func (s *lifecycleMockStorage) TransitionStatus(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID, fromStatus, toStatus string, actorID uuid.UUID, actorRole, note string) error {
	s.RecordEvent("PARCEL_STATUS_TRANSITIONED:" + toStatus)
	return nil
}

func (s *lifecycleMockStorage) GetStatusWithLock(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (string, int, error) {
	return "OUT_FOR_DELIVERY", 1, nil
}

func (s *lifecycleMockStorage) GetMerchantID(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*string, error) {
	return nil, nil
}

func (s *lifecycleMockStorage) GetActiveAssignment(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (*deliveryApp.ActiveAssignmentDTO, error) {
	for _, a := range s.assignments {
		if a.ParcelID == parcelID && a.Status == dispatchDomain.AssignmentStatusActive {
			return &deliveryApp.ActiveAssignmentDTO{
				AssignmentID: a.ID,
				ParcelID:     a.ParcelID,
				CourierID:    a.CourierID,
				Status:       string(a.Status),
			}, nil
		}
	}
	return nil, nil
}

func (s *lifecycleMockStorage) CompleteAssignment(ctx context.Context, tx *sql.Tx, assignmentID uuid.UUID) error {
	for _, a := range s.assignments {
		if a.ID == assignmentID {
			a.Status = dispatchDomain.AssignmentStatusCompleted
		}
	}
	return nil
}

func (s *lifecycleMockStorage) InsertAttempt(ctx context.Context, tx *sql.Tx, attempt *deliveryDomain.DeliveryAttempt) error {
	s.attempts[attempt.ParcelID] = append(s.attempts[attempt.ParcelID], attempt)
	return nil
}

func (s *lifecycleMockStorage) ListAttemptsByParcelID(ctx context.Context, parcelID uuid.UUID) ([]*deliveryDomain.DeliveryAttempt, error) {
	return s.attempts[parcelID], nil
}

func (s *lifecycleMockStorage) GetAttemptCount(ctx context.Context, tx *sql.Tx, parcelID uuid.UUID) (int, error) {
	return len(s.attempts[parcelID]), nil
}

func (s *lifecycleMockStorage) Write(ctx context.Context, tx *sql.Tx, aggregateType string, aggregateID uuid.UUID, aggregateVersion int, eventType string, payload any, correlationID string) error {
	s.RecordEvent(eventType)
	return nil
}

func TestAcceptance_AT001_FullLifecycle(t *testing.T) {
	ctx := context.Background()
	storage := newLifecycleMockStorage()

	courierUUID := uuid.MustParse(SeedCourierUserID)
	parcelUUID := uuid.New()
	dispatcherUUID := uuid.MustParse(SeedDispatcherUserID)

	// 1. Step 1: Booking Creation (State: CREATED)
	t.Run("1. Parcel Booking with Valid Parameters", func(t *testing.T) {
		dim, err := parcelsdomain.NewDimensions(30.0, 25.0, 15.0)
		require.NoError(t, err)

		weight, err := parcelsdomain.NewWeight(2.5, 2.25, 2.5)
		require.NoError(t, err)

		pickup, err := parcelsdomain.NewAddress("Sender Alpha", "+94770000001", "123 Main St", "", "Colombo", "00100", "ZONE_WESTERN_01", "")
		require.NoError(t, err)

		dropoff, err := parcelsdomain.NewAddress("Recipient Beta", "+94710000002", "456 Kandy Rd", "", "Peradeniya", "20400", "ZONE_CENTRAL_01", "")
		require.NoError(t, err)

		price, err := parcelsdomain.NewMoney(55000, "LKR")
		require.NoError(t, err)

		trackingNum, err := parcelsdomain.FormatTrackingNumber(time.Now().UTC(), "A3X9K7")
		require.NoError(t, err)

		senderID := SeedCustomerUserID
		parcel, err := parcelsdomain.NewParcel(parcelsdomain.CreateParcelParams{
			ID:                 parcelUUID.String(),
			TrackingNumber:     trackingNum,
			SenderID:           &senderID,
			RecipientName:      "Recipient Beta",
			RecipientPhone:     "+94710000002",
			QuoteID:            "quote-12345",
			PricingRuleID:      "rule-1",
			PricingRuleVersion: 1,
			DeliveryFee:        price,
			OriginZoneID:       "ZONE_WESTERN_01",
			DestinationZoneID:  "ZONE_CENTRAL_01",
			ServiceLevel:       parcelsdomain.ServiceLevelStandard,
			Weight:             weight,
			Dimensions:         dim,
			PickupAddress:      pickup,
			DropoffAddress:     dropoff,
			CreatedAt:          time.Now().UTC(),
		})
		require.NoError(t, err)
		assert.Equal(t, parcelsdomain.StatusCreated, parcel.Status)
		storage.parcels[parcelUUID.String()] = &parcel
		storage.RecordEvent("PARCEL_CREATED")
	})

	// 2. Step 2: Dispatcher Assignment (State: ASSIGNED)
	t.Run("2. Dispatcher Assigns Courier", func(t *testing.T) {
		asgn := dispatchDomain.NewCourierAssignment(
			parcelUUID,
			courierUUID,
			dispatcherUUID,
			"ZONE_CENTRAL_01",
		)
		require.NotNil(t, asgn)
		require.Equal(t, dispatchDomain.AssignmentStatusActive, asgn.Status)
		storage.assignments[asgn.ID] = asgn
		storage.RecordEvent("COURIER_ASSIGNED")
	})

	// 3. Step 3: Courier Pick Up & Start Doorstep Run (State: OUT_FOR_DELIVERY)
	t.Run("3. Courier Transitions to Out for Delivery", func(t *testing.T) {
		err := storage.TransitionStatus(ctx, nil, parcelUUID, "ASSIGNED", "OUT_FOR_DELIVERY", courierUUID, "COURIER", "Courier started delivery run")
		require.NoError(t, err)
	})

	// 4. Step 4: Doorstep Delivery & Proof Verification (State: DELIVERED)
	t.Run("4. Courier Completes Delivery with Proof", func(t *testing.T) {
		completeUC := deliveryApp.NewCompleteDeliveryUseCase(
			storage,
			storage,
			storage,
			storage,
			storage,
		)

		lat := 6.9271
		lng := 79.8612
		cmd := deliveryApp.CompleteDeliveryCommand{
			ParcelID:       parcelUUID.String(),
			CourierID:      courierUUID.String(),
			ProofType:      string(deliveryDomain.ProofTypePhoto),
			ProofURL:       "https://storage.swiftdrop.local/pod/photo-001.jpg",
			ReceivedByName: "Recipient Beta",
			Latitude:       &lat,
			Longitude:      &lng,
		}

		res, err := completeUC.Execute(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "DELIVERED", res.Status)
	})

	// 5. Invariant Check: Verify Outbox Sequence
	t.Run("5. Outbox Event Sequence Verification", func(t *testing.T) {
		expectedEvents := []string{
			"PARCEL_CREATED",
			"COURIER_ASSIGNED",
			"PARCEL_STATUS_TRANSITIONED:OUT_FOR_DELIVERY",
			"PARCEL_STATUS_TRANSITIONED:DELIVERED",
			"parcel.delivered.v1",
		}
		for _, ev := range expectedEvents {
			assert.Contains(t, storage.outbox, ev, "expected outbox to contain %s", ev)
		}
	})
}
