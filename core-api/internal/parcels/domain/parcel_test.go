package domain

import (
	"errors"
	"testing"
	"time"
)

func validTestParcelParams() CreateParcelParams {
	now := time.Date(2026, time.November, 6, 10, 0, 0, 0, time.UTC)
	tn, _ := NewTrackingNumber("SD-20261106-A3X9K7")
	fee, _ := NewMoney(47000, CurrencyLKR)
	weight, _ := NewWeight(2.0, 2.5, 2.5)
	dims, _ := NewDimensions(30.0, 25.0, 15.0)

	pickupAddr, _ := NewAddress(
		"Kasun Perera",
		"+94771234567",
		"No 45, Galle Road",
		"Kollupitiya",
		"Colombo",
		"00300",
		"ZONE_WESTERN_01",
		"Front reception desk",
	)

	dropoffAddr, _ := NewAddress(
		"Nimali Silva",
		"+94719876543",
		"No 12/A, Kandy Road",
		"Peradeniya",
		"Kandy",
		"20400",
		"ZONE_CENTRAL_01",
		"Call receiver before arrival",
	)

	senderID := "customer-uuid-001"
	merchantID := "merchant-999"

	return CreateParcelParams{
		ID:                 "parcel-uuid-001",
		TrackingNumber:     tn,
		SenderID:           &senderID,
		MerchantID:         &merchantID,
		RecipientName:      "Nimali Silva",
		RecipientPhone:     "+94719876543",
		QuoteID:            "quote-uuid-001",
		PricingRuleID:      "rule-uuid-001",
		PricingRuleVersion: 2,
		DeliveryFee:        fee,
		OriginZoneID:       "ZONE_WESTERN_01",
		DestinationZoneID:  "ZONE_CENTRAL_01",
		ServiceLevel:       ServiceLevelStandard,
		Weight:             weight,
		Dimensions:         dims,
		PickupAddress:      pickupAddr,
		DropoffAddress:     dropoffAddr,
		CreatedAt:          now,
	}
}

func TestNewParcel_Success(t *testing.T) {
	params := validTestParcelParams()
	parcel, err := NewParcel(params)
	if err != nil {
		t.Fatalf("expected NewParcel to succeed, got: %v", err)
	}

	if parcel.ID != "parcel-uuid-001" {
		t.Errorf("expected ID parcel-uuid-001, got %s", parcel.ID)
	}
	if parcel.TrackingNumber != params.TrackingNumber {
		t.Errorf("expected tracking number %s, got %s", params.TrackingNumber, parcel.TrackingNumber)
	}
	if parcel.SenderID == nil || *parcel.SenderID != "customer-uuid-001" {
		t.Errorf("expected SenderID customer-uuid-001, got %v", parcel.SenderID)
	}
	if parcel.MerchantID == nil || *parcel.MerchantID != "merchant-999" {
		t.Errorf("expected MerchantID merchant-999, got %v", parcel.MerchantID)
	}
	if parcel.Status != StatusCreated {
		t.Errorf("expected initial status CREATED, got %s", parcel.Status)
	}
	if parcel.AttemptCount != 0 {
		t.Errorf("expected 0 delivery attempts, got %d", parcel.AttemptCount)
	}
	if parcel.Version != 1 {
		t.Errorf("expected initial version 1, got %d", parcel.Version)
	}
	if !parcel.CreatedAt.Equal(params.CreatedAt) {
		t.Errorf("expected CreatedAt %v, got %v", params.CreatedAt, parcel.CreatedAt)
	}
	if !parcel.UpdatedAt.Equal(params.CreatedAt) {
		t.Errorf("expected UpdatedAt %v, got %v", params.CreatedAt, parcel.UpdatedAt)
	}
	if parcel.DeliveryFee.Amount != 47000 {
		t.Errorf("expected fee 47000, got %d", parcel.DeliveryFee.Amount)
	}
	if parcel.DeliveryFee.Currency != CurrencyLKR {
		t.Errorf("expected currency LKR, got %s", parcel.DeliveryFee.Currency)
	}
	if parcel.PricingRuleVersion != 2 {
		t.Errorf("expected rule version 2, got %d", parcel.PricingRuleVersion)
	}
}

