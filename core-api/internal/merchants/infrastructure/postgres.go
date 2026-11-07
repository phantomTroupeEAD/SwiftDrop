// Package infrastructure implements the merchants ports with PostgreSQL (database/sql + lib/pq).
package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/swiftdrop/core-api/internal/merchants/application"
	"github.com/swiftdrop/core-api/internal/merchants/domain"
)

// sqlQuerier is satisfied by both *sql.DB and *sql.Tx (database/sql standard library).
type sqlQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// asQ safely casts application.Querier to sqlQuerier without panicking.
func asQ(querier application.Querier) (sqlQuerier, error) {
	switch v := querier.(type) {
	case *sql.DB:
		return v, nil
	case *sql.Tx:
		return v, nil
	default:
		return nil, fmt.Errorf("merchants/infrastructure: unexpected or nil Querier type %T (expected *sql.DB or *sql.Tx)", querier)
	}
}

// pgConstraint extracts the PostgreSQL error code and constraint name from a lib/pq error.
func pgConstraint(err error) (code, constraint string) {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code), pqErr.Constraint
	}
	return "", ""
}

// mapErr turns PostgreSQL constraint errors into application errors (FM-03).
func mapErr(err error, notFound error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return notFound
	}
	code, constraint := pgConstraint(err)
	if code == "23505" { // unique_violation
		switch constraint {
		case "uq_merchants_active_owner":
			return application.ErrMerchantExists
		case "uq_credentials_key_prefix":
			return application.ErrPrefixCollision
		case "uq_credentials_merchant_label_active":
			return application.ErrLabelDuplicate
		case "uq_webhooks_merchant_url_live":
			return application.ErrWebhookDuplicate
		}
	}
	return err
}

// ---------------- merchants ----------------

// MerchantRepo implements application.MerchantRepo against PostgreSQL.
type MerchantRepo struct{}

const merchantCols = `id, owner_user_id, business_name, status, coalesce(status_reason,''), contact_email,
	contact_phone, address, approved_at, version, created_at, updated_at`

func scanMerchant(row *sql.Row) (application.MerchantView, error) {
	var v application.MerchantView
	var addr []byte
	var st string
	var approvedAt sql.NullTime
	err := row.Scan(
		&v.ID, &v.OwnerUserID, &v.BusinessName, &st, &v.StatusReason, &v.ContactEmail,
		&v.ContactPhone, &addr, &approvedAt, &v.Version, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return v, err
	}
	v.Status = domain.MerchantStatus(st)
	if approvedAt.Valid {
		v.ApprovedAt = &approvedAt.Time
	}
	if err := json.Unmarshal(addr, &v.Address); err != nil {
		return v, fmt.Errorf("merchants: malformed address JSON in database: %w", err)
	}
	return v, nil
}

func scanMerchantRow(rows *sql.Rows) (application.MerchantView, error) {
	var v application.MerchantView
	var addr []byte
	var st string
	var approvedAt sql.NullTime
	err := rows.Scan(
		&v.ID, &v.OwnerUserID, &v.BusinessName, &st, &v.StatusReason, &v.ContactEmail,
		&v.ContactPhone, &addr, &approvedAt, &v.Version, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return v, err
	}
	v.Status = domain.MerchantStatus(st)
	if approvedAt.Valid {
		v.ApprovedAt = &approvedAt.Time
	}
	if err := json.Unmarshal(addr, &v.Address); err != nil {
		return v, fmt.Errorf("merchants: malformed address JSON in database: %w", err)
	}
	return v, nil
}

