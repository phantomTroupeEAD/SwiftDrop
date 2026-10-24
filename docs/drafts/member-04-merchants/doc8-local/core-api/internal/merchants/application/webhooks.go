package application

import (
	"context"
	"errors"
	"time"

	"swiftdrop.local/m4/core-api/internal/merchants/domain"
)

const dnsTimeout = 2 * time.Second // BR-W-06

func (s *Service) allowedPrivate(host string) bool {
	for _, h := range s.cfg.AllowedPrivateHosts { // local only, enforced by config
		if h == host {
			return true
		}
	}
	return false
}

// validateURL runs BR-W-02..06. DNS runs here, never inside a DB transaction.
func (s *Service) validateURL(ctx context.Context, raw string) (domain.WebhookURL, error) {
	u, err := domain.ParseWebhookURL(raw, s.cfg.AllowHTTP)
	if err != nil {
		return u, err
	}
	if s.allowedPrivate(u.Host()) {
		return u, nil
	}
	p := domain.WebhookURLPolicy{}
	if err := p.CheckHost(u); err != nil {
		if s.Observer != nil {
			s.Observer.SSRFBlocked()
		}
		return u, err
	}
	dctx, cancel := context.WithTimeout(ctx, dnsTimeout)
	defer cancel()
	addrs, err := s.Resolver.LookupAddrs(dctx, u.Host())
	if err != nil {
		return u, domain.ErrURLUnresolvable
	}
	if err := p.CheckResolved(addrs); err != nil {
		if errors.Is(err, domain.ErrSSRFProhibited) && s.Observer != nil {
			s.Observer.SSRFBlocked()
		}
		return u, err
	}
	return u, nil
}

// RegisterWebhook implements F-09. Returns the signing secret once.
func (s *Service) RegisterWebhook(ctx context.Context, a Actor, rawURL string, events []string) (WebhookView, string, error) {
	merchantID, err := tenant(a)
	if err != nil {
		return WebhookView{}, "", err
	}
	st, err := s.Merchants.Status(ctx, s.Tx.Reader(), merchantID)
	if err != nil {
		return WebhookView{}, "", err
	}
	if st != domain.StatusApproved {
		return WebhookView{}, "", ErrNotApproved
	}
	if len(events) == 0 {
		return WebhookView{}, "", ErrEventUnsupported
	}
	seen := map[string]bool{}
	var evs []string
	for _, e := range events {
		if !inCatalog(e) {
			return WebhookView{}, "", ErrEventUnsupported
		}
		if !seen[e] {
			seen[e] = true
			evs = append(evs, e)
		}
	}
	u, err := s.validateURL(ctx, rawURL)
	if err != nil {
		return WebhookView{}, "", err
	}
	secret, err := s.KeyGen.NewSigningSecret()
	if err != nil {
		return WebhookView{}, "", err
	}
	ct, ver, err := s.Cipher.Encrypt(secret)
	if err != nil {
		return WebhookView{}, "", err
	}
	now := s.Now().UTC()
	rec := WebhookRecord{ID: newUUID(), MerchantID: merchantID, URL: u.String(), Last4: secret[len(secret)-4:],
		Ciphertext: ct, KeyVersion: ver, Events: evs, CreatedBy: a.UserID}
	err = s.Tx.WithinTx(ctx, func(ctx context.Context, q Querier) error {
		m, err := s.Merchants.LockForUpdate(ctx, q, merchantID)
		if err != nil {
			return err
		}
		if m.Status != domain.StatusApproved {
			return ErrNotApproved
		}
		n, err := s.Hooks.CountLive(ctx, q, merchantID)
		if err != nil {
			return err
		}
		if n >= s.cfg.MaxWebhooks {
			return ErrWebhookLimit
		}
		if err := s.Hooks.Insert(ctx, q, rec); err != nil {
			return err
		}
		return s.Audit.Write(ctx, q, AuditEntry{
			ActorID: a.UserID, ActorRole: a.Role, Action: "WEBHOOK_CREATED", TargetID: merchantID, RequestID: a.RequestID,
			New: map[string]any{"webhook_id": rec.ID, "host": u.Host(), "events": evs, "enabled": true},
		})
	})
	if err != nil {
		return WebhookView{}, "", err
	}
	return WebhookView{ID: rec.ID, URL: rec.URL, Last4: rec.Last4, Events: evs, Enabled: true, CreatedAt: now, UpdatedAt: now}, secret, nil
}

func (s *Service) ListWebhooks(ctx context.Context, a Actor) ([]WebhookView, error) {
	merchantID, err := tenant(a)
	if err != nil {
		return nil, err
	}
	return s.Hooks.List(ctx, s.Tx.Reader(), merchantID)
}

// DeleteWebhook is a tenant-scoped soft delete (BR-W-12).
func (s *Service) DeleteWebhook(ctx context.Context, a Actor, id string) error {
	merchantID, err := tenant(a)
	if err != nil {
		return err
	}
	return s.Tx.WithinTx(ctx, func(ctx context.Context, q Querier) error {
		ok, err := s.Hooks.SoftDelete(ctx, q, merchantID, id, s.Now().UTC())
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		return s.Audit.Write(ctx, q, AuditEntry{
			ActorID: a.UserID, ActorRole: a.Role, Action: "WEBHOOK_DELETED", TargetID: merchantID, RequestID: a.RequestID,
			New: map[string]any{"webhook_id": id},
		})
	})
}
