// Package application holds the merchants use cases and the ports they need (Doc 3 §6.2).
// It imports only domain and the standard library.
package application

import (
	"context"
	"net/netip"
	"time"

	"swiftdrop.local/m4/core-api/internal/merchants/domain"
)

// Querier is an opaque DB handle (pool or tx) passed explicitly (U-20).
type Querier = any

type TxRunner interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, q Querier) error) error
	Reader() Querier
}

type MerchantView struct {
	ID           string
	OwnerUserID  string
	BusinessName string
	Status       domain.MerchantStatus
	StatusReason string
	ContactEmail string
	ContactPhone string
	Address      domain.Address
	ApprovedAt   *time.Time
	Version      int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type MerchantRepo interface {
	Insert(ctx context.Context, q Querier, v MerchantView) error
	Get(ctx context.Context, q Querier, id string) (MerchantView, error)
	FindByOwner(ctx context.Context, q Querier, ownerUserID string) (MerchantView, error)
	Status(ctx context.Context, q Querier, id string) (domain.MerchantStatus, error)
	LockForUpdate(ctx context.Context, q Querier, id string) (MerchantView, error)
	SaveStatus(ctx context.Context, q Querier, m *domain.Merchant, expectedVersion int, reason string) error
	List(ctx context.Context, q Querier, status string, limit, offset int) ([]MerchantView, int, error)
}

type CredentialRecord struct {
	ID, MerchantID, Prefix, Hash, Label, CreatedBy string
	PepperVersion                                  int
	Scopes                                         []string
	ExpiresAt                                      *time.Time
}

type CredentialView struct {
	ID, Label, Prefix string
	Scopes            []string
	CreatedAt         time.Time
	ExpiresAt         *time.Time
	RevokedAt         *time.Time
	LastUsedAt        *time.Time
}

type AuthRecord struct {
	CredentialID, MerchantID, Hash string
	PepperVersion                  int
	Scopes                         []string
	ExpiresAt, RevokedAt           *time.Time
	MerchantStatus                 domain.MerchantStatus
}

type CredentialRepo interface {
	Insert(ctx context.Context, q Querier, r CredentialRecord) error
	// FindByPrefix is the only non-tenant-scoped lookup (authentication, Doc 7 §3.3).
	FindByPrefix(ctx context.Context, q Querier, prefix string) (AuthRecord, error)
	List(ctx context.Context, q Querier, merchantID string) ([]CredentialView, error)
	CountActive(ctx context.Context, q Querier, merchantID string, now time.Time) (int, error)
	Revoke(ctx context.Context, q Querier, merchantID, id string, now time.Time) (changed, exists bool, err error)
	RevokeAll(ctx context.Context, q Querier, merchantID string, now time.Time) (int, error)
	TouchLastUsed(ctx context.Context, q Querier, id string, now time.Time) error
}

type WebhookRecord struct {
	ID, MerchantID, URL, Last4, CreatedBy string
	Ciphertext                            []byte
	KeyVersion                            int
	Events                                []string
}

type WebhookView struct {
	ID, URL, Last4       string
	Events               []string
	Enabled              bool
	CreatedAt, UpdatedAt time.Time
}

type WebhookRepo interface {
	Insert(ctx context.Context, q Querier, r WebhookRecord) error
	List(ctx context.Context, q Querier, merchantID string) ([]WebhookView, error)
	CountLive(ctx context.Context, q Querier, merchantID string) (int, error)
	SoftDelete(ctx context.Context, q Querier, merchantID, id string, now time.Time) (bool, error)
	DisableAll(ctx context.Context, q Querier, merchantID string) (int, error)
}

type AuditEntry struct {
	ActorID, ActorRole, Action, TargetID, RequestID string
	Previous, New                                   map[string]any
}

type AuditWriter interface {
	Write(ctx context.Context, q Querier, e AuditEntry) error
}

type RoleAssigner interface { // Member 1 (U-15)
	GrantMerchantRole(ctx context.Context, q Querier, userID, merchantID string) error
}

type KeyGenerator interface {
	NewAPIKey(envPrefix string) (domain.PlainAPIKey, error)
	NewSigningSecret() (string, error)
}

type KeyHasher interface {
	CurrentVersion() int
	Hash(pepperVersion int, fullKey string) (string, error)
	Equal(a, b string) bool
}

type SecretCipher interface {
	Encrypt(plain string) (ciphertext []byte, keyVersion int, err error)
}

type HostResolver interface {
	LookupAddrs(ctx context.Context, host string) ([]netip.Addr, error)
}

// ParcelBooking is Member 2's capability (Doc 4 §5.1). A stand-in is used locally.
type ParcelBooking interface {
	CreateForMerchant(ctx context.Context, merchantID, idemKey, requestHash string, req OrderRequest) (OrderResult, bool, error)
	ListForMerchant(ctx context.Context, merchantID string) ([]OrderResult, error)
}

// Observer receives security metrics (Doc 7 §11.2).
type Observer interface {
	AuthResult(result string)
	SSRFBlocked()
	StatusChanged(action string)
}