func (MerchantRepo) Insert(ctx context.Context, querier application.Querier, v application.MerchantView) error {
	q, err := asQ(querier)
	if err != nil {
		return err
	}
	addr, err := json.Marshal(v.Address)
	if err != nil {
		return fmt.Errorf("merchants: marshal address JSON: %w", err)
	}
	_, err = q.ExecContext(ctx,
		`INSERT INTO merchants (id, owner_user_id, business_name, status, contact_email,
		contact_phone, address, version, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,1,$8,$8)`,
		v.ID, v.OwnerUserID, v.BusinessName, string(v.Status), v.ContactEmail, v.ContactPhone, addr, v.CreatedAt)
	return mapErr(err, application.ErrMerchantNotFound)
}

func (MerchantRepo) Get(ctx context.Context, querier application.Querier, id string) (application.MerchantView, error) {
	q, err := asQ(querier)
	if err != nil {
		return application.MerchantView{}, err
	}
	row := q.QueryRowContext(ctx, `SELECT `+merchantCols+` FROM merchants WHERE id=$1`, id)
	v, err := scanMerchant(row)
	return v, mapErr(err, application.ErrMerchantNotFound)
}

func (MerchantRepo) FindByOwner(ctx context.Context, querier application.Querier, owner string) (application.MerchantView, error) {
	q, err := asQ(querier)
	if err != nil {
		return application.MerchantView{}, err
	}
	row := q.QueryRowContext(ctx,
		`SELECT `+merchantCols+` FROM merchants WHERE owner_user_id=$1 ORDER BY created_at DESC LIMIT 1`, owner)
	v, err := scanMerchant(row)
	return v, mapErr(err, application.ErrMerchantNotFound)
}

func (MerchantRepo) LockForUpdate(ctx context.Context, querier application.Querier, id string) (application.MerchantView, error) {
	q, err := asQ(querier)
	if err != nil {
		return application.MerchantView{}, err
	}
	row := q.QueryRowContext(ctx, `SELECT `+merchantCols+` FROM merchants WHERE id=$1 FOR UPDATE`, id)
	v, err := scanMerchant(row)
	return v, mapErr(err, application.ErrMerchantNotFound)
}

func (MerchantRepo) Status(ctx context.Context, querier application.Querier, id string) (domain.MerchantStatus, error) {
	q, err := asQ(querier)
	if err != nil {
		return "", err
	}
	var s string
	err = q.QueryRowContext(ctx, `SELECT status FROM merchants WHERE id=$1`, id).Scan(&s)
	return domain.MerchantStatus(s), mapErr(err, application.ErrMerchantNotFound)
}

// SaveStatus applies optimistic locking (BR-M-09).
func (MerchantRepo) SaveStatus(ctx context.Context, querier application.Querier, m *domain.Merchant, expected int, reason string) error {
	q, err := asQ(querier)
	if err != nil {
		return err
	}
	result, err := q.ExecContext(ctx,
		`UPDATE merchants SET status=$1, status_reason=NULLIF($2,''),
		approved_by=COALESCE($3, approved_by), approved_at=COALESCE($4, approved_at),
		version=version+1, updated_at=now() WHERE id=$5 AND version=$6`,
		string(m.Status()), reason, m.ApprovedBy(), m.ApprovedAt(), m.ID(), expected)
	if err != nil {
		return mapErr(err, application.ErrMerchantNotFound)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("merchants: RowsAffected: %w", err)
	}
	if rows == 0 {
		return application.ErrVersionConflict
	}
	return nil
}

