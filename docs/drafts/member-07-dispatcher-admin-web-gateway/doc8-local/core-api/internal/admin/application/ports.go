// Package application holds the admin use cases. It talks to storage only through
// the interfaces below; infrastructure/postgres implements them.
package application

import (
	"context"
	"encoding/json"
	"time"

	"swiftdrop.local/m7/core-api/internal/admin/domain"
)

// Actor is who asked, taken from the verified token plus request facts for audit.
type Actor struct {
	ID, Role, IP, UserAgent, RequestID string
}

type AuditEntry struct {
	Action, TargetType, TargetID string
	Before, After                map[string]any
	Reason                       string
}

// Tx is one database transaction. Every write and its audit row share one Tx.
type Tx interface {
	LockZonePrefixes(ctx context.Context) error
	OtherZonePrefixes(ctx context.Context, exceptZoneID string) ([]domain.PrefixOwner, error)
	InsertZone(ctx context.Context, z *domain.Zone) error
	LoadZone(ctx context.Context, id string) (*domain.Zone, error)
	SaveZone(ctx context.Context, z *domain.Zone, expectedVersion int) error
	LoadSetting(ctx context.Context, key string) (Setting, error)
	SaveSetting(ctx context.Context, key string, value json.RawMessage, expectedVersion int, actorID string) error
	Audit(ctx context.Context, a Actor, e AuditEntry) error
}

type Store interface {
	InTx(ctx context.Context, fn func(Tx) error) error
	ListZones(ctx context.Context) ([]*domain.Zone, error)
	GetZone(ctx context.Context, id string) (*domain.Zone, error)
	ListPublicZones(ctx context.Context) ([]PublicZone, error)
	// ZoneForPostalCode returns the enabled zone with the longest matching prefix.
	ZoneForPostalCode(ctx context.Context, postalCode string) (PublicZone, error)
	ListSettings(ctx context.Context) ([]Setting, error)
	ListAudit(ctx context.Context, limit int) ([]AuditRow, error)
}

type Setting struct {
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	ValueType   string          `json:"value_type"`
	Description string          `json:"description"`
	Version     int             `json:"version"`
}

type AuditRow struct {
	ID         string          `json:"id"`
	ActorID    *string         `json:"actor_id"`
	ActorRole  string          `json:"actor_role"`
	Action     string          `json:"action"`
	TargetType string          `json:"target_type"`
	TargetID   string          `json:"target_id"`
	Before     json.RawMessage `json:"previous_state"`
	After      json.RawMessage `json:"new_state"`
	Reason     *string         `json:"reason"`
	RequestID  *string         `json:"request_id"`
	CreatedAt  time.Time       `json:"created_at"`
}