func TestNewParcel_OwnershipModes(t *testing.T) {
	t.Run("sender-only parcel", func(t *testing.T) {
		params := validTestParcelParams()
		params.SenderID = strPtr(" sender-001 ")
		params.MerchantID = nil

		parcel, err := NewParcel(params)
		if err != nil {
			t.Fatalf("expected sender-only parcel to succeed, got %v", err)
		}
		if parcel.SenderID == nil || *parcel.SenderID != "sender-001" {
			t.Errorf("expected trimmed SenderID sender-001, got %v", parcel.SenderID)
		}
		if parcel.MerchantID != nil {
			t.Errorf("expected nil MerchantID, got %v", parcel.MerchantID)
		}
	})

	t.Run("merchant-only parcel keeps SenderID nil", func(t *testing.T) {
		params := validTestParcelParams()
		params.SenderID = nil
		params.MerchantID = strPtr(" merchant-001 ")

		parcel, err := NewParcel(params)
		if err != nil {
			t.Fatalf("expected merchant-only parcel to succeed, got %v", err)
		}
		if parcel.SenderID != nil {
			t.Errorf("expected nil SenderID without dummy value, got %v", *parcel.SenderID)
		}
		if parcel.MerchantID == nil || *parcel.MerchantID != "merchant-001" {
			t.Errorf("expected trimmed MerchantID merchant-001, got %v", parcel.MerchantID)
		}
	})

	t.Run("sender-plus-merchant parcel", func(t *testing.T) {
		params := validTestParcelParams()
		parcel, err := NewParcel(params)
		if err != nil {
			t.Fatalf("expected combined owner parcel to succeed, got %v", err)
		}
		if parcel.SenderID == nil || parcel.MerchantID == nil {
			t.Errorf("expected both owners set, got sender=%v merchant=%v", parcel.SenderID, parcel.MerchantID)
		}
	})

	t.Run("missing both owners fails", func(t *testing.T) {
		params := validTestParcelParams()
		params.SenderID = nil
		params.MerchantID = nil

		_, err := NewParcel(params)
		if !errors.Is(err, ErrInvalidOwner) {
			t.Fatalf("expected ErrInvalidOwner, got %v", err)
		}
	})

	t.Run("non-nil whitespace sender fails even when merchant set", func(t *testing.T) {
		params := validTestParcelParams()
		params.SenderID = strPtr("   ")

		_, err := NewParcel(params)
		if !errors.Is(err, ErrInvalidSenderID) {
			t.Fatalf("expected ErrInvalidSenderID, got %v", err)
		}
	})

	t.Run("non-nil whitespace merchant fails even when sender set", func(t *testing.T) {
		params := validTestParcelParams()
		params.MerchantID = strPtr("   ")

		_, err := NewParcel(params)
		if !errors.Is(err, ErrInvalidMerchantID) {
			t.Fatalf("expected ErrInvalidMerchantID, got %v", err)
		}
	})
}

