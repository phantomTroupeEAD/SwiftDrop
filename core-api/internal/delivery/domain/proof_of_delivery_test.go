package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/swiftdrop/core-api/internal/delivery/domain"
)

func TestProofOfDelivery_Photo(t *testing.T) {
	// 1. Success with valid URL
	pod, err := domain.NewPhotoProof("https://storage.swiftdrop.io/proofs/doorstep_01.jpg", "Jane Doe")
	if err != nil {
		t.Fatalf("unexpected error for photo proof: %v", err)
	}
	if pod.Type != domain.ProofTypePhoto {
		t.Errorf("expected PHOTO proof type, got %s", pod.Type)
	}

	// 2. Empty attachment URL
	_, err = domain.NewPhotoProof("   ", "Jane Doe")
	if !errors.Is(err, domain.ErrMissingProofPhotoURL) {
		t.Errorf("expected ErrMissingProofPhotoURL, got %v", err)
	}
}

func TestProofOfDelivery_Signature(t *testing.T) {
	validSignature := "M10,20 L30,40 L50,60 L70,80 L90,100 L110,120 L130,140 L150,160 L170,180" // > 50 chars

	// 1. Success with valid stroke and recipient name
	pod, err := domain.NewSignatureProof(validSignature, "Jane Smith")
	if err != nil {
		t.Fatalf("unexpected error for signature proof: %v", err)
	}
	if pod.Type != domain.ProofTypeSignature {
		t.Errorf("expected SIGNATURE proof type, got %s", pod.Type)
	}

	// 2. Short signature stroke data (< 50 chars)
	_, err = domain.NewSignatureProof("M10,20 L30,40", "Jane Smith")
	if !errors.Is(err, domain.ErrInvalidSignatureData) {
		t.Errorf("expected ErrInvalidSignatureData, got %v", err)
	}

	// 3. Missing recipient name
	_, err = domain.NewSignatureProof(validSignature, " ")
	if !errors.Is(err, domain.ErrRecipientNameRequired) {
		t.Errorf("expected ErrRecipientNameRequired, got %v", err)
	}

	// 4. Single-character recipient name
	_, err = domain.NewSignatureProof(validSignature, "J")
	if !errors.Is(err, domain.ErrRecipientNameRequired) {
		t.Errorf("expected ErrRecipientNameRequired for 1-char name, got %v", err)
	}
}

func TestProofOfDelivery_RecipientConfirmation(t *testing.T) {
	// 1. Success
	pod, err := domain.NewConfirmationProof("Mark Perera (Receptionist)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pod.Type != domain.ProofTypeRecipientConfirmation {
		t.Errorf("expected RECIPIENT_CONFIRMATION, got %s", pod.Type)
	}

	// 2. Missing name
	_, err = domain.NewConfirmationProof(strings.Repeat(" ", 5))
	if !errors.Is(err, domain.ErrRecipientNameRequired) {
		t.Errorf("expected ErrRecipientNameRequired, got %v", err)
	}
}
