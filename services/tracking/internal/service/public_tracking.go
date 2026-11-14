package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/swiftdrop/tracking/internal/domain"
	"github.com/swiftdrop/tracking/internal/dto"
	"github.com/swiftdrop/tracking/internal/repository"
)

// PublicTrackingService coordinates validation, persistence retrieval,
// and SEC-009 privacy sanitization for public parcel tracking queries.
type PublicTrackingService struct {
	repo repository.TrackingRepository
}

// NewPublicTrackingService constructs a PublicTrackingService with the provided repository.
func NewPublicTrackingService(repo repository.TrackingRepository) *PublicTrackingService {
	return &PublicTrackingService{repo: repo}
}

// GetPublicTracking validates the tracking number, fetches authoritative parcel and milestone data,
// and projects the internal records into an allowlisted public response DTO.
// Any sensitive fields (PII, street addresses, internal notes, fee info) are strictly redacted per SEC-009.
func (s *PublicTrackingService) GetPublicTracking(ctx context.Context, rawTrackingNumber string) (*dto.PublicTrackingResponse, error) {
	// 1. Validate tracking number using Member 2's canonical format
	tn, err := domain.NewTrackingNumber(rawTrackingNumber)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidTrackingNumber, err)
	}

	if s.repo == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	// 2. Retrieve parcel and status history consistently from repository
	parcel, history, err := s.repo.FindTrackingSummary(ctx, tn.String())
	if err != nil {
		return nil, err
	}

	// 3. Construct chronological milestones with safe status descriptions
	milestones := make([]dto.PublicMilestone, 0, len(history))
	for _, h := range history {
		milestones = append(milestones, dto.PublicMilestone{
			Status:          h.ToStatus,
			Description:     domain.SafeStatusDescription(h.ToStatus),
			Timestamp:       h.Timestamp,
			StatusHistoryID: h.ID,
		})
	}

	// If history is empty, synthesize initial milestone from parcel creation
	if len(milestones) == 0 {
		milestones = append(milestones, dto.PublicMilestone{
			Status:          string(parcel.Status),
			Description:     domain.SafeStatusDescription(string(parcel.Status)),
			Timestamp:       parcel.CreatedAt,
			StatusHistoryID: "",
		})
	}

	// 4. Construct allowlisted PublicTrackingResponse DTO
	response := &dto.PublicTrackingResponse{
		TrackingNumber:    parcel.TrackingNumber.String(),
		Status:            string(parcel.Status),
		StatusDescription: domain.SafeStatusDescription(string(parcel.Status)),
		Destination: dto.PublicDestination{
			City: strings.TrimSpace(parcel.DestinationCity),
		},
		EstimatedDelivery: nil, // Omitted per SEC-009 and instruction: do not invent unavailable timestamps
		Milestones:        milestones,
	}

	return response, nil
}
