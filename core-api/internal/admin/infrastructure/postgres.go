// Package infrastructure implements the admin storage ports with PostgreSQL
// (database/sql + lib/pq).
package infrastructure

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/swiftdrop/core-api/internal/admin/application"
	"github.com/swiftdrop/core-api/internal/admin/domain"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

// zonePrefixLockKey is the advisory lock that makes prefix changes run one at a
// time (Doc 3 DD-07). Any constant works if every writer uses the same one.
const zonePrefixLockKey = 7_700_001

type Store struct {
	DB          *sql.DB
	LockTimeout time.Duration
}

func (s *Store) InTx(ctx context.Context, fn func(application.Tx) error) error {
	err := db.WithTx(ctx, s.DB, func(t *sql.Tx) error {
		timeout := fmt.Sprintf("%dms", s.LockTimeout.Milliseconds())
		if _, err := t.ExecContext(ctx, "SELECT set_config('lock_timeout', $1, true)", timeout); err != nil {
			return err
		}
		return fn(&tx{t: t})
	})
	return mapPG(err)
}

// mapPG turns PostgreSQL error codes into error kinds the handler understands.
// Errors that are already application kinds pass through unchanged.
func mapPG(err error) error {
	if err == nil {
		return nil
	}
	var netErr *net.OpError
	if errors.Is(err, driver.ErrBadConn) || errors.As(err, &netErr) {
		return fmt.Errorf("%w: %v", application.ErrUnavailable, err) // database down or unreachable
	}
	var pe *pq.Error
	if errors.As(err, &pe) {
		code := string(pe.Code)
		switch {
		case code == "23505" && (pe.Constraint == "delivery_zones_code_key" || pe.Constraint == "delivery_zones_pkey"):
			return application.ErrZoneCodeTaken // the id or the short code is already used
		case code == "23505" && pe.Constraint == "zone_postal_codes_pkey":
			return application.ErrPrefixConflict // last safety net behind the overlap check
		case code == "23514":
			return fmt.Errorf("%w: database check %s", application.ErrValidation, pe.Constraint)
		case code == "55P03" || code == "40001" || code == "40P01":
			return application.ErrBusy // lock timeout, serialization failure, deadlock
		case strings.HasPrefix(code, "08") || strings.HasPrefix(code, "57P"):
			return fmt.Errorf("%w: %v", application.ErrUnavailable, err) // connection lost, server shutting down
		case code == "57014":
			return fmt.Errorf("%w: statement timeout", application.ErrUnavailable)
		}
	}
	return err
}

type tx struct{ t *sql.Tx }

func (x *tx) LockZonePrefixes(ctx context.Context) error {
	_, err := x.t.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", zonePrefixLockKey)
	return err
}

func (x *tx) OtherZonePrefixes(ctx context.Context, exceptZoneID string) ([]domain.PrefixOwner, error) {
	rows, err := x.t.QueryContext(ctx, "SELECT prefix, zone_id FROM zone_postal_codes WHERE zone_id <> $1", exceptZoneID)
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
	_, err := x.t.ExecContext(ctx, `INSERT INTO delivery_zones (id, code, name, description, enabled, version)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6)`, z.ID().String(), z.Code().String(), z.Name(), z.Description(), z.Enabled(), z.Version())
	if err != nil {
		return err
	}
	return x.writePrefixes(ctx, z)
}

