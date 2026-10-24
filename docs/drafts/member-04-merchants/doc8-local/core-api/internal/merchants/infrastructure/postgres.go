// Package infrastructure implements the merchants ports with PostgreSQL (pgx), crypto and DNS.
package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"swiftdrop.local/m4/core-api/internal/merchants/application"
	"swiftdrop.local/m4/core-api/internal/merchants/domain"
	"swiftdrop.local/m4/core-api/internal/platform/db"
	"swiftdrop.local/m4/core-api/internal/platform/web"
)

// mapErr turns PostgreSQL constraint errors into application errors (FM-03).
func mapErr(err error, notFound error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	code, c := db.Violation(err)
	if code == "23505" {
		switch c {
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

type MerchantRepo struct{}

const merchantCols = `id, owner_user_id, business_name, status, coalesce(status_reason,''), contact_email,
	contact_phone, address, approved_at, version, created_at, updated_at`

func scanMerchant(row pgx.Row) (application.MerchantView, error) {
	var v application.MerchantView
	var addr []byte
	var st string
	err := row.Scan(&v.ID, &v.OwnerUserID, &v.BusinessName, &st, &v.StatusReason, &v.ContactEmail,
		&v.ContactPhone, &addr, &v.ApprovedAt, &v.Version, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return v, err
	}
	v.Status = domain.MerchantStatus(st)
	_ = json.Unmarshal(addr, &v.Address)
	return v, nil
}

func (MerchantRepo) Insert(ctx context.Context, q application.Querier, v application.MerchantView) error {
	addr, _ := json.Marshal(v.Address)
	_, err := db.Q(q).Exec(ctx, `INSERT INTO merchants (id, owner_user_id, business_name, status, contact_email,
		contact_phone, address, version, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,1,$8,$8)`,
		v.ID, v.OwnerUserID, v.BusinessName, string(v.Status), v.ContactEmail, v.ContactPhone, addr, v.CreatedAt)
	return mapErr(err, application.ErrMerchantNotFound)
}

func (MerchantRepo) Get(ctx context.Context, q application.Querier, id string) (application.MerchantView, error) {
	v, err := scanMerchant(db.Q(q).QueryRow(ctx, `SELECT `+merchantCols+` FROM merchants WHERE id=$1`, id))
	return v, mapErr(err, application.ErrMerchantNotFound)
}

func (MerchantRepo) FindByOwner(ctx context.Context, q application.Querier, owner string) (application.MerchantView, error) {
	v, err := scanMerchant(db.Q(q).QueryRow(ctx,
		`SELECT `+merchantCols+` FROM merchants WHERE owner_user_id=$1 ORDER BY created_at DESC LIMIT 1`, owner))
	return v, mapErr(err, application.ErrMerchantNotFound)
}

func (MerchantRepo) LockForUpdate(ctx context.Context, q application.Querier, id string) (application.MerchantView, error) {
	v, err := scanMerchant(db.Q(q).QueryRow(ctx, `SELECT `+merchantCols+` FROM merchants WHERE id=$1 FOR UPDATE`, id))
	return v, mapErr(err, application.ErrMerchantNotFound)
}

func (MerchantRepo) Status(ctx context.Context, q application.Querier, id string) (domain.MerchantStatus, error) {
	var s string
	err := db.Q(q).QueryRow(ctx, `SELECT status FROM merchants WHERE id=$1`, id).Scan(&s)
	return domain.MerchantStatus(s), mapErr(err, application.ErrMerchantNotFound)
}

// SaveStatus applies optimistic locking (BR-M-09).
func (MerchantRepo) SaveStatus(ctx context.Context, q application.Querier, m *domain.Merchant, expected int, reason string) error {
	tag, err := db.Q(q).Exec(ctx, `UPDATE merchants SET status=$1, status_reason=NULLIF($2,''),
		approved_by=COALESCE($3, approved_by), approved_at=COALESCE($4, approved_at),
		version=version+1, updated_at=now() WHERE id=$5 AND version=$6`,
		string(m.Status()), reason, m.ApprovedBy(), m.ApprovedAt(), m.ID(), expected)
	if err != nil {
		return mapErr(err, application.ErrMerchantNotFound)
	}
	if tag.RowsAffected() == 0 {
		return application.ErrVersionConflict
	}
	return nil
}

func (MerchantRepo) List(ctx context.Context, q application.Querier, status string, limit, offset int) ([]application.MerchantView, int, error) {
	var total int
	if err := db.Q(q).QueryRow(ctx, `SELECT count(*) FROM merchants WHERE ($1='' OR status=$1)`, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.Q(q).Query(ctx, `SELECT `+merchantCols+` FROM merchants WHERE ($1='' OR status=$1)
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []application.MerchantView
	for rows.Next() {
		v, err := scanMerchant(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

// ---------------- credentials ----------------

type CredentialRepo struct{}

func (CredentialRepo) Insert(ctx context.Context, q application.Querier, r application.CredentialRecord) error {
	_, err := db.Q(q).Exec(ctx, `INSERT INTO merchant_api_credentials
		(id, merchant_id, key_prefix, key_hash, pepper_version, label, scopes, created_by, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		r.ID, r.MerchantID, r.Prefix, r.Hash, r.PepperVersion, r.Label, r.Scopes, r.CreatedBy, r.ExpiresAt)
	return mapErr(err, application.ErrNotFound)
}

func (CredentialRepo) FindByPrefix(ctx context.Context, q application.Querier, prefix string) (application.AuthRecord, error) {
	var r application.AuthRecord
	var st string
	err := db.Q(q).QueryRow(ctx, `SELECT c.id, c.merchant_id, c.key_hash, c.pepper_version, c.scopes,
		c.expires_at, c.revoked_at, m.status
		FROM merchant_api_credentials c JOIN merchants m ON m.id = c.merchant_id WHERE c.key_prefix=$1`, prefix).
		Scan(&r.CredentialID, &r.MerchantID, &r.Hash, &r.PepperVersion, &r.Scopes, &r.ExpiresAt, &r.RevokedAt, &st)
	r.MerchantStatus = domain.MerchantStatus(st)
	return r, mapErr(err, application.ErrNotFound)
}

func (CredentialRepo) List(ctx context.Context, q application.Querier, merchantID string) ([]application.CredentialView, error) {
	rows, err := db.Q(q).Query(ctx, `SELECT id, label, key_prefix, scopes, created_at, expires_at, revoked_at, last_used_at
		FROM merchant_api_credentials WHERE merchant_id=$1 ORDER BY created_at DESC`, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []application.CredentialView{}
	for rows.Next() {
		var v application.CredentialView
		if err := rows.Scan(&v.ID, &v.Label, &v.Prefix, &v.Scopes, &v.CreatedAt, &v.ExpiresAt, &v.RevokedAt, &v.LastUsedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (CredentialRepo) CountActive(ctx context.Context, q application.Querier, merchantID string, now time.Time) (int, error) {
	var n int
	err := db.Q(q).QueryRow(ctx, `SELECT count(*) FROM merchant_api_credentials WHERE merchant_id=$1
		AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > $2)`, merchantID, now).Scan(&n)
	return n, err
}

func (CredentialRepo) Revoke(ctx context.Context, q application.Querier, merchantID, id string, now time.Time) (bool, bool, error) {
	tag, err := db.Q(q).Exec(ctx, `UPDATE merchant_api_credentials SET revoked_at=$3
		WHERE id=$1 AND merchant_id=$2 AND revoked_at IS NULL`, id, merchantID, now)
	if err != nil {
		return false, false, err
	}
	if tag.RowsAffected() == 1 {
		return true, true, nil
	}
	var exists bool
	err = db.Q(q).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM merchant_api_credentials WHERE id=$1 AND merchant_id=$2)`,
		id, merchantID).Scan(&exists)
	return false, exists, err
}

func (CredentialRepo) RevokeAll(ctx context.Context, q application.Querier, merchantID string, now time.Time) (int, error) {
	tag, err := db.Q(q).Exec(ctx, `UPDATE merchant_api_credentials SET revoked_at=$2
		WHERE merchant_id=$1 AND revoked_at IS NULL`, merchantID, now)
	return int(tag.RowsAffected()), err
}

func (CredentialRepo) TouchLastUsed(ctx context.Context, q application.Querier, id string, now time.Time) error {
	_, err := db.Q(q).Exec(ctx, `UPDATE merchant_api_credentials SET last_used_at=$2 WHERE id=$1`, id, now)
	return err
}

// ---------------- webhooks ----------------

type WebhookRepo struct{}

func (WebhookRepo) Insert(ctx context.Context, q application.Querier, r application.WebhookRecord) error {
	_, err := db.Q(q).Exec(ctx, `INSERT INTO webhook_endpoints (id, merchant_id, url, secret_ciphertext,
		secret_key_version, secret_last4, subscribed_event_types, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		r.ID, r.MerchantID, r.URL, r.Ciphertext, r.KeyVersion, r.Last4, r.Events, r.CreatedBy)
	return mapErr(err, application.ErrNotFound)
}

func (WebhookRepo) List(ctx context.Context, q application.Querier, merchantID string) ([]application.WebhookView, error) {
	rows, err := db.Q(q).Query(ctx, `SELECT id, url, secret_last4, subscribed_event_types, enabled, created_at, updated_at
		FROM webhook_endpoints WHERE merchant_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC`, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []application.WebhookView{}
	for rows.Next() {
		var v application.WebhookView
		if err := rows.Scan(&v.ID, &v.URL, &v.Last4, &v.Events, &v.Enabled, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (WebhookRepo) CountLive(ctx context.Context, q application.Querier, merchantID string) (int, error) {
	var n int
	err := db.Q(q).QueryRow(ctx, `SELECT count(*) FROM webhook_endpoints WHERE merchant_id=$1 AND deleted_at IS NULL`,
		merchantID).Scan(&n)
	return n, err
}

func (WebhookRepo) SoftDelete(ctx context.Context, q application.Querier, merchantID, id string, now time.Time) (bool, error) {
	tag, err := db.Q(q).Exec(ctx, `UPDATE webhook_endpoints SET deleted_at=$3, enabled=false, updated_at=$3
		WHERE id=$1 AND merchant_id=$2 AND deleted_at IS NULL`, id, merchantID, now)
	return tag.RowsAffected() == 1, err
}

func (WebhookRepo) DisableAll(ctx context.Context, q application.Querier, merchantID string) (int, error) {
	tag, err := db.Q(q).Exec(ctx, `UPDATE webhook_endpoints SET enabled=false, updated_at=now()
		WHERE merchant_id=$1 AND enabled`, merchantID)
	return int(tag.RowsAffected()), err
}

// ---------------- audit (STAND-IN for Member 1's shared writer) ----------------

type AuditWriter struct{}

func (AuditWriter) Write(ctx context.Context, q application.Querier, e application.AuditEntry) error {
	prev, _ := json.Marshal(e.Previous)
	next, _ := json.Marshal(e.New)
	meta := web.Meta(ctx)
	_, err := db.Q(q).Exec(ctx, `INSERT INTO audit_logs (actor_id, actor_role, action, target_type, target_id,
		previous_state, new_state, client_ip, user_agent, request_id)
		VALUES ($1,$2,$3,'MERCHANT',$4,NULLIF($5,'null')::jsonb,NULLIF($6,'null')::jsonb,NULLIF($7,'')::inet,$8,$9)`,
		e.ActorID, e.ActorRole, e.Action, e.TargetID, string(prev), string(next), meta.ClientIP, meta.UserAgent,
		web.RequestID(ctx))
	return err
}

// RoleAssigner is a STAND-IN for Member 1 (U-15): records the request in user_roles_stub.
type RoleAssigner struct{}

func (RoleAssigner) GrantMerchantRole(ctx context.Context, q application.Querier, userID, merchantID string) error {
	_, err := db.Q(q).Exec(ctx, `INSERT INTO user_roles_stub (user_id, role, tenant_id) VALUES ($1,'MERCHANT',$2)
		ON CONFLICT (user_id, role) DO UPDATE SET tenant_id = EXCLUDED.tenant_id`, userID, merchantID)
	return err
}
