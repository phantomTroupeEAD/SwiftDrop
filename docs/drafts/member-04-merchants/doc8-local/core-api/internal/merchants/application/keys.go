package application

import (
	"context"
	"errors"
	"time"

	"swiftdrop.local/m4/core-api/internal/merchants/domain"
)

// CreateAPIKey implements F-04. Returns the plaintext key once.
func (s *Service) CreateAPIKey(ctx context.Context, a Actor, label string, scopes []string, expiresAt *time.Time) (CredentialView, domain.PlainAPIKey, error) {
	merchantID, err := tenant(a)
	if err != nil {
		return CredentialView{}, domain.PlainAPIKey{}, err
	}
	l, err := domain.ValidateLabel(label)
	if err != nil || domain.HasControlChars(l) {
		return CredentialView{}, domain.PlainAPIKey{}, ValidationError{"label", "must be 1-60 characters, no control characters"}
	}
	ss, err := domain.NewScopeSet(scopes)
	if err != nil {
		return CredentialView{}, domain.PlainAPIKey{}, ValidationError{"scopes", "allowed: parcels:create, parcels:read"}
	}
	now := s.Now().UTC()
	if err := domain.ValidateExpiry(now, expiresAt); err != nil {
		return CredentialView{}, domain.PlainAPIKey{}, ValidationError{"expires_at", "must be 1-365 days in the future"}
	}
	scopeStrs := make([]string, len(ss))
	for i, sc := range ss {
		scopeStrs[i] = string(sc)
	}

	for attempt := 0; attempt < 3; attempt++ { // BR-K-02: regenerate on prefix collision
		key, err := s.KeyGen.NewAPIKey(s.cfg.KeyEnvPrefix) // generated + hashed before the tx
		if err != nil {
			return CredentialView{}, domain.PlainAPIKey{}, err
		}
		ver := s.Hasher.CurrentVersion()
		hash, err := s.Hasher.Hash(ver, key.Reveal())
		if err != nil {
			return CredentialView{}, domain.PlainAPIKey{}, err
		}
		rec := CredentialRecord{
			ID: newUUID(), MerchantID: merchantID, Prefix: string(key.Prefix()), Hash: hash,
			PepperVersion: ver, Label: l, Scopes: scopeStrs, CreatedBy: a.UserID, ExpiresAt: expiresAt,
		}
		err = s.Tx.WithinTx(ctx, func(ctx context.Context, q Querier) error {
			m, err := s.Merchants.LockForUpdate(ctx, q, merchantID)
			if err != nil {
				return err
			}
			if m.Status != domain.StatusApproved {
				return ErrNotApproved
			}
			n, err := s.Creds.CountActive(ctx, q, merchantID, now)
			if err != nil {
				return err
			}
			if n >= s.cfg.MaxActiveKeys {
				return ErrKeyLimit
			}
			if err := s.Creds.Insert(ctx, q, rec); err != nil {
				return err
			}
			return s.Audit.Write(ctx, q, AuditEntry{
				ActorID: a.UserID, ActorRole: a.Role, Action: "API_KEY_CREATED", TargetID: merchantID, RequestID: a.RequestID,
				New: map[string]any{"key_id": rec.ID, "key_prefix": rec.Prefix, "label": l, "scopes": scopeStrs},
			})
		})
		if errors.Is(err, ErrPrefixCollision) {
			continue
		}
		if err != nil {
			return CredentialView{}, domain.PlainAPIKey{}, err
		}
		return CredentialView{ID: rec.ID, Label: l, Prefix: rec.Prefix, Scopes: scopeStrs, CreatedAt: now, ExpiresAt: expiresAt}, key, nil
	}
	return CredentialView{}, domain.PlainAPIKey{}, ErrPrefixCollision
}

func (s *Service) ListAPIKeys(ctx context.Context, a Actor) ([]CredentialView, error) {
	merchantID, err := tenant(a)
	if err != nil {
		return nil, err
	}
	return s.Creds.List(ctx, s.Tx.Reader(), merchantID)
}

// RevokeAPIKey implements F-05; idempotent (BR-K-11).
func (s *Service) RevokeAPIKey(ctx context.Context, a Actor, keyID string) error {
	merchantID, err := tenant(a)
	if err != nil {
		return err
	}
	return s.Tx.WithinTx(ctx, func(ctx context.Context, q Querier) error {
		changed, exists, err := s.Creds.Revoke(ctx, q, merchantID, keyID, s.Now().UTC())
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		if !changed {
			return nil
		}
		return s.Audit.Write(ctx, q, AuditEntry{
			ActorID: a.UserID, ActorRole: a.Role, Action: "API_KEY_REVOKED", TargetID: merchantID, RequestID: a.RequestID,
			New: map[string]any{"key_id": keyID},
		})
	})
}

// VerifyAPIKey implements F-06. One indexed query, no cache (DD-08).
func (s *Service) VerifyAPIKey(ctx context.Context, raw string) (MerchantPrincipal, error) {
	res := "invalid"
	defer func() {
		if s.Observer != nil {
			s.Observer.AuthResult(res)
		}
	}()
	key, err := domain.ParseAPIKey(raw, s.cfg.KeyEnvPrefix)
	if err != nil {
		return MerchantPrincipal{}, domain.ErrAPIKeyInvalid
	}
	rec, err := s.Creds.FindByPrefix(ctx, s.Tx.Reader(), string(key.Prefix()))
	if errors.Is(err, ErrNotFound) {
		return MerchantPrincipal{}, domain.ErrAPIKeyInvalid
	}
	if err != nil {
		res = "error"
		return MerchantPrincipal{}, err
	}
	got, err := s.Hasher.Hash(rec.PepperVersion, key.Reveal())
	if err != nil || !s.Hasher.Equal(got, rec.Hash) {
		return MerchantPrincipal{}, domain.ErrAPIKeyInvalid
	}
	now := s.Now()
	switch {
	case rec.RevokedAt != nil:
		res = "revoked"
		return MerchantPrincipal{}, ErrAPIKeyRevoked
	case rec.ExpiresAt != nil && !now.Before(*rec.ExpiresAt):
		res = "expired"
		return MerchantPrincipal{}, ErrAPIKeyExpired
	case rec.MerchantStatus != domain.StatusApproved:
		res = "merchant_not_approved"
		return MerchantPrincipal{}, ErrNotApproved
	}
	res = "ok"
	ss, _ := domain.NewScopeSet(rec.Scopes)
	s.touch(rec.CredentialID, now)
	return MerchantPrincipal{MerchantID: rec.MerchantID, CredentialID: rec.CredentialID, KeyPrefix: string(key.Prefix()), Scopes: ss}, nil
}

// touch updates last_used_at at most once per minute per key, off the request path (BR-K-14).
func (s *Service) touch(id string, now time.Time) {
	s.touchMu.Lock()
	if t, ok := s.lastTouch[id]; ok && now.Sub(t) < time.Minute {
		s.touchMu.Unlock()
		return
	}
	s.lastTouch[id] = now
	s.touchMu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.Creds.TouchLastUsed(ctx, s.Tx.Reader(), id, now.UTC())
	}()
}