func (MerchantRepo) List(ctx context.Context, querier application.Querier, status string, limit, offset int) ([]application.MerchantView, int, error) {
	q, err := asQ(querier)
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM merchants WHERE ($1='' OR status=$1)`, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := q.QueryContext(ctx,
		`SELECT `+merchantCols+` FROM merchants WHERE ($1='' OR status=$1)
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []application.MerchantView
	for rows.Next() {
		v, err := scanMerchantRow(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

// ---------------- credentials ----------------

// CredentialRepo implements application.CredentialRepo against PostgreSQL.
type CredentialRepo struct{}

func (CredentialRepo) Insert(ctx context.Context, querier application.Querier, r application.CredentialRecord) error {
	q, err := asQ(querier)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx,
		`INSERT INTO merchant_api_credentials
		(id, merchant_id, key_prefix, key_hash, pepper_version, label, scopes, created_by, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		r.ID, r.MerchantID, r.Prefix, r.Hash, r.PepperVersion, r.Label, pq.Array(r.Scopes), r.CreatedBy, r.ExpiresAt)
	return mapErr(err, application.ErrNotFound)
}

func (CredentialRepo) FindByPrefix(ctx context.Context, querier application.Querier, prefix string) (application.AuthRecord, error) {
	q, err := asQ(querier)
	if err != nil {
		return application.AuthRecord{}, err
	}
	var r application.AuthRecord
	var st string
	var scopes pq.StringArray
	err = q.QueryRowContext(ctx,
		`SELECT c.id, c.merchant_id, c.key_hash, c.pepper_version, c.scopes,
		c.expires_at, c.revoked_at, m.status
		FROM merchant_api_credentials c JOIN merchants m ON m.id = c.merchant_id WHERE c.key_prefix=$1`, prefix).
		Scan(&r.CredentialID, &r.MerchantID, &r.Hash, &r.PepperVersion, &scopes, &r.ExpiresAt, &r.RevokedAt, &st)
	r.Scopes = []string(scopes)
	r.MerchantStatus = domain.MerchantStatus(st)
	return r, mapErr(err, application.ErrNotFound)
}

func (CredentialRepo) List(ctx context.Context, querier application.Querier, merchantID string) ([]application.CredentialView, error) {
	q, err := asQ(querier)
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx,
		`SELECT id, label, key_prefix, scopes, created_at, expires_at, revoked_at, last_used_at
		FROM merchant_api_credentials WHERE merchant_id=$1 ORDER BY created_at DESC`, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []application.CredentialView{}
	for rows.Next() {
		var v application.CredentialView
		var scopes pq.StringArray
		if err := rows.Scan(&v.ID, &v.Label, &v.Prefix, &scopes, &v.CreatedAt, &v.ExpiresAt, &v.RevokedAt, &v.LastUsedAt); err != nil {
			return nil, err
		}
		v.Scopes = []string(scopes)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (CredentialRepo) CountActive(ctx context.Context, querier application.Querier, merchantID string, now time.Time) (int, error) {
	q, err := asQ(querier)
	if err != nil {
		return 0, err
	}
	var n int
	err = q.QueryRowContext(ctx,
		`SELECT count(*) FROM merchant_api_credentials WHERE merchant_id=$1
		AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > $2)`, merchantID, now).Scan(&n)
	return n, err
}

func (CredentialRepo) Revoke(ctx context.Context, querier application.Querier, merchantID, id string, now time.Time) (changed, exists bool, err error) {
	q, err := asQ(querier)
	if err != nil {
		return false, false, err
	}
	result, err := q.ExecContext(ctx,
		`UPDATE merchant_api_credentials SET revoked_at=$3
		WHERE id=$1 AND merchant_id=$2 AND revoked_at IS NULL`, id, merchantID, now)
	if err != nil {
		return false, false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, false, fmt.Errorf("merchants: RowsAffected: %w", err)
	}
	if rows == 1 {
		return true, true, nil
	}
	err = q.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM merchant_api_credentials WHERE id=$1 AND merchant_id=$2)`,
		id, merchantID).Scan(&exists)
	return false, exists, err
}

func (CredentialRepo) RevokeAll(ctx context.Context, querier application.Querier, merchantID string, now time.Time) (int, error) {
	q, err := asQ(querier)
	if err != nil {
		return 0, err
	}
	result, err := q.ExecContext(ctx,
		`UPDATE merchant_api_credentials SET revoked_at=$2
		WHERE merchant_id=$1 AND revoked_at IS NULL`, merchantID, now)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("merchants: RowsAffected: %w", err)
	}
	return int(rows), nil
}