func (x *tx) writePrefixes(ctx context.Context, z *domain.Zone) error {
	if _, err := x.t.ExecContext(ctx, "DELETE FROM zone_postal_codes WHERE zone_id = $1", z.ID().String()); err != nil {
		return err
	}
	for _, p := range z.Prefixes() {
		if _, err := x.t.ExecContext(ctx, "INSERT INTO zone_postal_codes (prefix, zone_id) VALUES ($1, $2)", p.String(), z.ID().String()); err != nil {
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

const (
	selectZone          = "SELECT " + zoneCols + " FROM delivery_zones z WHERE z.id = $1"
	selectZoneForUpdate = selectZone + " FOR UPDATE"
)

func loadZone(ctx context.Context, q db.Querier, id string, forUpdate bool) (*domain.Zone, error) {
	query := selectZone
	if forUpdate {
		query = selectZoneForUpdate
	}
	var zr zoneRow
	err := q.QueryRowContext(ctx, query, id).Scan(&zr.id, &zr.code, &zr.name, &zr.description, &zr.enabled, &zr.version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, application.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, "SELECT prefix FROM zone_postal_codes WHERE zone_id = $1 ORDER BY prefix", zr.id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ps []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	if err := rows.Err(); err != nil {
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
	res, err := x.t.ExecContext(ctx, `UPDATE delivery_zones SET code = $1, name = $2, description = NULLIF($3, ''), enabled = $4, version = $5,
		updated_at = now() WHERE id = $6 AND version = $7`,
		z.Code().String(), z.Name(), z.Description(), z.Enabled(), z.Version(), z.ID().String(), expectedVersion)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return application.ErrConcurrentConflict
	}
	return x.writePrefixes(ctx, z)
}

func (x *tx) LoadSetting(ctx context.Context, key string) (application.Setting, error) {
	var s application.Setting
	var value []byte
	err := x.t.QueryRowContext(ctx, "SELECT key, value, value_type, COALESCE(description, ''), version FROM system_settings WHERE key = $1 FOR UPDATE", key).
		Scan(&s.Key, &value, &s.ValueType, &s.Description, &s.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return s, application.ErrNotFound
	}
	s.Value = value
	return s, err
}

// SaveSetting sends the value as text: lib/pq would send []byte as bytea, which jsonb refuses.
func (x *tx) SaveSetting(ctx context.Context, key string, value json.RawMessage, expectedVersion int, actorID string) error {
	res, err := x.t.ExecContext(ctx, `UPDATE system_settings SET value = $1::jsonb, version = version + 1, updated_by = NULLIF($2, '')::uuid, updated_at = now()
		WHERE key = $3 AND version = $4`, string(value), actorID, key, expectedVersion)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return application.ErrConcurrentConflict
	}
	return nil
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
	_, err := x.t.ExecContext(ctx, `INSERT INTO audit_logs (actor_id, actor_role, action, target_type, target_id, previous_state, new_state,
		reason, client_ip, user_agent, request_id)
		VALUES (NULLIF($1, '')::uuid, $2, $3, $4, $5, $6::jsonb, $7::jsonb, NULLIF($8, ''), $9::inet, NULLIF($10, ''), NULLIF($11, ''))`,
		a.ID, a.Role, e.Action, e.TargetType, e.TargetID, jsonOrNil(e.Before), jsonOrNil(e.After), e.Reason, ip, ua, a.RequestID)
	return err
}

// jsonOrNil redacts secrets (Doc 7 section 10.3) and encodes the snapshot as text.
func jsonOrNil(m map[string]any) any {
	if m == nil {
		return nil
	}
	b, _ := json.Marshal(redactForAudit(m))
	return string(b)
}

func (s *Store) ListZones(ctx context.Context, offset, limit int) ([]*domain.Zone, int, error) {
	z, total, err := s.listZones(ctx, offset, limit)
	return z, total, mapPG(err)
}

func (s *Store) listZones(ctx context.Context, offset, limit int) ([]*domain.Zone, int, error) {
	var total int
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM delivery_zones").Scan(&total); err != nil {
		return nil, 0, err
	}
	// The page is cut in a subquery first, so only its zones are joined to their prefixes.
	rows, err := s.DB.QueryContext(ctx, `SELECT `+zoneCols+`, COALESCE(array_agg(p.prefix ORDER BY p.prefix) FILTER (WHERE p.prefix IS NOT NULL), '{}')
		FROM (SELECT * FROM delivery_zones ORDER BY id LIMIT $1 OFFSET $2) z
		LEFT JOIN zone_postal_codes p ON p.zone_id = z.id GROUP BY z.id, z.code, z.name, z.description, z.enabled, z.version ORDER BY z.id`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*domain.Zone{}
	for rows.Next() {
		var r zoneRow
		var ps []string
		if err := rows.Scan(&r.id, &r.code, &r.name, &r.description, &r.enabled, &r.version, pq.Array(&ps)); err != nil {
			return nil, 0, err
		}
		out = append(out, restore(r, ps))
	}
	return out, total, rows.Err()
}

func (s *Store) ListPublicZones(ctx context.Context) ([]application.PublicZone, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, code, name FROM delivery_zones WHERE enabled ORDER BY name")
	if err != nil {
		return nil, mapPG(err)
	}
	defer rows.Close()
	out := []application.PublicZone{}
	for rows.Next() {
		var z application.PublicZone
		if err := rows.Scan(&z.ID, &z.Code, &z.Name); err != nil {
			return nil, mapPG(err)
		}
		out = append(out, z)
	}
	return out, mapPG(rows.Err())
}

// ZoneForPostalCode picks the longest prefix that starts the postal code. Overlaps
// are blocked on write, so there is at most one, but ORDER BY keeps it safe.
func (s *Store) ZoneForPostalCode(ctx context.Context, postalCode string) (application.PublicZone, error) {
	var z application.PublicZone
	err := s.DB.QueryRowContext(ctx, `SELECT z.id, z.code, z.name FROM zone_postal_codes p JOIN delivery_zones z ON z.id = p.zone_id
		WHERE z.enabled AND $1 LIKE p.prefix || '%' ORDER BY length(p.prefix) DESC LIMIT 1`, postalCode).Scan(&z.ID, &z.Code, &z.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return z, application.ErrZoneNotFound
	}
	return z, mapPG(err)
}

func (s *Store) GetZone(ctx context.Context, id string) (*domain.Zone, error) {
	z, err := loadZone(ctx, s.DB, id, false)
	return z, mapPG(err)
}

func (s *Store) ListSettings(ctx context.Context) ([]application.Setting, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT key, value, value_type, COALESCE(description, ''), version FROM system_settings ORDER BY key")
	if err != nil {
		return nil, mapPG(err)
	}
	defer rows.Close()
	out := []application.Setting{}
	for rows.Next() {
		var st application.Setting
		var value []byte
		if err := rows.Scan(&st.Key, &value, &st.ValueType, &st.Description, &st.Version); err != nil {
			return nil, mapPG(err)
		}
		st.Value = value
		out = append(out, st)
	}
	return out, mapPG(rows.Err())
}

func (s *Store) ListAudit(ctx context.Context, offset, limit int) ([]application.AuditRow, int, error) {
	var total int
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM audit_logs").Scan(&total); err != nil {
		return nil, 0, mapPG(err)
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id::text, actor_id::text, actor_role, action, target_type, target_id, previous_state, new_state,
		reason, request_id, created_at FROM audit_logs ORDER BY created_at DESC, id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, mapPG(err)
	}
	defer rows.Close()
	out := []application.AuditRow{}
	for rows.Next() {
		var a application.AuditRow
		var actorID, reason, requestID sql.NullString
		var before, after []byte
		if err := rows.Scan(&a.ID, &actorID, &a.ActorRole, &a.Action, &a.TargetType, &a.TargetID, &before, &after, &reason, &requestID, &a.CreatedAt); err != nil {
			return nil, 0, mapPG(err)
		}
		a.ActorID, a.Reason, a.RequestID = strPtr(actorID), strPtr(reason), strPtr(requestID)
		a.Before, a.After = before, after
		out = append(out, a)
	}
	return out, total, mapPG(rows.Err())
}

func strPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	return &n.String
}
