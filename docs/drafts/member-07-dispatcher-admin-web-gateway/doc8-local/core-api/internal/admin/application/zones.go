package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"swiftdrop.local/m7/core-api/internal/admin/domain"
	"swiftdrop.local/m7/core-api/internal/platform/apperr"
)

type Service struct {
	Store Store
	NewID func() string
}

type ZoneView struct {
	ID                 string   `json:"id"`
	Code               string   `json:"code"`
	Name               string   `json:"name"`
	BaseSurchargeCents int64    `json:"base_surcharge_cents"`
	Currency           string   `json:"currency"`
	PostalPrefixes     []string `json:"postal_prefixes"`
	Enabled            bool     `json:"enabled"`
	Version            int      `json:"version"`
}

func View(z *domain.Zone) ZoneView {
	p := make([]string, 0, len(z.Prefixes()))
	for _, x := range z.Prefixes() {
		p = append(p, x.String())
	}
	return ZoneView{ID: z.ID(), Code: z.Code().String(), Name: z.Name(), BaseSurchargeCents: z.Surcharge().Cents(),
		Currency: z.Surcharge().Currency(), PostalPrefixes: p, Enabled: z.Enabled(), Version: z.Version()}
}

type CreateZoneInput struct {
	Code               string   `json:"code"`
	Name               string   `json:"name"`
	BaseSurchargeCents int64    `json:"base_surcharge_cents"`
	PostalPrefixes     []string `json:"postal_prefixes"`
}

type UpdateZoneInput struct {
	Version            int      `json:"version"`
	Name               *string  `json:"name,omitempty"`
	BaseSurchargeCents *int64   `json:"base_surcharge_cents,omitempty"`
	PostalPrefixes     []string `json:"postal_prefixes,omitempty"`
	Reason             string   `json:"reason,omitempty"`
}

type StatusInput struct {
	Version int    `json:"version"`
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}

// MaxSurchargeCents is LKR 100 000 (Doc 7 section 6.1).
const MaxSurchargeCents = 10_000_000

func domainErr(err error) error {
	for _, e := range []error{domain.ErrInvalidZoneCode, domain.ErrInvalidZoneName, domain.ErrInvalidPrefix, domain.ErrNegativeMoney,
		domain.ErrNoPrefixes, domain.ErrDuplicatePrefixInZone, domain.ErrAlreadyEnabled, domain.ErrAlreadyDisabled} {
		if errors.Is(err, e) {
			return fmt.Errorf("%w: %v", apperr.ErrValidation, err)
		}
	}
	return err
}

func prefixes(in []string) ([]domain.PostalPrefix, error) {
	if len(in) > 200 {
		return nil, fmt.Errorf("%w: at most 200 prefixes", apperr.ErrValidation)
	}
	out := make([]domain.PostalPrefix, 0, len(in))
	for _, s := range in {
		p, err := domain.NewPostalPrefix(strings.TrimSpace(s))
		if err != nil {
			return nil, domainErr(err)
		}
		out = append(out, p)
	}
	return out, nil
}

func money(c int64) (domain.Money, error) {
	if c > MaxSurchargeCents {
		return domain.Money{}, fmt.Errorf("%w: surcharge above LKR 100,000", apperr.ErrValidation)
	}
	m, err := domain.NewLKR(c)
	return m, domainErr(err)
}

// checkOverlap must run inside the transaction, after LockZonePrefixes.
func checkOverlap(ctx context.Context, tx Tx, z *domain.Zone) error {
	others, err := tx.OtherZonePrefixes(ctx, z.ID())
	if err != nil {
		return err
	}
	if c := (domain.PrefixOverlapPolicy{}).Clashes(z.Code().String(), z.Prefixes(), others); len(c) > 0 {
		return fmt.Errorf("%w: %s overlaps %s in %s", apperr.ErrPrefixConflict, c[0].New, c[0].Existing.Prefix, c[0].Existing.ZoneCode)
	}
	return nil
}

func (s *Service) CreateZone(ctx context.Context, a Actor, in CreateZoneInput) (ZoneView, error) {
	code, err := domain.NewZoneCode(strings.TrimSpace(in.Code))
	if err != nil {
		return ZoneView{}, domainErr(err)
	}
	m, err := money(in.BaseSurchargeCents)
	if err != nil {
		return ZoneView{}, err
	}
	p, err := prefixes(in.PostalPrefixes)
	if err != nil {
		return ZoneView{}, err
	}
	z, err := domain.NewZone(s.NewID(), code, in.Name, m, p)
	if err != nil {
		return ZoneView{}, domainErr(err)
	}
	z.PullEvents()
	err = s.Store.InTx(ctx, func(tx Tx) error {
		if err := tx.LockZonePrefixes(ctx); err != nil {
			return err
		}
		if err := checkOverlap(ctx, tx, z); err != nil {
			return err
		}
		if err := tx.InsertZone(ctx, z, a.ID); err != nil {
			return err
		}
		return tx.Audit(ctx, a, AuditEntry{Action: "ZONE_CREATED", TargetType: "ZONE", TargetID: z.ID(), After: toMap(View(z))})
	})
	if err != nil {
		return ZoneView{}, err
	}
	return View(z), nil
}