func (CredentialRepo) TouchLastUsed(ctx context.Context, querier application.Querier, id string, now time.Time) error {
	q, err := asQ(querier)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx,
		`UPDATE merchant_api_credentials SET last_used_at=$2 WHERE id=$1`, id, now)
	return err
}

// ---------------- webhooks ----------------

// WebhookRepo implements application.WebhookRepo against PostgreSQL.
type WebhookRepo struct{}

func (WebhookRepo) Insert(ctx context.Context, querier application.Querier, r application.WebhookRecord) error {
	q, err := asQ(querier)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx,
		`INSERT INTO webhook_endpoints (id, merchant_id, url, secret_ciphertext,
		secret_key_version, secret_last4, subscribed_event_types, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		r.ID, r.MerchantID, r.URL, r.Ciphertext, r.KeyVersion, r.Last4, pq.Array(r.Events), r.CreatedBy)
	return mapErr(err, application.ErrNotFound)
}

func (WebhookRepo) List(ctx context.Context, querier application.Querier, merchantID string) ([]application.WebhookView, error) {
	q, err := asQ(querier)
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx,
		`SELECT id, url, secret_last4, subscribed_event_types, enabled, created_at, updated_at
		FROM webhook_endpoints WHERE merchant_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC`, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []application.WebhookView{}
	for rows.Next() {
		var v application.WebhookView
		var events pq.StringArray
		if err := rows.Scan(&v.ID, &v.URL, &v.Last4, &events, &v.Enabled, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		v.Events = []string(events)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (WebhookRepo) CountLive(ctx context.Context, querier application.Querier, merchantID string) (int, error) {
	q, err := asQ(querier)
	if err != nil {
		return 0, err
	}
	var n int
	err = q.QueryRowContext(ctx,
		`SELECT count(*) FROM webhook_endpoints WHERE merchant_id=$1 AND deleted_at IS NULL`, merchantID).Scan(&n)
	return n, err
}

func (WebhookRepo) SoftDelete(ctx context.Context, querier application.Querier, merchantID, id string, now time.Time) (bool, error) {
	q, err := asQ(querier)
	if err != nil {
		return false, err
	}
	result, err := q.ExecContext(ctx,
		`UPDATE webhook_endpoints SET deleted_at=$3, enabled=false, updated_at=$3
		WHERE id=$1 AND merchant_id=$2 AND deleted_at IS NULL`, id, merchantID, now)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("merchants: RowsAffected: %w", err)
	}
	return rows == 1, nil
}

func (WebhookRepo) DisableAll(ctx context.Context, querier application.Querier, merchantID string) (int, error) {
	q, err := asQ(querier)
	if err != nil {
		return 0, err
	}
	result, err := q.ExecContext(ctx,
		`UPDATE webhook_endpoints SET enabled=false, updated_at=now()
		WHERE merchant_id=$1 AND enabled=true`, merchantID)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("merchants: RowsAffected: %w", err)
	}
	return int(rows), nil
}

// ---------------- audit (STAND-IN for Member 1's shared audit_logs table) ----------------

// AuditWriter is a temporary stub until Member 1 adds the audit_logs migration.
// Replace this no-op with the real SQL INSERT once audit_logs is available.
type AuditWriter struct{}

func (AuditWriter) Write(_ context.Context, _ application.Querier, _ application.AuditEntry) error {
	// TODO(Member 1): wire to audit_logs table once migration is committed.
	return nil
}

// ---------------- role assigner (STAND-IN for Member 1's user_roles table) ----------------

// RoleAssigner is a temporary stub until Member 1 confirms the user_roles schema.
// Replace with the real SQL INSERT once the table is available.
type RoleAssigner struct{}

func (RoleAssigner) GrantMerchantRole(_ context.Context, _ application.Querier, _, _ string) error {
	// TODO(Member 1): wire to user_roles table once migration is committed.
	return nil
}
