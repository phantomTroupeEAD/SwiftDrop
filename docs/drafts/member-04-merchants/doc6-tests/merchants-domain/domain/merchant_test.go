package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)

// TC-D-01..25: every (status, action) pair — the full state machine (Doc 2 §2.1).
func TestStatusMachine_AllPairs(t *testing.T) {
	want := map[MerchantStatus]map[StatusAction]MerchantStatus{
		StatusPending:   {ActionApprove: StatusApproved, ActionReject: StatusRejected},
		StatusApproved:  {ActionSuspend: StatusSuspended, ActionDisable: StatusDisabled},
		StatusSuspended: {ActionReinstate: StatusApproved, ActionDisable: StatusDisabled},
	}
	for _, from := range AllStatuses {
		for _, action := range AllActions {
			name := string(from) + "_" + string(action)
			t.Run(name, func(t *testing.T) {
				m := Rehydrate("m1", "u1", "Shop", from, 3)
				err := m.Do(action, "admin1", "valid reason text", t0)
				to, allowed := want[from][action]
				if allowed {
					if err != nil {
						t.Fatalf("expected allowed, got %v", err)
					}
					if m.Status() != to {
						t.Fatalf("status = %s, want %s", m.Status(), to)
					}
					return
				}
				var ite InvalidTransitionError
				if !errors.As(err, &ite) {
					t.Fatalf("expected InvalidTransitionError, got %v", err)
				}
				if m.Status() != from {
					t.Fatalf("status changed on failure: %s", m.Status())
				}
				if len(m.PullEvents()) != 0 {
					t.Fatal("event recorded on failed transition")
				}
			})
		}
	}
}

func TestTerminalStatesAllowNothing(t *testing.T) {
	for _, s := range []MerchantStatus{StatusRejected, StatusDisabled} {
		for _, a := range AllActions {
			m := Rehydrate("m1", "u1", "Shop", s, 1)
			if err := m.Do(a, "admin", "valid reason", t0); err == nil {
				t.Errorf("%s %s: expected error", s, a)
			}
		}
	}
}

// BR-M-08: reject/suspend/disable need a 5-500 char reason.
func TestReasonRules(t *testing.T) {
	cases := []struct {
		name   string
		from   MerchantStatus
		action StatusAction
		reason string
		want   error
	}{
		{"reject no reason", StatusPending, ActionReject, "", ErrReasonRequired},
		{"reject whitespace reason", StatusPending, ActionReject, "   ", ErrReasonRequired},
		{"reject 4 chars", StatusPending, ActionReject, "abcd", ErrInvalidReason},
		{"reject 5 chars", StatusPending, ActionReject, "abcde", nil},
		{"reject 500 chars", StatusPending, ActionReject, strings.Repeat("a", 500), nil},
		{"reject 501 chars", StatusPending, ActionReject, strings.Repeat("a", 501), ErrInvalidReason},
		{"suspend no reason", StatusApproved, ActionSuspend, "", ErrReasonRequired},
		{"disable no reason", StatusSuspended, ActionDisable, "", ErrReasonRequired},
		{"approve without reason ok", StatusPending, ActionApprove, "", nil},
		{"reinstate without reason ok", StatusSuspended, ActionReinstate, "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Rehydrate("m1", "u1", "Shop", c.from, 1)
			err := m.Do(c.action, "admin", c.reason, t0)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if c.want != nil && m.Status() != c.from {
				t.Fatal("status changed despite reason error")
			}
		})
	}
}

func TestApproveSetsApprovalFieldsAndEvent(t *testing.T) {
	m, err := NewMerchant("m1", "u1", "  Ceylon Tea Shop  ")
	if err != nil {
		t.Fatal(err)
	}
	if m.Status() != StatusPending || m.Version() != 1 {
		t.Fatalf("new merchant = %s v%d", m.Status(), m.Version())
	}
	if m.CanIssueCredentials() {
		t.Fatal("PENDING must not issue credentials")
	}
	if err := m.Do(ActionApprove, "admin1", "ignored", t0); err != nil {
		t.Fatal(err)
	}
	if m.ApprovedBy() == nil || *m.ApprovedBy() != "admin1" || !m.ApprovedAt().Equal(t0) {
		t.Fatal("approval fields not set")
	}
	if !m.CanIssueCredentials() {
		t.Fatal("APPROVED must issue credentials")
	}
	ev := m.PullEvents()
	if len(ev) != 1 || ev[0].From != StatusPending || ev[0].To != StatusApproved || ev[0].Reason != "" {
		t.Fatalf("events = %+v", ev)
	}
	if len(m.PullEvents()) != 0 {
		t.Fatal("PullEvents must clear")
	}
}

func TestSuspendReinstateKeepsApproval(t *testing.T) {
	m, _ := NewMerchant("m1", "u1", "Shop")
	_ = m.Do(ActionApprove, "a1", "", t0)
	if err := m.Do(ActionSuspend, "a2", "chargeback investigation", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if m.CanIssueCredentials() {
		t.Fatal("SUSPENDED must not issue credentials")
	}
	if err := m.Do(ActionReinstate, "a2", "", t0.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if *m.ApprovedBy() != "a1" {
		t.Fatal("reinstate must not overwrite original approver")
	}
	if len(m.PullEvents()) != 3 {
		t.Fatal("expected 3 events")
	}
}

func TestBusinessNameValidation(t *testing.T) {
	cases := map[string]bool{
		"":                       false,
		"A":                      false,
		"  A  ":                  false,
		"AB":                     true,
		strings.Repeat("x", 150): true,
		strings.Repeat("x", 151): false,
		"ශ්‍රී ලංකා Tea":         true, // Sinhala, counted in runes
	}
	for in, ok := range cases {
		_, err := NewMerchant("m", "u", in)
		if (err == nil) != ok {
			t.Errorf("NewMerchant(%q) err=%v, want ok=%v", in, err, ok)
		}
	}
}
