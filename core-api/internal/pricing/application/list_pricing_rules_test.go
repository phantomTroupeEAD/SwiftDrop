package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

type mockTariffListRepo struct {
	tariffs    []domain.Tariff
	total      int
	err        error
	called     bool
	lastFilter ListTariffsFilter
	lastOffset int
	lastLimit  int
}

func (m *mockTariffListRepo) FindApplicable(_ context.Context, _, _ string, _ time.Time) (domain.Tariff, error) {
	return domain.Tariff{}, nil
}

func (m *mockTariffListRepo) List(_ context.Context, filter ListTariffsFilter, offset, limit int) ([]domain.Tariff, int, error) {
	m.called = true
	m.lastFilter = filter
	m.lastOffset = offset
	m.lastLimit = limit
	return m.tariffs, m.total, m.err
}

func TestListPricingRulesUseCase_Execute_Success(t *testing.T) {
	now := time.Now().UTC()
	active := true
	rules := []domain.Tariff{
		{
			ID:                "rule_01",
			OriginZoneID:      "ZONE_WESTERN_01",
			DestinationZoneID: "ZONE_CENTRAL_01",
			BaseFee:           domain.Money{Amount: 35000, Currency: domain.CurrencyLKR},
			PerKgRate:         domain.Money{Amount: 8000, Currency: domain.CurrencyLKR},
			EffectiveFrom:     now.Add(-24 * time.Hour),
			IsActive:          true,
			Version:           1,
			CreatedAt:         now.Add(-24 * time.Hour),
			UpdatedAt:         now.Add(-24 * time.Hour),
		},
	}

	repo := &mockTariffListRepo{
		tariffs: rules,
		total:   1,
	}

	uc := NewListPricingRulesUseCase(repo)

	res, err := uc.Execute(context.Background(), ListPricingRulesQuery{
		Page:       1,
		PerPage:    10,
		ActiveOnly: &active,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.called {
		t.Fatal("expected repository to be called")
	}
	if repo.lastOffset != 0 || repo.lastLimit != 10 {
		t.Errorf("expected offset 0 limit 10, got offset %d limit %d", repo.lastOffset, repo.lastLimit)
	}
	if repo.lastFilter.ActiveOnly == nil || !*repo.lastFilter.ActiveOnly {
		t.Error("expected ActiveOnly filter to be true")
	}
	if res.TotalItems != 1 {
		t.Errorf("expected total items 1, got %d", res.TotalItems)
	}
	if len(res.Rules) != 1 || res.Rules[0].ID != "rule_01" {
		t.Errorf("unexpected rules returned: %+v", res.Rules)
	}
}

func TestListPricingRulesUseCase_Execute_PaginationBounds(t *testing.T) {
	repo := &mockTariffListRepo{total: 0}
	uc := NewListPricingRulesUseCase(repo)

	// Page < 1 defaults to 1, PerPage < 1 defaults to 20
	_, err := uc.Execute(context.Background(), ListPricingRulesQuery{
		Page:    0,
		PerPage: -5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastOffset != 0 || repo.lastLimit != 20 {
		t.Errorf("expected offset 0 limit 20, got offset %d limit %d", repo.lastOffset, repo.lastLimit)
	}

	// PerPage > 100 capped at 100, Page 3 -> offset 200
	_, err = uc.Execute(context.Background(), ListPricingRulesQuery{
		Page:    3,
		PerPage: 150,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastOffset != 200 || repo.lastLimit != 100 {
		t.Errorf("expected offset 200 limit 100, got offset %d limit %d", repo.lastOffset, repo.lastLimit)
	}
}

func TestListPricingRulesUseCase_Execute_InvalidZoneFilter(t *testing.T) {
	repo := &mockTariffListRepo{}
	uc := NewListPricingRulesUseCase(repo)

	_, err := uc.Execute(context.Background(), ListPricingRulesQuery{
		OriginZoneID: "invalid-zone",
	})
	if err == nil || !errors.Is(err, domain.ErrInvalidTariff) {
		t.Fatalf("expected ErrInvalidTariff for bad origin zone, got %v", err)
	}

	_, err = uc.Execute(context.Background(), ListPricingRulesQuery{
		DestinationZoneID: "bad_destination",
	})
	if err == nil || !errors.Is(err, domain.ErrInvalidTariff) {
		t.Fatalf("expected ErrInvalidTariff for bad destination zone, got %v", err)
	}
}

func TestListPricingRulesUseCase_Execute_RepoError(t *testing.T) {
	dbErr := errors.New("db query error")
	repo := &mockTariffListRepo{err: dbErr}
	uc := NewListPricingRulesUseCase(repo)

	_, err := uc.Execute(context.Background(), ListPricingRulesQuery{})
	if err == nil || !errors.Is(err, dbErr) {
		t.Fatalf("expected dbErr, got %v", err)
	}
}
