package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/swiftdrop/core-api/internal/events"
	"github.com/swiftdrop/core-api/internal/parcels/domain"
)

// Supported caller role identifiers matching SwiftDrop RBAC roles without coupling to identity/transport.
const (
	RoleCustomer   = "CUSTOMER"
	RoleMerchant   = "MERCHANT"
	RoleAdmin      = "ADMIN"
	RoleCourier    = "COURIER"
	RoleDispatcher = "DISPATCHER"
)

// CallerContext carries the authenticated principal identity resolved at the request boundary.
type CallerContext struct {
	UserID   string
	Role     string
	TenantID *string
}

// ResolvedBookingCaller contains verified parcel ownership derived from an authorized CallerContext.
type ResolvedBookingCaller struct {
	SenderID   *string
	MerchantID *string
	ActorType  domain.ActorType
}

// GetParcelQuery encapsulates parameters for retrieving a single parcel aggregate.
type GetParcelQuery struct {
	ParcelID string
	Caller   CallerContext
}

// GetParcelHistoryQuery encapsulates parameters for retrieving chronological parcel status history.
type GetParcelHistoryQuery struct {
	ParcelID string
	Caller   CallerContext
}

// GetParcelHistoryResult combines the authorized parcel aggregate and its immutable chronological audit trail.
type GetParcelHistoryResult struct {
	Parcel  domain.Parcel
	Entries []domain.StatusHistoryEntry
}

// GetParcelUseCase retrieves parcel details after enforcing strict role, merchant-approval, and IDOR ownership checks.
type GetParcelUseCase struct {
	parcelRepo     ParcelRepository
	merchantReader MerchantReader
}

// NewGetParcelUseCase constructs a GetParcelUseCase.
func NewGetParcelUseCase(parcelRepo ParcelRepository, merchantReader MerchantReader) (*GetParcelUseCase, error) {
	if parcelRepo == nil {
		return nil, ErrNilParcelRepository
	}
	if merchantReader == nil {
		return nil, ErrNilMerchantReader
	}

	return &GetParcelUseCase{
		parcelRepo:     parcelRepo,
		merchantReader: merchantReader,
	}, nil
}

// Execute validates caller authorization, loads the requested parcel, and enforces object-level ownership.
func (uc *GetParcelUseCase) Execute(ctx context.Context, query GetParcelQuery) (domain.Parcel, error) {
	return loadAuthorizedParcel(ctx, uc.parcelRepo, uc.merchantReader, query.Caller, query.ParcelID)
}

// GetParcelHistoryUseCase retrieves chronological status history entries after enforcing parcel ownership checks.
type GetParcelHistoryUseCase struct {
	parcelRepo        ParcelRepository
	statusHistoryRepo StatusHistoryRepository
	merchantReader    MerchantReader
}

// NewGetParcelHistoryUseCase constructs a GetParcelHistoryUseCase.
func NewGetParcelHistoryUseCase(
	parcelRepo ParcelRepository,
	statusHistoryRepo StatusHistoryRepository,
	merchantReader MerchantReader,
) (*GetParcelHistoryUseCase, error) {
	if parcelRepo == nil {
		return nil, ErrNilParcelRepository
	}
	if statusHistoryRepo == nil {
		return nil, ErrNilStatusHistoryRepository
	}
	if merchantReader == nil {
		return nil, ErrNilMerchantReader
	}

	return &GetParcelHistoryUseCase{
		parcelRepo:        parcelRepo,
		statusHistoryRepo: statusHistoryRepo,
		merchantReader:    merchantReader,
	}, nil
}

// Execute validates caller authorization and parcel ownership before loading chronological status history.
func (uc *GetParcelHistoryUseCase) Execute(ctx context.Context, query GetParcelHistoryQuery) (GetParcelHistoryResult, error) {
	parcel, err := loadAuthorizedParcel(ctx, uc.parcelRepo, uc.merchantReader, query.Caller, query.ParcelID)
	if err != nil {
		return GetParcelHistoryResult{}, err
	}

	entries, err := uc.statusHistoryRepo.FindByParcelID(ctx, nil, parcel.ID)
	if err != nil {
		return GetParcelHistoryResult{}, fmt.Errorf("load status history for parcel %s: %w", parcel.ID, err)
	}
	if entries == nil {
		entries = make([]domain.StatusHistoryEntry, 0)
	}

	return GetParcelHistoryResult{
		Parcel:  parcel,
		Entries: entries,
	}, nil
}

// ResolveParcelCallerForBooking validates booking role/merchant eligibility and resolves authoritative owner IDs.
func ResolveParcelCallerForBooking(ctx context.Context, caller CallerContext, merchantReader MerchantReader) (ResolvedBookingCaller, error) {
	verified, err := authorizeAndResolveCaller(ctx, caller, merchantReader)
	if err != nil {
		return ResolvedBookingCaller{}, err
	}

	senderID := verified.userID
	return ResolvedBookingCaller{
		SenderID:   &senderID,
		MerchantID: verified.merchantID,
		ActorType:  domain.ActorTypeUser,
	}, nil
}

