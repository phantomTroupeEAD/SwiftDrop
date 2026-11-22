package application

import (
	"context"
	"fmt"

	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

// ListPricingRulesQuery holds pagination and filtering parameters
// for retrieving pricing rules.
type ListPricingRulesQuery struct {
	OriginZoneID      string
	DestinationZoneID string
	ActiveOnly        *bool
	Page              int
	PerPage           int
}

// ListPricingRulesResult represents the paginated output of pricing rules.
type ListPricingRulesResult struct {
	Rules      []domain.Tariff
	TotalItems int
	Page       int
	PerPage    int
}

// ListPricingRulesUseCase coordinates retrieval of pricing rules for administrators.
type ListPricingRulesUseCase struct {
	tariffRepo TariffRepository
}

// NewListPricingRulesUseCase constructs a new ListPricingRulesUseCase.
func NewListPricingRulesUseCase(tariffRepo TariffRepository) *ListPricingRulesUseCase {
	return &ListPricingRulesUseCase{tariffRepo: tariffRepo}
}

// Execute retrieves a paginated slice of pricing rules matching the given query.
func (uc *ListPricingRulesUseCase) Execute(
	ctx context.Context,
	query ListPricingRulesQuery,
) (ListPricingRulesResult, error) {
	if uc.tariffRepo == nil {
		return ListPricingRulesResult{}, fmt.Errorf("pricing application: nil tariff repository")
	}

	page := query.Page
	if page < 1 {
		page = 1
	}

	perPage := query.PerPage
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	if query.OriginZoneID != "" {
		if err := domain.ValidateZoneID(query.OriginZoneID); err != nil {
			return ListPricingRulesResult{}, fmt.Errorf("%w: origin zone %w", domain.ErrInvalidTariff, err)
		}
	}

	if query.DestinationZoneID != "" {
		if err := domain.ValidateZoneID(query.DestinationZoneID); err != nil {
			return ListPricingRulesResult{}, fmt.Errorf("%w: destination zone %w", domain.ErrInvalidTariff, err)
		}
	}

	offset := (page - 1) * perPage

	filter := ListTariffsFilter{
		OriginZoneID:      query.OriginZoneID,
		DestinationZoneID: query.DestinationZoneID,
		ActiveOnly:        query.ActiveOnly,
	}

	rules, total, err := uc.tariffRepo.List(ctx, filter, offset, perPage)
	if err != nil {
		return ListPricingRulesResult{}, fmt.Errorf("list pricing rules: %w", err)
	}

	return ListPricingRulesResult{
		Rules:      rules,
		TotalItems: total,
		Page:       page,
		PerPage:    perPage,
	}, nil
}