func TestNewParcel_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CreateParcelParams)
		wantErr error
	}{
		{
			name: "empty parcel id",
			mutate: func(p *CreateParcelParams) {
				p.ID = "  "
			},
			wantErr: ErrInvalidParcelID,
		},
		{
			name: "invalid tracking number",
			mutate: func(p *CreateParcelParams) {
				p.TrackingNumber = TrackingNumber("INVALID")
			},
			wantErr: ErrInvalidTrackingNumber,
		},
		{
			name: "non-nil empty sender id",
			mutate: func(p *CreateParcelParams) {
				p.SenderID = strPtr("")
			},
			wantErr: ErrInvalidSenderID,
		},
		{
			name: "missing both sender id and merchant id",
			mutate: func(p *CreateParcelParams) {
				p.SenderID = nil
				p.MerchantID = nil
			},
			wantErr: ErrInvalidOwner,
		},
		{
			name: "non-nil whitespace merchant id",
			mutate: func(p *CreateParcelParams) {
				p.MerchantID = strPtr("   ")
			},
			wantErr: ErrInvalidMerchantID,
		},
		{
			name: "empty recipient name",
			mutate: func(p *CreateParcelParams) {
				p.RecipientName = ""
			},
			wantErr: ErrInvalidRecipient,
		},
		{
			name: "empty quote id",
			mutate: func(p *CreateParcelParams) {
				p.QuoteID = " "
			},
			wantErr: ErrInvalidQuoteID,
		},
		{
			name: "empty pricing rule id",
			mutate: func(p *CreateParcelParams) {
				p.PricingRuleID = ""
			},
			wantErr: ErrInvalidPricingRule,
		},
		{
			name: "non-positive pricing rule version",
			mutate: func(p *CreateParcelParams) {
				p.PricingRuleVersion = 0
			},
			wantErr: ErrInvalidPricingRule,
		},
		{
			name: "invalid delivery fee currency",
			mutate: func(p *CreateParcelParams) {
				p.DeliveryFee = Money{Amount: 47000, Currency: "USD"}
			},
			wantErr: ErrInvalidDeliveryFee,
		},
		{
			name: "empty origin zone",
			mutate: func(p *CreateParcelParams) {
				p.OriginZoneID = ""
			},
			wantErr: ErrInvalidZone,
		},
		{
			name: "invalid origin zone format",
			mutate: func(p *CreateParcelParams) {
				p.OriginZoneID = "ZONE-WESTERN-01"
			},
			wantErr: ErrInvalidZone,
		},
		{
			name: "invalid destination zone format",
			mutate: func(p *CreateParcelParams) {
				p.DestinationZoneID = "zone_central_01"
			},
			wantErr: ErrInvalidZone,
		},
		{
			name: "origin zone mismatch with pickup address",
			mutate: func(p *CreateParcelParams) {
				p.OriginZoneID = "ZONE_SOUTHERN_01"
			},
			wantErr: ErrZoneMismatch,
		},
		{
			name: "destination zone mismatch with dropoff address",
			mutate: func(p *CreateParcelParams) {
				p.DestinationZoneID = "ZONE_NORTHERN_01"
			},
			wantErr: ErrZoneMismatch,
		},
		{
			name: "zero created_at",
			mutate: func(p *CreateParcelParams) {
				p.CreatedAt = time.Time{}
			},
			wantErr: ErrInvalidTimestamp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := validTestParcelParams()
			tt.mutate(&params)
			_, err := NewParcel(params)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.name)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error wrapping %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestReconstituteParcel(t *testing.T) {
	params := validTestParcelParams()
	now := params.CreatedAt
	later := now.Add(2 * time.Hour)

	reconstituteParams := ReconstituteParcelParams{
		ID:                 params.ID,
		TrackingNumber:     params.TrackingNumber,
		SenderID:           params.SenderID,
		MerchantID:         params.MerchantID,
		RecipientName:      params.RecipientName,
		RecipientPhone:     params.RecipientPhone,
		QuoteID:            params.QuoteID,
		PricingRuleID:      params.PricingRuleID,
		PricingRuleVersion: params.PricingRuleVersion,
		DeliveryFee:        params.DeliveryFee,
		OriginZoneID:       params.OriginZoneID,
		DestinationZoneID:  params.DestinationZoneID,
		ServiceLevel:       params.ServiceLevel,
		Weight:             params.Weight,
		Dimensions:         params.Dimensions,
		PickupAddress:      params.PickupAddress,
		DropoffAddress:     params.DropoffAddress,
		Status:             StatusInTransit,
		AttemptCount:       1,
		Version:            4,
		CreatedAt:          now,
		UpdatedAt:          later,
	}

	parcel, err := ReconstituteParcel(reconstituteParams)
	if err != nil {
		t.Fatalf("unexpected reconstitution error: %v", err)
	}

	if parcel.Status != StatusInTransit {
		t.Errorf("expected reconstituted status IN_TRANSIT, got %s", parcel.Status)
	}
	if parcel.Version != 4 {
		t.Errorf("expected reconstituted version 4, got %d", parcel.Version)
	}
	if parcel.AttemptCount != 1 {
		t.Errorf("expected 1 delivery attempt, got %d", parcel.AttemptCount)
	}

	// Reconstitute merchant-only parcel (SenderID == nil)
	merchantOnlyParams := reconstituteParams
	merchantOnlyParams.SenderID = nil
	merchantParcel, err := ReconstituteParcel(merchantOnlyParams)
	if err != nil {
		t.Fatalf("unexpected reconstitution error for merchant-only parcel: %v", err)
	}
	if merchantParcel.SenderID != nil {
		t.Errorf("expected nil SenderID for merchant-only reconstituted parcel, got %v", merchantParcel.SenderID)
	}

	// Reconstitution error on negative delivery attempts
	reconstituteParams.AttemptCount = -1
	_, err = ReconstituteParcel(reconstituteParams)
	if !errors.Is(err, ErrInvalidParcel) {
		t.Errorf("expected ErrInvalidParcel for negative attempts, got: %v", err)
	}
}

func TestParcel_LifecycleHappyPath(t *testing.T) {
	parcel, err := NewParcel(validTestParcelParams())
	if err != nil {
		t.Fatalf("failed to create parcel: %v", err)
	}

	t0 := parcel.CreatedAt
	t1 := t0.Add(10 * time.Minute)
	t2 := t0.Add(30 * time.Minute)
	t3 := t0.Add(2 * time.Hour)
	t4 := t0.Add(4 * time.Hour)
	t5 := t0.Add(5 * time.Hour)

	// 1. CREATED -> ASSIGNED
	entry1, err := parcel.Assign("courier-001", t1)
	if err != nil {
		t.Fatalf("Assign failed: %v", err)
	}
	if parcel.Status != StatusAssigned || parcel.Version != 2 || !parcel.UpdatedAt.Equal(t1) {
		t.Errorf("invalid state after Assign: status=%s, ver=%d", parcel.Status, parcel.Version)
	}
	if *entry1.FromStatus != StatusCreated || entry1.ToStatus != StatusAssigned {
		t.Errorf("unexpected history entry 1: %+v", entry1)
	}

	// 2. ASSIGNED -> PICKED_UP
	entry2, err := parcel.PickUp("courier-001", t2)
	if err != nil {
		t.Fatalf("PickUp failed: %v", err)
	}
	if parcel.Status != StatusPickedUp || parcel.Version != 3 || !parcel.UpdatedAt.Equal(t2) {
		t.Errorf("invalid state after PickUp: status=%s, ver=%d", parcel.Status, parcel.Version)
	}
	if *entry2.FromStatus != StatusAssigned || entry2.ToStatus != StatusPickedUp {
		t.Errorf("unexpected history entry 2: %+v", entry2)
	}

	// 3. PICKED_UP -> IN_TRANSIT
	entry3, err := parcel.MarkInTransit(strPtr("hub-op-001"), ActorTypeUser, "HUB_OPERATOR", t3)
	if err != nil {
		t.Fatalf("MarkInTransit failed: %v", err)
	}
	if parcel.Status != StatusInTransit || parcel.Version != 4 {
		t.Errorf("invalid state after MarkInTransit: status=%s, ver=%d", parcel.Status, parcel.Version)
	}
	if *entry3.FromStatus != StatusPickedUp || entry3.ToStatus != StatusInTransit {
		t.Errorf("unexpected history entry 3: %+v", entry3)
	}
	if entry3.ActorType != ActorTypeUser || entry3.ActorID == nil || *entry3.ActorID != "hub-op-001" || !entry3.OccurredAt.Equal(t3) {
		t.Errorf("unexpected history entry 3 actor/time metadata: %+v", entry3)
	}

	// 4. IN_TRANSIT -> OUT_FOR_DELIVERY
	entry4, err := parcel.DispatchForDelivery("courier-002", t4)
	if err != nil {
		t.Fatalf("DispatchForDelivery failed: %v", err)
	}
	if parcel.Status != StatusOutForDelivery || parcel.Version != 5 {
		t.Errorf("invalid state after DispatchForDelivery: status=%s, ver=%d", parcel.Status, parcel.Version)
	}
	if *entry4.FromStatus != StatusInTransit || entry4.ToStatus != StatusOutForDelivery {
		t.Errorf("unexpected history entry 4: %+v", entry4)
	}

	// 5. OUT_FOR_DELIVERY -> DELIVERED
	entry5, err := parcel.ConfirmDelivered("courier-002", "Recipient signature captured", t5)
	if err != nil {
		t.Fatalf("ConfirmDelivered failed: %v", err)
	}
	if parcel.Status != StatusDelivered || parcel.Version != 6 || !parcel.UpdatedAt.Equal(t5) {
		t.Errorf("invalid state after ConfirmDelivered: status=%s, ver=%d", parcel.Status, parcel.Version)
	}
	if *entry5.FromStatus != StatusOutForDelivery || entry5.ToStatus != StatusDelivered {
		t.Errorf("unexpected history entry 5: %+v", entry5)
	}

	// 6. Terminal state check: cannot transition from DELIVERED
	_, err = parcel.TransitionTo(StatusInTransit, strPtr("courier-002"), ActorTypeUser, "COURIER", "Try reopen", t5.Add(time.Hour))
	if !errors.Is(err, ErrTerminalState) {
		t.Fatalf("expected ErrTerminalState after DELIVERED, got: %v", err)
	}
}

func TestParcel_CancellationRules(t *testing.T) {
	// 1. Cancellation while CREATED is permitted
	parcel1, _ := NewParcel(validTestParcelParams())
	t1 := parcel1.CreatedAt.Add(5 * time.Minute)
	entry, err := parcel1.Cancel(strPtr("user-001"), ActorTypeUser, "CUSTOMER", "Customer changed mind", t1)
	if err != nil {
		t.Fatalf("Cancel failed while CREATED: %v", err)
	}
	if parcel1.Status != StatusCancelled || parcel1.Version != 2 {
		t.Errorf("expected CANCELLED, got %s (ver %d)", parcel1.Status, parcel1.Version)
	}
	if *entry.FromStatus != StatusCreated || entry.ToStatus != StatusCancelled {
		t.Errorf("unexpected history entry: %+v", entry)
	}

	// Terminal check: cannot transition from CANCELLED
	_, err = parcel1.Assign("courier-001", t1.Add(time.Minute))
	if !errors.Is(err, ErrTerminalState) {
		t.Fatalf("expected ErrTerminalState from CANCELLED, got: %v", err)
	}

	// Merchant API cancellation while CREATED is permitted
	parcelMerchant, _ := NewParcel(validTestParcelParams())
	merchantEntry, err := parcelMerchant.Cancel(strPtr("merchant-999"), ActorTypeMerchantAPI, "MERCHANT", "Order cancelled via webhook", parcelMerchant.CreatedAt.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("Cancel via MERCHANT_API failed while CREATED: %v", err)
	}
	if merchantEntry.ActorType != ActorTypeMerchantAPI || merchantEntry.ActorID == nil || *merchantEntry.ActorID != "merchant-999" {
		t.Errorf("unexpected merchant cancellation metadata: %+v", merchantEntry)
	}

	// 2. Cancellation once ASSIGNED is rejected
	parcel2, _ := NewParcel(validTestParcelParams())
	_, _ = parcel2.Assign("courier-001", parcel2.CreatedAt.Add(5*time.Minute))
	_, err = parcel2.Cancel(strPtr("user-001"), ActorTypeUser, "CUSTOMER", "Customer requested cancellation", parcel2.CreatedAt.Add(10*time.Minute))
	if !errors.Is(err, ErrCancellationNotAllowed) {
		t.Fatalf("expected ErrCancellationNotAllowed while ASSIGNED, got: %v", err)
	}

	// 3. Cancellation once PICKED_UP is rejected
	parcel3, _ := NewParcel(validTestParcelParams())
	_, _ = parcel3.Assign("courier-001", parcel3.CreatedAt.Add(5*time.Minute))
	_, _ = parcel3.PickUp("courier-001", parcel3.CreatedAt.Add(10*time.Minute))
	_, err = parcel3.Cancel(strPtr("user-001"), ActorTypeUser, "CUSTOMER", "Customer requested cancellation", parcel3.CreatedAt.Add(15*time.Minute))
	if !errors.Is(err, ErrCancellationNotAllowed) {
		t.Fatalf("expected ErrCancellationNotAllowed while PICKED_UP, got: %v", err)
	}
}

func TestParcel_DeliveryAttemptLimitsAndReturn(t *testing.T) {
	parcel, _ := NewParcel(validTestParcelParams())
	now := parcel.CreatedAt

	// Advance to OUT_FOR_DELIVERY
	_, _ = parcel.Assign("courier-001", now.Add(10*time.Minute))
	_, _ = parcel.PickUp("courier-001", now.Add(20*time.Minute))
	_, _ = parcel.MarkInTransit(strPtr("hub-001"), ActorTypeUser, "HUB", now.Add(30*time.Minute))
	_, _ = parcel.DispatchForDelivery("courier-002", now.Add(40*time.Minute))

	// Attempt 1 fails
	tFail1 := now.Add(50 * time.Minute)
	_, err := parcel.RecordFailedAttempt("courier-002", "RECIPIENT_UNAVAILABLE", tFail1)
	if err != nil {
		t.Fatalf("failed attempt 1: %v", err)
	}
	if parcel.Status != StatusAttemptFailed || parcel.AttemptCount != 1 {
		t.Fatalf("expected ATTEMPT_FAILED with 1 attempt, got %s (%d)", parcel.Status, parcel.AttemptCount)
	}

	// Reattempt 1 permitted (attempts = 1 < 3)
	tReattempt1 := now.Add(2 * time.Hour)
	_, err = parcel.DispatchForDelivery("courier-002", tReattempt1)
	if err != nil {
		t.Fatalf("reattempt 1 dispatch failed: %v", err)
	}
	if parcel.Status != StatusOutForDelivery {
		t.Fatalf("expected OUT_FOR_DELIVERY, got %s", parcel.Status)
	}

	// Attempt 2 fails
	tFail2 := now.Add(3 * time.Hour)
	_, err = parcel.RecordFailedAttempt("courier-002", "ACCESS_RESTRICTED", tFail2)
	if err != nil {
		t.Fatalf("failed attempt 2: %v", err)
	}
	if parcel.Status != StatusAttemptFailed || parcel.AttemptCount != 2 {
		t.Fatalf("expected ATTEMPT_FAILED with 2 attempts, got %s (%d)", parcel.Status, parcel.AttemptCount)
	}

	// Reattempt 2 permitted (attempts = 2 < 3)
	tReattempt2 := now.Add(4 * time.Hour)
	_, err = parcel.DispatchForDelivery("courier-002", tReattempt2)
	if err != nil {
		t.Fatalf("reattempt 2 dispatch failed: %v", err)
	}

	// Attempt 3 fails
	tFail3 := now.Add(5 * time.Hour)
	_, err = parcel.RecordFailedAttempt("courier-002", "RECIPIENT_UNAVAILABLE", tFail3)
	if err != nil {
		t.Fatalf("failed attempt 3: %v", err)
	}
	if parcel.Status != StatusAttemptFailed || parcel.AttemptCount != 3 {
		t.Fatalf("expected ATTEMPT_FAILED with 3 attempts, got %s (%d)", parcel.Status, parcel.AttemptCount)
	}

	// 4th reattempt MUST BE REJECTED (max 3 delivery attempts exceeded)
	tReattempt3 := now.Add(6 * time.Hour)
	_, err = parcel.DispatchForDelivery("courier-002", tReattempt3)
	if !errors.Is(err, ErrMaxDeliveryAttemptsExceeded) {
		t.Fatalf("expected ErrMaxDeliveryAttemptsExceeded for 4th reattempt, got: %v", err)
	}

	// Eligible to transition to RETURNED via SYSTEM after 3 failed attempts
	tReturn := now.Add(7 * time.Hour)
	entryReturn, err := parcel.ReturnToSender(nil, ActorTypeSystem, "SYSTEM", "Maximum delivery attempts exceeded; returning to sender", tReturn)
	if err != nil {
		t.Fatalf("ReturnToSender failed: %v", err)
	}
	if parcel.Status != StatusReturned {
		t.Errorf("expected RETURNED, got %s", parcel.Status)
	}
	if *entryReturn.FromStatus != StatusAttemptFailed || entryReturn.ToStatus != StatusReturned {
		t.Errorf("unexpected return entry: %+v", entryReturn)
	}
	if entryReturn.ActorType != ActorTypeSystem || entryReturn.ActorID != nil || !entryReturn.OccurredAt.Equal(tReturn) {
		t.Errorf("unexpected SYSTEM return actor metadata: %+v", entryReturn)
	}

	// Terminal check: cannot transition from RETURNED
	_, err = parcel.DispatchForDelivery("courier-002", tReturn.Add(time.Hour))
	if !errors.Is(err, ErrTerminalState) {
		t.Fatalf("expected ErrTerminalState from RETURNED, got: %v", err)
	}
}

func TestParcel_PriceSnapshotImmutability(t *testing.T) {
	params := validTestParcelParams()
	parcel, err := NewParcel(params)
	if err != nil {
		t.Fatalf("failed to create parcel: %v", err)
	}

	origFee := parcel.DeliveryFee
	origQuoteID := parcel.QuoteID
	origRuleID := parcel.PricingRuleID
	origVersion := parcel.PricingRuleVersion

	// Execute lifecycle transitions
	t1 := parcel.CreatedAt.Add(10 * time.Minute)
	_, _ = parcel.Assign("courier-001", t1)

	t2 := parcel.CreatedAt.Add(20 * time.Minute)
	_, _ = parcel.PickUp("courier-001", t2)

	// Ensure pricing snapshot has not mutated
	if parcel.DeliveryFee != origFee {
		t.Errorf("delivery fee was mutated: was %v, now %v", origFee, parcel.DeliveryFee)
	}
	if parcel.QuoteID != origQuoteID {
		t.Errorf("quote id was mutated: was %s, now %s", origQuoteID, parcel.QuoteID)
	}
	if parcel.PricingRuleID != origRuleID {
		t.Errorf("pricing rule id was mutated: was %s, now %s", origRuleID, parcel.PricingRuleID)
	}
	if parcel.PricingRuleVersion != origVersion {
		t.Errorf("pricing rule version was mutated: was %d, now %d", origVersion, parcel.PricingRuleVersion)
	}
}

func TestParcel_TimestampMonotonicity(t *testing.T) {
	parcel, _ := NewParcel(validTestParcelParams())

	// Transition with timestamp preceding aggregate UpdatedAt must fail
	earlier := parcel.UpdatedAt.Add(-time.Minute)
	_, err := parcel.Assign("courier-001", earlier)
	if !errors.Is(err, ErrInvalidTimestamp) {
		t.Fatalf("expected ErrInvalidTimestamp for past timestamp, got: %v", err)
	}

	// Zero timestamp must fail
	_, err = parcel.Assign("courier-001", time.Time{})
	if !errors.Is(err, ErrInvalidTimestamp) {
		t.Fatalf("expected ErrInvalidTimestamp for zero timestamp, got: %v", err)
	}
}

func TestParcel_TransitionTo_ActorValidationDoesNotMutateState(t *testing.T) {
	parcel, err := NewParcel(validTestParcelParams())
	if err != nil {
		t.Fatalf("failed to create parcel: %v", err)
	}

	t1 := parcel.CreatedAt.Add(10 * time.Minute)
	if _, err := parcel.Cancel(strPtr("SYSTEM"), ActorTypeSystem, "SYSTEM", "bad system id", t1); !errors.Is(err, ErrInvalidActorID) {
		t.Fatalf("expected ErrInvalidActorID when passing literal SYSTEM with ActorTypeSystem, got %v", err)
	}
	if _, err := parcel.Cancel(nil, ActorTypeUser, "CUSTOMER", "missing actor id", t1); !errors.Is(err, ErrInvalidActorID) {
		t.Fatalf("expected ErrInvalidActorID when passing nil actor id for USER, got %v", err)
	}
	if _, err := parcel.TransitionTo(StatusCancelled, strPtr("bot"), ActorType("UNKNOWN"), "BOT", "bad type", t1); !errors.Is(err, ErrInvalidActorType) {
		t.Fatalf("expected ErrInvalidActorType, got %v", err)
	}

	if parcel.Status != StatusCreated || parcel.Version != 1 || !parcel.UpdatedAt.Equal(parcel.CreatedAt) {
		t.Errorf("failed actor validation must not mutate parcel state: status=%s ver=%d updated=%v",
			parcel.Status, parcel.Version, parcel.UpdatedAt)
	}
}
