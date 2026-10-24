package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"swiftdrop.local/m4/core-api/internal/merchants/domain"
)

// Actor is the authenticated JWT caller.
type Actor struct {
	UserID, Role, TenantID, RequestID string
}

// MerchantPrincipal is the verified merchant identity for order endpoints.
type MerchantPrincipal struct {
	MerchantID, CredentialID, KeyPrefix string
	Scopes                              domain.ScopeSet
}

type Config struct {
	KeyEnvPrefix        string
	MaxActiveKeys       int
	MaxWebhooks         int
	AllowHTTP           bool
	AllowedPrivateHosts []string
}

type Deps struct {
	Tx        TxRunner
	Merchants MerchantRepo
	Creds     CredentialRepo
	Hooks     WebhookRepo
	Audit     AuditWriter
	Roles     RoleAssigner
	KeyGen    KeyGenerator
	Hasher    KeyHasher
	Cipher    SecretCipher
	Resolver  HostResolver
	Booking   ParcelBooking
	Observer  Observer
	Now       func() time.Time
}

// Service groups the merchants use cases. (Doc 3 proposes one struct per use case;
// this local slice keeps them on one type to stay small.)
type Service struct {
	Deps
	cfg Config

	touchMu   sync.Mutex
	lastTouch map[string]time.Time
}

func NewService(d Deps, c Config) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{Deps: d, cfg: c, lastTouch: map[string]time.Time{}}
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// EventCatalog lists subscribable webhook events (event-contracts.md; U-06).
var EventCatalog = []struct{ Type, Description string }{
	{"parcel.booked.v1", "Parcel created"},
	{"parcel.assigned.v1", "Courier assigned"},
	{"parcel.picked_up.v1", "Parcel collected from sender"},
	{"parcel.out_for_delivery.v1", "Out for final-mile delivery"},
	{"parcel.delivered.v1", "Delivered with proof"},
	{"parcel.delivery_failed.v1", "Delivery attempt failed"},
}

func inCatalog(t string) bool {
	for _, e := range EventCatalog {
		if e.Type == t {
			return true
		}
	}
	return false
}

// ---------------- Applications and merchant lookup ----------------

type ApplicationInput struct {
	BusinessName, ContactEmail, ContactPhone string
	Address                                  domain.Address
}

