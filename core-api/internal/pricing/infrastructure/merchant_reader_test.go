package infrastructure

import (
	"context"
	"errors"
	"testing"

	merchantsapp "github.com/swiftdrop/core-api/internal/merchants/application"
	merchantsdomain "github.com/swiftdrop/core-api/internal/merchants/domain"
	"github.com/swiftdrop/core-api/internal/pricing/application"
)

type stubMerchantRepo struct {
	view merchantsapp.MerchantView
	err  error
}

func (s *stubMerchantRepo) Insert(_ context.Context, _ merchantsapp.Querier, _ merchantsapp.MerchantView) error {
	return nil
}

func (s *stubMerchantRepo) Get(_ context.Context, _ merchantsapp.Querier, _ string) (merchantsapp.MerchantView, error) {
	return s.view, s.err
}

func (s *stubMerchantRepo) FindByOwner(_ context.Context, _ merchantsapp.Querier, _ string) (merchantsapp.MerchantView, error) {
	return s.view, s.err
}

func (s *stubMerchantRepo) Status(_ context.Context, _ merchantsapp.Querier, _ string) (merchantsdomain.MerchantStatus, error) {
	return s.view.Status, s.err
}

func (s *stubMerchantRepo) LockForUpdate(_ context.Context, _ merchantsapp.Querier, _ string) (merchantsapp.MerchantView, error) {
	return s.view, s.err
}

func (s *stubMerchantRepo) SaveStatus(_ context.Context, _ merchantsapp.Querier, _ *merchantsdomain.Merchant, _ int, _ string) error {
	return nil
}

func (s *stubMerchantRepo) List(_ context.Context, _ merchantsapp.Querier, _ string, _, _ int) ([]merchantsapp.MerchantView, int, error) {
	return nil, 0, nil
}

func TestMerchantRepoAdapter_FindByID(t *testing.T) {
	t.Run("maps merchant view fields accurately", func(t *testing.T) {
		repo := &stubMerchantRepo{
			view: merchantsapp.MerchantView{
				ID:          "0192e2b3-7c8a-7890-a123-000000000010",
				OwnerUserID: "018f4a7c-1004-7000-8000-000000000004",
				Status:      merchantsdomain.StatusApproved,
			},
		}
		adapter := NewMerchantRepoAdapter(repo, nil)

		got, err := adapter.FindByID(context.Background(), "0192e2b3-7c8a-7890-a123-000000000010")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != "0192e2b3-7c8a-7890-a123-000000000010" ||
			got.OwnerUserID != "018f4a7c-1004-7000-8000-000000000004" ||
			got.Status != application.MerchantStatusApproved {
			t.Fatalf("unexpected profile: %+v", got)
		}
	})

	t.Run("translates merchant not found error", func(t *testing.T) {
		repo := &stubMerchantRepo{err: merchantsapp.ErrMerchantNotFound}
		adapter := NewMerchantRepoAdapter(repo, nil)

		_, err := adapter.FindByID(context.Background(), "0192e2b3-7c8a-7890-a123-000000000099")
		if !errors.Is(err, application.ErrMerchantNotFound) {
			t.Fatalf("expected ErrMerchantNotFound, got %v", err)
		}
	})

	t.Run("rejects blank merchant ID without hitting repo", func(t *testing.T) {
		repo := &stubMerchantRepo{err: errors.New("should not be called")}
		adapter := NewMerchantRepoAdapter(repo, nil)

		_, err := adapter.FindByID(context.Background(), "   ")
		if !errors.Is(err, application.ErrMerchantNotFound) {
			t.Fatalf("expected ErrMerchantNotFound, got %v", err)
		}
	})

	t.Run("propagates unexpected repository failures", func(t *testing.T) {
		repo := &stubMerchantRepo{err: errors.New("connection lost")}
		adapter := NewMerchantRepoAdapter(repo, nil)

		_, err := adapter.FindByID(context.Background(), "0192e2b3-7c8a-7890-a123-000000000010")
		if err == nil || errors.Is(err, application.ErrMerchantNotFound) {
			t.Fatalf("expected wrapped infrastructure error, got %v", err)
		}
	})
}