func (s *Service) UpdateZone(ctx context.Context, a Actor, id string, in UpdateZoneInput) (ZoneView, error) {
	if in.Version < 1 {
		return ZoneView{}, fmt.Errorf("%w: version is required", apperr.ErrValidation)
	}
	if in.BaseSurchargeCents != nil {
		if err := ValidateReason(in.Reason); err != nil {
			return ZoneView{}, err // money changes always need a reason
		}
	}
	var out ZoneView
	err := s.Store.InTx(ctx, func(tx Tx) error {
		if in.PostalPrefixes != nil {
			if err := tx.LockZonePrefixes(ctx); err != nil {
				return err
			}
		}
		z, err := tx.LoadZone(ctx, id)
		if err != nil {
			return err
		}
		if z.Version() != in.Version {
			return apperr.ErrConcurrentConflict
		}
		before := toMap(View(z))
		if in.Name != nil {
			if err := z.Rename(strings.TrimSpace(*in.Name)); err != nil {
				return domainErr(err)
			}
		}
		if in.BaseSurchargeCents != nil {
			m, err := money(*in.BaseSurchargeCents)
			if err != nil {
				return err
			}
			if err := z.ChangeSurcharge(m); err != nil {
				return domainErr(err)
			}
		}
		if in.PostalPrefixes != nil {
			p, err := prefixes(in.PostalPrefixes)
			if err != nil {
				return err
			}
			if err := z.ReplacePrefixes(p); err != nil {
				return domainErr(err)
			}
			if err := checkOverlap(ctx, tx, z); err != nil {
				return err
			}
		}
		if len(z.PullEvents()) == 0 {
			out = View(z)
			return nil // nothing changed: no write, no audit row
		}
		if err := tx.SaveZone(ctx, z, in.Version, a.ID); err != nil {
			return err
		}
		out = View(z)
		return tx.Audit(ctx, a, AuditEntry{Action: "ZONE_UPDATED", TargetType: "ZONE", TargetID: id, Before: before, After: toMap(out), Reason: in.Reason})
	})
	return out, err
}

func (s *Service) SetZoneEnabled(ctx context.Context, a Actor, id string, in StatusInput) (ZoneView, error) {
	if err := ValidateReason(in.Reason); err != nil {
		return ZoneView{}, err
	}
	var out ZoneView
	err := s.Store.InTx(ctx, func(tx Tx) error {
		z, err := tx.LoadZone(ctx, id)
		if err != nil {
			return err
		}
		if z.Version() != in.Version {
			return apperr.ErrConcurrentConflict
		}
		action := "ZONE_ENABLED"
		if in.Enabled {
			err = z.Enable()
		} else {
			action, err = "ZONE_DISABLED", z.Disable()
		}
		if err != nil {
			return domainErr(err)
		}
		if err := tx.SaveZone(ctx, z, in.Version, a.ID); err != nil {
			return err
		}
		out = View(z)
		return tx.Audit(ctx, a, AuditEntry{Action: action, TargetType: "ZONE", TargetID: id,
			Before: map[string]any{"enabled": !in.Enabled}, After: map[string]any{"enabled": in.Enabled}, Reason: in.Reason})
	})
	return out, err
}

type SettingInput struct {
	Version int             `json:"version"`
	Value   json.RawMessage `json:"value"`
	Reason  string          `json:"reason"`
}

func (s *Service) UpdateSetting(ctx context.Context, a Actor, key string, in SettingInput) (Setting, error) {
	if err := ValidateSetting(key, in.Value); err != nil {
		return Setting{}, err
	}
	if err := ValidateReason(in.Reason); err != nil {
		return Setting{}, err
	}
	var out Setting
	err := s.Store.InTx(ctx, func(tx Tx) error {
		cur, err := tx.LoadSetting(ctx, key)
		if err != nil {
			return err
		}
		if cur.Version != in.Version {
			return apperr.ErrConcurrentConflict
		}
		if err := tx.SaveSetting(ctx, key, in.Value, in.Version, a.ID); err != nil {
			return err
		}
		out = cur
		out.Value, out.Version = in.Value, cur.Version+1
		return tx.Audit(ctx, a, AuditEntry{Action: "SETTING_CHANGED", TargetType: "SETTING", TargetID: key,
			Before: map[string]any{"value": cur.Value}, After: map[string]any{"value": in.Value}, Reason: in.Reason})
	})
	return out, err
}

// toMap turns a view into a map for the audit snapshot. Infrastructure redacts
// secrets before the row is written.
func toMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}
