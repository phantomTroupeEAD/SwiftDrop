package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/swiftdrop/tracking/internal/auth"
	"github.com/swiftdrop/tracking/internal/domain"
	"github.com/swiftdrop/tracking/internal/dto"
	"github.com/swiftdrop/tracking/internal/repository"
)

// AuthenticatedTrackingService coordinates validation, persistence retrieval,
// and ownership authorization for authenticated private parcel tracking queries.
type AuthenticatedTrackingService struct {
	repo repository.TrackingRepository
}

// NewAuthenticatedTrackingService constructs an AuthenticatedTrackingService.
func NewAuthenticatedTrackingService(repo repository.TrackingRepository) *AuthenticatedTrackingService {
	return &AuthenticatedTrackingService{repo: repo}
}

// GetAuthenticatedTracking retrieves full parcel milestone history for authorized callers.
// Enforces server-side ownership authorization:
// - Role CUSTOMER: authorized only if parcel.sender_id matches principal.UserID.
// - Role MERCHANT: authorized only if parcel.merchant_id matches principal.TenantID.
// - Role ADMIN / DISPATCHER: authorized for operational tracking oversight.
// - Role COURIER or any other role: rejected with ErrAccessDenied.
func (s *AuthenticatedTrackingService) GetAuthenticatedTracking(
	ctx context.Context,
	rawTrackingNumber string,
	principal *auth.Principal,
) (*dto.AuthenticatedTrackingResponse, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if principal == nil {
		return nil, domain.ErrUnauthenticated
	}

	// 1. Validate tracking number syntax
	tn, err := domain.NewTrackingNumber(rawTrackingNumber)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidTrackingNumber, err)
	}

	if s.repo == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	// 2. Fetch authoritative parcel state and history from read-only repository
	parcel, history, err := s.repo.FindTrackingSummary(ctx, tn.String())
	if err != nil {
		return nil, err
	}

	// 3. Enforce server-side ownership authorization
	if err := s.authorizePrincipal(principal, parcel); err != nil {
		return nil, err
	}

	// 4. Construct chronological milestones
	milestones := make([]dto.AuthenticatedMilestone, 0, len(history))
	for _, h := range history {
		milestones = append(milestones, dto.AuthenticatedMilestone{
			Status:      h.ToStatus,
			Description: domain.SafeStatusDescription(h.ToStatus),
			ActorRole:   h.ActorRole,
			ReasonCode:  SanitizeReasonCode(h.Reason),
			Timestamp:   h.Timestamp,
		})
	}

	// If history is empty, synthesize initial milestone from parcel creation
	if len(milestones) == 0 {
		milestones = append(milestones, dto.AuthenticatedMilestone{
			Status:      string(parcel.Status),
			Description: domain.SafeStatusDescription(string(parcel.Status)),
			Timestamp:   parcel.CreatedAt,
		})
	}

	// 5. Construct authenticated tracking response DTO
	response := &dto.AuthenticatedTrackingResponse{
		TrackingNumber:    parcel.TrackingNumber.String(),
		ParcelID:          parcel.ID,
		Status:            string(parcel.Status),
		StatusDescription: domain.SafeStatusDescription(string(parcel.Status)),
		Destination: dto.AuthenticatedDestination{
			City:       strings.TrimSpace(parcel.DestinationCity),
			PostalCode: strings.TrimSpace(parcel.DestinationPostalCode),
			ZoneID:     strings.TrimSpace(parcel.DestinationZoneID),
		},
		Milestones: milestones,
	}

	return response, nil
}

// authorizePrincipal checks whether the principal has permission to view the parcel.
func (s *AuthenticatedTrackingService) authorizePrincipal(p *auth.Principal, parcel *domain.ParcelTrackingInfo) error {
	switch p.Role {
	case auth.RoleAdmin, auth.RoleDispatcher:
		// Operational oversight: permitted
		return nil

	case auth.RoleCustomer:
		if parcel.SenderID == nil || strings.TrimSpace(*parcel.SenderID) == "" {
			return domain.ErrAccessDenied
		}
		if strings.TrimSpace(*parcel.SenderID) != strings.TrimSpace(p.UserID) {
			return domain.ErrAccessDenied
		}
		return nil

	case auth.RoleMerchant:
		if p.TenantID == nil || strings.TrimSpace(*p.TenantID) == "" {
			return domain.ErrAccessDenied
		}
		if parcel.MerchantID == nil || strings.TrimSpace(*parcel.MerchantID) == "" {
			return domain.ErrAccessDenied
		}
		if strings.TrimSpace(*parcel.MerchantID) != strings.TrimSpace(*p.TenantID) {
			return domain.ErrAccessDenied
		}
		return nil

	default:
		// COURIER or any other role: not permitted on customer/merchant tracking endpoint
		return domain.ErrAccessDenied
	}
}

// SanitizeReasonCode maps raw database reason text to an approved, standardized reason code.
// Per SEC-009 and ADR-004 §4, raw courier remarks or private operational notes must never be
// leaked to API clients. Only standardized enum codes are exposed.
func SanitizeReasonCode(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	upper := strings.ToUpper(trimmed)
	switch upper {
	case "RECIPIENT_UNAVAILABLE",
		"CUSTOMER_REQUESTED_RESCHEDULE",
		"ACCESS_RESTRICTED",
		"INVALID_ADDRESS",
		"REFUSED_BY_RECIPIENT",
		"DAMAGED_IN_TRANSIT",
		"OTHER":
		return upper
	default:
		// Do not leak freeform notes or unvetted text.
		return "OTHER"
	}
}