func (s *Service) SubmitApplication(ctx context.Context, a Actor, in ApplicationInput) (MerchantView, error) {
	m, err := domain.NewMerchant(newUUID(), a.UserID, in.BusinessName)
	if err != nil || domain.HasControlChars(in.BusinessName) {
		return MerchantView{}, ValidationError{"business_name", "must be 2-150 characters, no control characters"}
	}
	if err := domain.ValidateContact(in.ContactEmail, in.ContactPhone, in.Address); err != nil {
		var pe domain.ErrInvalidProfile
		if errors.As(err, &pe) {
			return MerchantView{}, ValidationError{pe.Field, pe.Issue}
		}
		return MerchantView{}, err
	}
	now := s.Now().UTC()
	v := MerchantView{
		ID: m.ID(), OwnerUserID: a.UserID, BusinessName: strings.TrimSpace(in.BusinessName),
		Status: domain.StatusPending, ContactEmail: in.ContactEmail, ContactPhone: in.ContactPhone,
		Address: in.Address, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Merchants.Insert(ctx, s.Tx.Reader(), v); err != nil {
		return MerchantView{}, err
	}
	return v, nil
}

// MyMerchant uses the JWT tenant_id when present, else the owner lookup for applicants (DD-12).
func (s *Service) MyMerchant(ctx context.Context, a Actor) (MerchantView, error) {
	if a.TenantID != "" {
		return s.Merchants.Get(ctx, s.Tx.Reader(), a.TenantID)
	}
	return s.Merchants.FindByOwner(ctx, s.Tx.Reader(), a.UserID)
}

// tenant returns the merchant ID for merchant self-service routes (role MERCHANT + tenant_id).
func tenant(a Actor) (string, error) {
	if a.Role != "MERCHANT" || a.TenantID == "" {
		return "", ErrForbidden
	}
	return a.TenantID, nil
}

// PrincipalFromJWT re-checks merchant status on every request (Doc 7 C-4).
func (s *Service) PrincipalFromJWT(ctx context.Context, a Actor) (MerchantPrincipal, error) {
	id, err := tenant(a)
	if err != nil {
		return MerchantPrincipal{}, err
	}
	st, err := s.Merchants.Status(ctx, s.Tx.Reader(), id)
	if err != nil {
		return MerchantPrincipal{}, err
	}
	if st != domain.StatusApproved {
		return MerchantPrincipal{}, ErrNotApproved
	}
	all, _ := domain.NewScopeSet(nil)
	return MerchantPrincipal{MerchantID: id, Scopes: all}, nil
}

// ---------------- Admin status changes (F-02) ----------------

func (s *Service) ChangeStatus(ctx context.Context, a Actor, merchantID string, action domain.StatusAction, expectedVersion int, reason string) (MerchantView, error) {
	if a.Role != "ADMIN" {
		return MerchantView{}, ErrNotAdmin
	}
	var out MerchantView
	err := s.Tx.WithinTx(ctx, func(ctx context.Context, q Querier) error {
		v, err := s.Merchants.LockForUpdate(ctx, q, merchantID)
		if err != nil {
			return err
		}
		if v.Version != expectedVersion {
			return ErrVersionConflict
		}
		m := domain.Rehydrate(v.ID, v.OwnerUserID, v.BusinessName, v.Status, v.Version)
		now := s.Now().UTC()
		if err := m.Do(action, a.UserID, reason, now); err != nil {
			return err
		}
		if err := s.Merchants.SaveStatus(ctx, q, m, expectedVersion, strings.TrimSpace(reason)); err != nil {
			return err
		}
		extra := map[string]any{}
		switch action {
		case domain.ActionApprove:
			if err := s.Roles.GrantMerchantRole(ctx, q, v.OwnerUserID, v.ID); err != nil {
				return err
			}
		case domain.ActionDisable:
			nk, err := s.Creds.RevokeAll(ctx, q, v.ID, now)
			if err != nil {
				return err
			}
			nw, err := s.Hooks.DisableAll(ctx, q, v.ID)
			if err != nil {
				return err
			}
			extra["revoked_keys"], extra["disabled_webhooks"] = nk, nw
		}
		for _, e := range m.PullEvents() {
			newState := map[string]any{"status": e.To, "version": expectedVersion + 1}
			for k, val := range extra {
				newState[k] = val
			}
			if e.Reason != "" {
				newState["reason"] = e.Reason
			}
			if err := s.Audit.Write(ctx, q, AuditEntry{
				ActorID: a.UserID, ActorRole: a.Role, Action: "MERCHANT_" + pastTense(e.Action),
				TargetID: v.ID, RequestID: a.RequestID,
				Previous: map[string]any{"status": e.From, "version": expectedVersion}, New: newState,
			}); err != nil {
				return err
			}
		}
		out, err = s.Merchants.Get(ctx, q, merchantID)
		return err
	})
	if err == nil && s.Observer != nil {
		s.Observer.StatusChanged(string(action))
	}
	return out, err
}

func pastTense(a domain.StatusAction) string {
	switch a {
	case domain.ActionApprove:
		return "APPROVED"
	case domain.ActionReject:
		return "REJECTED"
	case domain.ActionSuspend:
		return "SUSPENDED"
	case domain.ActionReinstate:
		return "REINSTATED"
	default:
		return "DISABLED"
	}
}

func (s *Service) ListMerchants(ctx context.Context, a Actor, status string, page, perPage int) ([]MerchantView, int, error) {
	if a.Role != "ADMIN" {
		return nil, 0, ErrNotAdmin
	}
	return s.Merchants.List(ctx, s.Tx.Reader(), status, perPage, (page-1)*perPage)
}

func (s *Service) GetMerchantAdmin(ctx context.Context, a Actor, id string) (MerchantView, error) {
	if a.Role != "ADMIN" {
		return MerchantView{}, ErrNotAdmin
	}
	return s.Merchants.Get(ctx, s.Tx.Reader(), id)
}
