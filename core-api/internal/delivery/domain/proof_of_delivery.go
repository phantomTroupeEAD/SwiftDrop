package domain

import "strings"

type ProofType string

const (
	ProofTypePhoto                 ProofType = "PHOTO"
	ProofTypeSignature             ProofType = "SIGNATURE"
	ProofTypeRecipientConfirmation ProofType = "RECIPIENT_CONFIRMATION"
)

// ProofOfDelivery is a value object capturing doorstep evidence of parcel handover.
type ProofOfDelivery struct {
	Type           ProofType
	AttachmentURL  string
	SignatureData  string
	ReceivedByName string
}

// NewPhotoProof creates a photo proof of delivery value object.
func NewPhotoProof(attachmentURL, receivedByName string) (*ProofOfDelivery, error) {
	pod := &ProofOfDelivery{
		Type:           ProofTypePhoto,
		AttachmentURL:  strings.TrimSpace(attachmentURL),
		ReceivedByName: strings.TrimSpace(receivedByName),
	}
	if err := pod.Validate(); err != nil {
		return nil, err
	}
	return pod, nil
}

// NewSignatureProof creates a vector signature proof of delivery value object.
func NewSignatureProof(signatureData, receivedByName string) (*ProofOfDelivery, error) {
	pod := &ProofOfDelivery{
		Type:           ProofTypeSignature,
		SignatureData:  strings.TrimSpace(signatureData),
		ReceivedByName: strings.TrimSpace(receivedByName),
	}
	if err := pod.Validate(); err != nil {
		return nil, err
	}
	return pod, nil
}

// NewConfirmationProof creates an attended recipient confirmation proof value object.
func NewConfirmationProof(receivedByName string) (*ProofOfDelivery, error) {
	pod := &ProofOfDelivery{
		Type:           ProofTypeRecipientConfirmation,
		ReceivedByName: strings.TrimSpace(receivedByName),
	}
	if err := pod.Validate(); err != nil {
		return nil, err
	}
	return pod, nil
}

// Validate verifies mandatory proof constraints based on proof type.
func (p *ProofOfDelivery) Validate() error {
	switch p.Type {
	case ProofTypePhoto:
		if p.AttachmentURL == "" {
			return ErrMissingProofPhotoURL
		}
		return nil

	case ProofTypeSignature:
		if len(p.SignatureData) < 50 {
			return ErrInvalidSignatureData
		}
		if len(p.ReceivedByName) < 2 {
			return ErrRecipientNameRequired
		}
		return nil

	case ProofTypeRecipientConfirmation:
		if len(p.ReceivedByName) < 2 {
			return ErrRecipientNameRequired
		}
		return nil

	default:
		return ErrMissingDeliveryProof
	}
}

func (p ProofType) String() string {
	return string(p)
}