type verifiedCaller struct {
	userID     string
	role       string
	merchantID *string
}

func loadAuthorizedParcel(
	ctx context.Context,
	parcelRepo ParcelRepository,
	merchantReader MerchantReader,
	caller CallerContext,
	rawParcelID string,
) (domain.Parcel, error) {
	// Authorize caller and merchant status before checking parcel existence to prevent cross-role/suspended enumeration.
	verified, err := authorizeAndResolveCaller(ctx, caller, merchantReader)
	if err != nil {
		return domain.Parcel{}, err
	}

	parcelID := strings.TrimSpace(rawParcelID)
	if parcelID == "" || !events.IsValidUUID(parcelID) {
		return domain.Parcel{}, domain.ErrInvalidParcelID
	}

	parcel, err := parcelRepo.FindByID(ctx, nil, parcelID)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidParcelID) || errors.Is(err, sql.ErrNoRows) {
			return domain.Parcel{}, fmt.Errorf("%w: %s", ErrParcelNotFound, parcelID)
		}
		return domain.Parcel{}, fmt.Errorf("load parcel %s: %w", parcelID, err)
	}

	if err := authorizeParcelAccess(parcel, verified); err != nil {
		return domain.Parcel{}, err
	}

	return parcel, nil
}

func authorizeAndResolveCaller(ctx context.Context, caller CallerContext, merchantReader MerchantReader) (verifiedCaller, error) {
	userID := strings.TrimSpace(caller.UserID)
	if !events.IsValidUUID(userID) {
		return verifiedCaller{}, fmt.Errorf("%w: %w", ErrForbiddenParcelAccess, ErrInvalidCallerID)
	}

	role := strings.ToUpper(strings.TrimSpace(caller.Role))
	switch role {
	case RoleCustomer, RoleAdmin:
		return verifiedCaller{
			userID: userID,
			role:   role,
		}, nil

	case RoleMerchant:
		merchantID, err := resolveVerifiedMerchant(ctx, userID, caller.TenantID, merchantReader)
		if err != nil {
			return verifiedCaller{}, err
		}
		return verifiedCaller{
			userID:     userID,
			role:       role,
			merchantID: &merchantID,
		}, nil

	default:
		return verifiedCaller{}, fmt.Errorf("%w: role %q is not permitted to access parcels", ErrForbiddenParcelAccess, caller.Role)
	}
}

func resolveVerifiedMerchant(
	ctx context.Context,
	userID string,
	rawTenantID *string,
	merchantReader MerchantReader,
) (string, error) {
	if rawTenantID == nil || strings.TrimSpace(*rawTenantID) == "" {
		return "", fmt.Errorf("%w: %w", ErrForbiddenParcelAccess, ErrMissingTenantID)
	}

	tenantID := strings.TrimSpace(*rawTenantID)
	if !events.IsValidUUID(tenantID) {
		return "", fmt.Errorf("%w: %w", ErrForbiddenParcelAccess, ErrInvalidTenantID)
	}

	if merchantReader == nil {
		return "", ErrNilMerchantReader
	}

	profile, err := merchantReader.FindByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, ErrMerchantNotFound) {
			return "", fmt.Errorf("%w: %w", ErrForbiddenParcelAccess, ErrMerchantNotFound)
		}
		return "", fmt.Errorf("lookup merchant %s: %w", tenantID, err)
	}

	if strings.TrimSpace(profile.OwnerUserID) != userID {
		return "", fmt.Errorf("%w: %w", ErrForbiddenParcelAccess, ErrMerchantOwnerMismatch)
	}

	if strings.TrimSpace(profile.Status) != MerchantStatusApproved {
		return "", fmt.Errorf("%w: %w", ErrForbiddenParcelAccess, ErrMerchantNotApproved)
	}

	merchantID := strings.TrimSpace(profile.ID)
	if !events.IsValidUUID(merchantID) {
		return "", fmt.Errorf("parcel: resolved merchant id %q is not a valid UUID", merchantID)
	}

	return merchantID, nil
}

func authorizeParcelAccess(parcel domain.Parcel, caller verifiedCaller) error {
	switch caller.role {
	case RoleAdmin:
		return nil

	case RoleCustomer:
		if parcel.SenderID == nil || strings.TrimSpace(*parcel.SenderID) != caller.userID {
			return fmt.Errorf("%w: customer %s does not own parcel %s", ErrForbiddenParcelAccess, caller.userID, parcel.ID)
		}
		return nil

	case RoleMerchant:
		if caller.merchantID == nil || parcel.MerchantID == nil || strings.TrimSpace(*parcel.MerchantID) != *caller.merchantID {
			return fmt.Errorf("%w: merchant tenant does not own parcel %s", ErrForbiddenParcelAccess, parcel.ID)
		}
		return nil

	default:
		return ErrForbiddenParcelAccess
	}
}
