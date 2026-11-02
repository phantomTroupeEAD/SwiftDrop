package merchants

import (
	"context"
	"errors"
	"testing"
)

const pending = "11111111-1111-4111-8111-111111111111"

var admin = Actor{UserID: "a", Role: "ADMIN"}

func TestDevFakeFollowsMember4Rules(t *testing.T) {
	f := NewDevFake()
	ctx := context.Background()
	if _, err := f.ChangeStatus(ctx, ChangeStatusCommand{MerchantID: pending, Action: "REJECT", ExpectedVersion: 1, Actor: admin}); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("reject without reason: %v", err)
	}
	if _, err := f.ChangeStatus(ctx, ChangeStatusCommand{MerchantID: pending, Action: "SUSPEND", ExpectedVersion: 1, Reason: "fraud check", Actor: admin}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("suspend a PENDING merchant: %v", err)
	}
	m, err := f.ChangeStatus(ctx, ChangeStatusCommand{MerchantID: pending, Action: "APPROVE", ExpectedVersion: 1, Actor: admin})
	if err != nil || m.Status != "APPROVED" || m.Version != 2 || m.ApprovedAt == nil {
		t.Fatalf("approve: %+v %v", m, err)
	}
	if _, err := f.ChangeStatus(ctx, ChangeStatusCommand{MerchantID: pending, Action: "SUSPEND", ExpectedVersion: 1, Reason: "late payments", Actor: admin}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale version: %v", err)
	}
	for _, step := range []struct{ action, reason, want string }{{"SUSPEND", "late payments", "SUSPENDED"}, {"REINSTATE", "", "APPROVED"}, {"DISABLE", "closed business", "DISABLED"}} {
		m, err = f.ChangeStatus(ctx, ChangeStatusCommand{MerchantID: pending, Action: step.action, ExpectedVersion: m.Version, Reason: step.reason, Actor: admin})
		if err != nil || m.Status != step.want {
			t.Fatalf("%s: %+v %v", step.action, m, err)
		}
	}
	if _, err := f.ChangeStatus(ctx, ChangeStatusCommand{MerchantID: pending, Action: "APPROVE", ExpectedVersion: m.Version, Actor: Actor{Role: "DISPATCHER"}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("non-admin actor must be refused by Member 4 too: %v", err)
	}
	if _, err := f.GetMerchant(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id: %v", err)
	}
}

func TestDevFakeListFiltersAndPages(t *testing.T) {
	f := NewDevFake()
	st := "APPROVED"
	p, _ := f.ListMerchants(context.Background(), ListFilter{Status: &st}, Page{1, 20})
	if p.Total != 1 || p.Items[0].BusinessName != "Kandy Crafts" {
		t.Fatalf("status filter: %+v", p)
	}
	p, _ = f.ListMerchants(context.Background(), ListFilter{Query: "CEYLON"}, Page{1, 20})
	if p.Total != 1 {
		t.Fatalf("search: %+v", p)
	}
	p, _ = f.ListMerchants(context.Background(), ListFilter{}, Page{2, 1})
	if p.Total != 2 || len(p.Items) != 1 || p.Items[0].BusinessName != "Kandy Crafts" {
		t.Fatalf("page 2: %+v", p)
	}
	p, _ = f.ListMerchants(context.Background(), ListFilter{}, Page{9, 10})
	if len(p.Items) != 0 {
		t.Fatalf("page past the end: %+v", p)
	}
}
