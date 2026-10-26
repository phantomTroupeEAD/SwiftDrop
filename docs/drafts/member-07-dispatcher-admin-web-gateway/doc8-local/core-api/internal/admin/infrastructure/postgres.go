// Package infrastructure implements the admin storage ports with PostgreSQL (pgx).
package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"swiftdrop.local/m7/core-api/internal/admin/application"
	"swiftdrop.local/m7/core-api/internal/admin/domain"
	"swiftdrop.local/m7/core-api/internal/platform/apperr"
	"swiftdrop.local/m7/core-api/internal/platform/web"
)

// zonePrefixLockKey is the advisory lock that makes prefix changes run one at a
// time (Doc 3 DD-07). Any constant works if every writer uses the same one.
const zonePrefixLockKey = 7_700_001

type Store struct {
	Pool        *pgxpool.Pool
	LockTimeout time.Duration
}

func (s *Store) InTx(ctx context.Context, fn func(application.Tx) error) error {
	err := pgx.BeginFunc(ctx, s.Pool, func(t pgx.Tx) error {
		if _, err := t.Exec(ctx, fmt.Sprintf("SET LOCAL lock_timeout = '%dms'", s.LockTimeout.Milliseconds())); err != nil {
			return err
		}
		return fn(&tx{t: t})
	})
	return mapPG(err)
}

// mapPG turns PostgreSQL error codes into error kinds the handler understands.
func mapPG(err error) error {
	var ce *pgconn.ConnectError
	if errors.As(err, &ce) {
		return fmt.Errorf("%w: %v", apperr.ErrUnavailable, err) // database down or unreachable
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch {
		case pe.Code == "23505" && (pe.ConstraintName == "delivery_zones_code_key" || pe.ConstraintName == "delivery_zones_pkey"):
			return apperr.ErrZoneCodeTaken // the id or the short code is already used
		case pe.Code == "23505" && pe.ConstraintName == "zone_postal_codes_pkey":
			return apperr.ErrPrefixConflict // last safety net behind the overlap check
		case pe.Code == "23514":
			return fmt.Errorf("%w: database check %s", apperr.ErrValidation, pe.ConstraintName)
		case pe.Code == "55P03" || pe.Code == "40001" || pe.Code == "40P01":
			return apperr.ErrBusy // lock timeout, serialization failure, deadlock
		case strings.HasPrefix(pe.Code, "08") || strings.HasPrefix(pe.Code, "57P"):
			return fmt.Errorf("%w: %v", apperr.ErrUnavailable, err) // connection lost, server shutting down
		case pe.Code == "57014":
			return fmt.Errorf("%w: statement timeout", apperr.ErrUnavailable)
		}
	}
	return err
}

type tx struct{ t pgx.Tx }

func (x *tx) LockZonePrefixes(ctx context.Context) error {
	_, err := x.t.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", zonePrefixLockKey)
	return err
}

