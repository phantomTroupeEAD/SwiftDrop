package merchants

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// DevFake is an in-memory AdminService for the local demo only, with Member 4's
// status rules (Member 4 Doc 2). The real module also writes the audit row,
// revokes keys on DISABLE and runs everything in one database transaction.
type DevFake struct {
	mu   sync.Mutex
	rows map[string]*MerchantSummary
	now  func() time.Time
}

func NewDevFake() *DevFake {
	t := time.Date(2026, 10, 20, 4, 30, 0, 0, time.UTC)
	f := &DevFake{rows: map[string]*MerchantSummary{}, now: time.Now}
	for _, m := range []MerchantSummary{
		{MerchantID: "11111111-1111-4111-8111-111111111111", BusinessName: "Ceylon Tea Shop", Status: "PENDING", ContactEmail: "owner@ceylontea.example", ContactPhone: "+94771234567"},
		{MerchantID: "22222222-2222-4222-8222-222222222222", BusinessName: "Kandy Crafts", Status: "APPROVED", ContactEmail: "hello@kandycrafts.example", ContactPhone: "+94812345678", ApprovedAt: &t},
	} {
		m := m
		m.Version, m.CreatedAt, m.UpdatedAt = 1, t, t
		f.rows[m.MerchantID] = &m
	}
	return f
}

// transitions: action -> allowed from-statuses -> new status, and whether a reason is required.
var transitions = map[string]struct {
	from   []string
	to     string
	reason bool
}{
	"APPROVE":   {[]string{"PENDING"}, "APPROVED", false},
	"REJECT":    {[]string{"PENDING"}, "REJECTED", true},
	"SUSPEND":   {[]string{"APPROVED"}, "SUSPENDED", true},
	"REINSTATE": {[]string{"SUSPENDED"}, "APPROVED", false},
	"DISABLE":   {[]string{"APPROVED", "SUSPENDED"}, "DISABLED", true},
}

func (f *DevFake) ListMerchants(_ context.Context, fl ListFilter, p Page) (MerchantPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	q := strings.ToLower(fl.Query)
	var all []MerchantSummary
	for _, m := range f.rows {
		if fl.Status != nil && m.Status != *fl.Status {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(m.BusinessName), q) && !strings.Contains(strings.ToLower(m.ContactEmail), q) {
			continue
		}
		all = append(all, *m)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].BusinessName < all[j].BusinessName })
	start := (p.Number - 1) * p.Size
	if start > len(all) {
		start = len(all)
	}
	end := start + p.Size
	if end > len(all) {
		end = len(all)
	}
	return MerchantPage{Items: append([]MerchantSummary{}, all[start:end]...), Total: len(all)}, nil
}

func (f *DevFake) GetMerchant(_ context.Context, id string) (MerchantSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.rows[id]
	if !ok {
		return MerchantSummary{}, ErrNotFound
	}
	return *m, nil
}

func (f *DevFake) ChangeStatus(_ context.Context, c ChangeStatusCommand) (MerchantSummary, error) {
	t, ok := transitions[c.Action]
	if !ok || c.Actor.Role != "ADMIN" {
		return MerchantSummary{}, ErrValidation
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(c.Reason)); (t.reason || c.Reason != "") && (n < 5 || n > 500) {
		return MerchantSummary{}, ErrReasonRequired
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.rows[c.MerchantID]
	if !ok {
		return MerchantSummary{}, ErrNotFound
	}
	if m.Version != c.ExpectedVersion {
		return MerchantSummary{}, ErrVersionConflict
	}
	allowed := false
	for _, s := range t.from {
		allowed = allowed || s == m.Status
	}
	if !allowed {
		return MerchantSummary{}, ErrInvalidTransition
	}
	now := f.now().UTC()
	m.Status, m.Version, m.UpdatedAt = t.to, m.Version+1, now
	if t.to == "APPROVED" && m.ApprovedAt == nil {
		m.ApprovedAt = &now
	}
	return *m, nil
}
