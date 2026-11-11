package domain_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/swiftdrop/core-api/internal/delivery/domain"
)

func TestNewSuccessfulAttempt(t *testing.T) {
	parcelID := uuid.New()
	courierID := uuid.New()

	pod, _ := domain.NewPhotoProof("https://storage.swiftdrop.io/proofs/123.jpg", "")

	// 1. Valid success attempt
	attempt, err := domain.NewSuccessfulAttempt(parcelID, courierID, 1, pod, nil, "Delivered to reception")
	if err != nil {
		t.Fatalf("unexpected error for successful attempt: %v", err)
	}
	if attempt.Outcome != domain.AttemptOutcomeSuccess {
		t.Errorf("expected outcome SUCCESS, got %s", attempt.Outcome)
	}
	if attempt.FailureReason != nil {
		t.Errorf("expected FailureReason to be nil for success attempt")
	}
	if attempt.IsTerminal() {
		t.Errorf("success attempt should not be flagged as terminal return")
	}

	// 2. Missing proof
	_, err = domain.NewSuccessfulAttempt(parcelID, courierID, 1, nil, nil, "")
	if !errors.Is(err, domain.ErrMissingDeliveryProof) {
		t.Errorf("expected ErrMissingDeliveryProof, got %v", err)
	}

	// 3. Attempt number > 3
	_, err = domain.NewSuccessfulAttempt(parcelID, courierID, 4, pod, nil, "")
	if !errors.Is(err, domain.ErrAttemptLimitExceeded) {
		t.Errorf("expected ErrAttemptLimitExceeded, got %v", err)
	}
}

func TestNewFailedAttempt(t *testing.T) {
	parcelID := uuid.New()
	courierID := uuid.New()

	// 1. Attempt 1 failure with RECIPIENT_UNAVAILABLE -> Not terminal
	attempt1, err := domain.NewFailedAttempt(
		parcelID, courierID, 1,
		domain.FailureReasonRecipientUnavailable,
		nil,
		"No answer at front door",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempt1.IsTerminal() {
		t.Errorf("attempt 1 failure should not be terminal")
	}

	// 2. Attempt 3 failure -> Terminal (ADR-004 3-attempt ceiling)
	attempt3, err := domain.NewFailedAttempt(
		parcelID, courierID, 3,
		domain.FailureReasonRecipientUnavailable,
		nil,
		"Final attempt: phone unanswered",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !attempt3.IsTerminal() {
		t.Errorf("attempt 3 failure must be flagged as terminal (triggering RETURNED)")
	}

	// 3. Attempt 1 failure with REFUSED_BY_RECIPIENT -> Immediate terminal return
	attemptRefused, err := domain.NewFailedAttempt(
		parcelID, courierID, 1,
		domain.FailureReasonRefusedByRecipient,
		nil,
		"Customer states package was not ordered",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !attemptRefused.IsTerminal() {
		t.Errorf("REFUSED_BY_RECIPIENT must be flagged as terminal immediately on attempt 1")
	}

	// 4. Invalid reason OTHER with short note
	_, err = domain.NewFailedAttempt(
		parcelID, courierID, 1,
		domain.FailureReasonOther,
		nil,
		"Gate closed",
	)
	if !errors.Is(err, domain.ErrFailureNoteTooShort) {
		t.Errorf("expected ErrFailureNoteTooShort, got %v", err)
	}
}

func TestDeliveryAttempt_Validate(t *testing.T) {
	parcelID := uuid.New()
	courierID := uuid.New()
	pod, _ := domain.NewPhotoProof("https://storage.swiftdrop.io/proofs/123.jpg", "")
	reason := domain.FailureReasonRecipientUnavailable

	// Valid success attempt
	successAttempt, _ := domain.NewSuccessfulAttempt(parcelID, courierID, 1, pod, nil, "")
	if err := successAttempt.Validate(); err != nil {
		t.Errorf("expected valid success attempt, got %v", err)
	}

	// Conflict: success with failure reason
	successAttempt.FailureReason = &reason
	if err := successAttempt.Validate(); !errors.Is(err, domain.ErrOutcomeConflict) {
		t.Errorf("expected ErrOutcomeConflict, got %v", err)
	}

	// Valid failed attempt
	failedAttempt, _ := domain.NewFailedAttempt(parcelID, courierID, 1, reason, nil, "")
	if err := failedAttempt.Validate(); err != nil {
		t.Errorf("expected valid failed attempt, got %v", err)
	}

	// Conflict: failed attempt with proof
	failedAttempt.Proof = pod
	if err := failedAttempt.Validate(); !errors.Is(err, domain.ErrFailureConflict) {
		t.Errorf("expected ErrFailureConflict, got %v", err)
	}

	// Attempt limit bounds on Validate
	invalidNumAttempt, _ := domain.NewSuccessfulAttempt(parcelID, courierID, 1, pod, nil, "")
	invalidNumAttempt.AttemptNumber = 0
	if err := invalidNumAttempt.Validate(); !errors.Is(err, domain.ErrAttemptLimitExceeded) {
		t.Errorf("expected ErrAttemptLimitExceeded for attempt 0, got %v", err)
	}
	invalidNumAttempt.AttemptNumber = 4
	if err := invalidNumAttempt.Validate(); !errors.Is(err, domain.ErrAttemptLimitExceeded) {
		t.Errorf("expected ErrAttemptLimitExceeded for attempt 4, got %v", err)
	}

	// Missing proof on success
	missingProofAttempt, _ := domain.NewSuccessfulAttempt(parcelID, courierID, 1, pod, nil, "")
	missingProofAttempt.Proof = nil
	if err := missingProofAttempt.Validate(); !errors.Is(err, domain.ErrMissingDeliveryProof) {
		t.Errorf("expected ErrMissingDeliveryProof, got %v", err)
	}

	// Missing failure reason on failed attempt
	missingReasonAttempt, _ := domain.NewFailedAttempt(parcelID, courierID, 1, reason, nil, "")
	missingReasonAttempt.FailureReason = nil
	if err := missingReasonAttempt.Validate(); !errors.Is(err, domain.ErrInvalidFailureReasonCode) {
		t.Errorf("expected ErrInvalidFailureReasonCode, got %v", err)
	}

	// Invalid outcome value
	invalidOutcomeAttempt, _ := domain.NewSuccessfulAttempt(parcelID, courierID, 1, pod, nil, "")
	invalidOutcomeAttempt.Outcome = domain.AttemptOutcome("INVALID_OUTCOME")
	if err := invalidOutcomeAttempt.Validate(); !errors.Is(err, domain.ErrInvalidAttemptOutcome) {
		t.Errorf("expected ErrInvalidAttemptOutcome, got %v", err)
	}

	// Outcome String() method
	if domain.AttemptOutcomeSuccess.String() != "SUCCESS" || domain.AttemptOutcomeFailed.String() != "FAILED" {
		t.Errorf("unexpected string representation for AttemptOutcome")
	}
}
