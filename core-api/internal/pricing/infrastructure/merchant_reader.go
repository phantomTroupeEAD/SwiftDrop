package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"strings"

	merchantsapp "github.com/swiftdrop/core-api/internal/merchants/application"
	"github.com/swiftdrop/core-api/internal/pricing/application"
)

var _ application.MerchantReader = (*MerchantRepoAdapter)(nil)

// MerchantRepoAdapter adapts Member 4's MerchantRepo to pricing's read-only MerchantReader port.
type MerchantRepoAdapter struct {
	repo    merchantsapp.MerchantRepo
	querier merchantsapp.Querier
}

// NewMerchantRepoAdapter constructs a MerchantRepoAdapter backed by a merchants repository and DB querier.
func NewMerchantRepoAdapter(repo merchantsapp.MerchantRepo, querier merchantsapp.Querier) *MerchantRepoAdapter {
	return &MerchantRepoAdapter{
		repo:    repo,
		querier: querier,
	}
}

// FindByID retrieves an authoritative merchant tenant record and projects it to application.MerchantProfile.
func (a *MerchantRepoAdapter) FindByID(ctx context.Context, merchantID string) (application.MerchantProfile, error) {
	if a == nil || a.repo == nil {
		return application.MerchantProfile{}, errors.New("pricing: nil merchant repository")
	}

	cleanID := strings.TrimSpace(merchantID)
	if cleanID == "" {
		return application.MerchantProfile{}, application.ErrMerchantNotFound
	}

	view, err := a.repo.Get(ctx, a.querier, cleanID)
	if err != nil {
		if errors.Is(err, merchantsapp.ErrMerchantNotFound) || errors.Is(err, merchantsapp.ErrNotFound) {
			return application.MerchantProfile{}, application.ErrMerchantNotFound
		}
		return application.MerchantProfile{}, fmt.Errorf("pricing: lookup merchant %s: %w", cleanID, err)
	}

	return application.MerchantProfile{
		ID:          strings.TrimSpace(view.ID),
		OwnerUserID: strings.TrimSpace(view.OwnerUserID),
		Status:      strings.TrimSpace(string(view.Status)),
	}, nil
}