func (x *tx) OtherZonePrefixes(ctx context.Context, exceptZoneID string) ([]domain.PrefixOwner, error) {
	rows, err := x.t.Query(ctx, `SELECT prefix, zone_id FROM zone_postal_codes WHERE zone_id <> $1`, exceptZoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PrefixOwner
	for rows.Next() {
		var p, zoneID string
		if err := rows.Scan(&p, &zoneID); err != nil {
			return nil, err
		}
		pp, _ := domain.NewPostalPrefix(p)
		out = append(out, domain.PrefixOwner{Prefix: pp, ZoneID: zoneID})
	}
	return out, rows.Err()
}

func (x *tx) InsertZone(ctx context.Context, z *domain.Zone) error {
	_, err := x.t.Exec(ctx, `INSERT INTO delivery_zones (id, code, name, description, enabled, version)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6)`, z.ID().String(), z.Code().String(), z.Name(), z.Description(), z.Enabled(), z.Version())
	if err != nil {
		return err
	}
	return x.writePrefixes(ctx, z)
}

func (x *tx) writePrefixes(ctx context.Context, z *domain.Zone) error {
	if _, err := x.t.Exec(ctx, "DELETE FROM zone_postal_codes WHERE zone_id = $1", z.ID().String()); err != nil {
		return err
	}
	for _, p := range z.Prefixes() {
		if _, err := x.t.Exec(ctx, "INSERT INTO zone_postal_codes (prefix, zone_id) VALUES ($1, $2)", p.String(), z.ID().String()); err != nil {
			return err
		}
	}
	return nil
}

const zoneCols = "z.id, z.code, z.name, COALESCE(z.description, ''), z.enabled, z.version"

type zoneRow struct {
	id, code, name, description string
	enabled                     bool
	version                     int
}

func scanZone(row pgx.Row) (r zoneRow, err error) {
	err = row.Scan(&r.id, &r.code, &r.name, &r.description, &r.enabled, &r.version)
	return
}

func loadZone(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, id string, forUpdate bool) (*domain.Zone, error) {
	sql := "SELECT " + zoneCols + " FROM delivery_zones z WHERE z.id = $1"
	if forUpdate {
		sql += " FOR UPDATE"
	}
	zr, err := scanZone(q.QueryRow(ctx, sql, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, "SELECT prefix FROM zone_postal_codes WHERE zone_id = $1 ORDER BY prefix", zr.id)
	if err != nil {
		return nil, err
	}
	ps, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	return restore(zr, ps), nil
}

// restore trusts the database CHECK constraints, which match the domain rules.
func restore(r zoneRow, ps []string) *domain.Zone {
	id, _ := domain.NewZoneID(r.id)
	zc, _ := domain.NewZoneCode(r.code)
	pp := make([]domain.PostalPrefix, 0, len(ps))
	for _, p := range ps {
		x, _ := domain.NewPostalPrefix(p)
		pp = append(pp, x)
	}
	return domain.Restore(id, zc, r.name, r.description, pp, r.enabled, r.version)
}

func (x *tx) LoadZone(ctx context.Context, id string) (*domain.Zone, error) {
	return loadZone(ctx, x.t, id, true)
}

// SaveZone is the optimistic lock: it only changes the row if nobody else did.
func (x *tx) SaveZone(ctx context.Context, z *domain.Zone, expectedVersion int) error {
	tag, err := x.t.Exec(ctx, `UPDATE delivery_zones SET code = $1, name = $2, description = NULLIF($3, ''), enabled = $4, version = $5,
		updated_at = now() WHERE id = $6 AND version = $7`,
		z.Code().String(), z.Name(), z.Description(), z.Enabled(), z.Version(), z.ID().String(), expectedVersion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrConcurrentConflict
	}
	return x.writePrefixes(ctx, z)
}

func (x *tx) LoadSetting(ctx context.Context, key string) (application.Setting, error) {
	var s application.Setting
	err := x.t.QueryRow(ctx, "SELECT key, value, value_type, description, version FROM system_settings WHERE key = $1 FOR UPDATE", key).
		Scan(&s.Key, &s.Value, &s.ValueType, &s.Description, &s.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, apperr.ErrNotFound
	}
	return s, err
}

func (x *tx) SaveSetting(ctx context.Context, key string, value json.RawMessage, expectedVersion int, actorID string) error {
	tag, err := x.t.Exec(ctx, `UPDATE system_settings SET value = $1, version = version + 1, updated_by = $2, updated_at = now()
		WHERE key = $3 AND version = $4`, value, actorID, key, expectedVersion)
	if err == nil && tag.RowsAffected() == 0 {
		return apperr.ErrConcurrentConflict
	}
	return err
}

func (x *tx) Audit(ctx context.Context, a application.Actor, e application.AuditEntry) error {
	var ip any
	if parsed := net.ParseIP(a.IP); parsed != nil {
		ip = parsed.String()
	}
	ua := a.UserAgent
	if len(ua) > 300 {
		ua = ua[:300]
	}
	_, err := x.t.Exec(ctx, `INSERT INTO audit_logs (actor_id, actor_role, action, target_type, target_id, previous_state, new_state,
		reason, client_ip, user_agent, request_id) VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9::inet, $10, $11)`,
		a.ID, a.Role, e.Action, e.TargetType, e.TargetID, jsonOrNil(e.Before), jsonOrNil(e.After), e.Reason, ip, ua, a.RequestID)
	return err
}

// jsonOrNil redacts secrets (Doc 7 section 10.3) and encodes the snapshot.
func jsonOrNil(m map[string]any) any {
	if m == nil {
		return nil
	}
	b, _ := json.Marshal(web.RedactForAudit(m))
	return b
}

func (s *Store) ListZones(ctx context.Context) ([]*domain.Zone, error) {
	z, err := s.listZones(ctx)
	return z, mapPG(err)
}

func (s *Store) listZones(ctx context.Context) ([]*domain.Zone, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+zoneCols+`, COALESCE(array_agg(p.prefix ORDER BY p.prefix) FILTER (WHERE p.prefix IS NOT NULL), '{}')
		FROM delivery_zones z LEFT JOIN zone_postal_codes p ON p.zone_id = z.id GROUP BY z.id ORDER BY z.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Zone
	for rows.Next() {
		var r zoneRow
		var ps []string
		if err := rows.Scan(&r.id, &r.code, &r.name, &r.description, &r.enabled, &r.version, &ps); err != nil {
			return nil, err
		}
		out = append(out, restore(r, ps))
	}
	return out, rows.Err()
}

func (s *Store) ListPublicZones(ctx context.Context) ([]application.PublicZone, error) {
	rows, err := s.Pool.Query(ctx, "SELECT id, code, name FROM delivery_zones WHERE enabled ORDER BY name")
	if err != nil {
		return nil, mapPG(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (application.PublicZone, error) {
		var z application.PublicZone
		return z, r.Scan(&z.ID, &z.Code, &z.Name)
	})
	return out, mapPG(err)
}

// ZoneForPostalCode picks the longest prefix that starts the postal code. Overlaps
// are blocked on write, so there is at most one, but ORDER BY keeps it safe.
func (s *Store) ZoneForPostalCode(ctx context.Context, postalCode string) (application.PublicZone, error) {
	var z application.PublicZone
	err := s.Pool.QueryRow(ctx, `SELECT z.id, z.code, z.name FROM zone_postal_codes p JOIN delivery_zones z ON z.id = p.zone_id
		WHERE z.enabled AND $1 LIKE p.prefix || '%' ORDER BY length(p.prefix) DESC LIMIT 1`, postalCode).Scan(&z.ID, &z.Code, &z.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return z, apperr.ErrZoneNotFound
	}
	return z, mapPG(err)
}

func (s *Store) GetZone(ctx context.Context, id string) (*domain.Zone, error) {
	z, err := loadZone(ctx, s.Pool, id, false)
	return z, mapPG(err)
}

func (s *Store) ListSettings(ctx context.Context) ([]application.Setting, error) {
	rows, err := s.Pool.Query(ctx, "SELECT key, value, value_type, description, version FROM system_settings ORDER BY key")
	if err != nil {
		return nil, mapPG(err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (application.Setting, error) {
		var s application.Setting
		err := r.Scan(&s.Key, &s.Value, &s.ValueType, &s.Description, &s.Version)
		return s, err
	})
}

func (s *Store) ListAudit(ctx context.Context, limit int) ([]application.AuditRow, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id::text, actor_id::text, actor_role, action, target_type, target_id, previous_state, new_state,
		reason, request_id, created_at FROM audit_logs ORDER BY created_at DESC, id LIMIT $1`, limit)
	if err != nil {
		return nil, mapPG(err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (application.AuditRow, error) {
		var a application.AuditRow
		err := r.Scan(&a.ID, &a.ActorID, &a.ActorRole, &a.Action, &a.TargetType, &a.TargetID, &a.Before, &a.After, &a.Reason, &a.RequestID, &a.CreatedAt)
		return a, err
	})
}
