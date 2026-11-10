package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

type MerchantStatus string

const (
	StatusPending   MerchantStatus = "PENDING"
	StatusApproved  MerchantStatus = "APPROVED"
	StatusRejected  MerchantStatus = "REJECTED"
	StatusSuspended MerchantStatus = "SUSPENDED"
	StatusDisabled  MerchantStatus = "DISABLED"
)

var AllStatuses = []MerchantStatus{StatusPending, StatusApproved, StatusRejected, StatusSuspended, StatusDisabled}

type StatusAction string

const (
	ActionApprove   StatusAction = "APPROVE"
	ActionReject    StatusAction = "REJECT"
	ActionSuspend   StatusAction = "SUSPEND"
	ActionReinstate StatusAction = "REINSTATE"
	ActionDisable   StatusAction = "DISABLE"
)

var AllActions = []StatusAction{ActionApprove, ActionReject, ActionSuspend, ActionReinstate, ActionDisable}

// transitions is the merchant state machine (Doc 2 §2.1).
var transitions = map[MerchantStatus]map[StatusAction]MerchantStatus{
	StatusPending:   {ActionApprove: StatusApproved, ActionReject: StatusRejected},
	StatusApproved:  {ActionSuspend: StatusSuspended, ActionDisable: StatusDisabled},
	StatusSuspended: {ActionReinstate: StatusApproved, ActionDisable: StatusDisabled},
}

var reasonRequired = map[StatusAction]bool{ActionReject: true, ActionSuspend: true, ActionDisable: true}

// StatusChanged is an in-process domain event used by the application layer for audit rows.
type StatusChanged struct {
	MerchantID string
	From, To   MerchantStatus
	Action     StatusAction
	Actor      string
	Reason     string
	At         time.Time
}

// Merchant is the aggregate root. Version is incremented by the repository on save.
type Merchant struct {
	id          string
	ownerUserID string
	name        string
	status      MerchantStatus
	approvedBy  *string
	approvedAt  *time.Time
	version     int
	events      []StatusChanged
}

// NewMerchant creates a merchant application (BR-M-01, BR-M-06).
func NewMerchant(id, ownerUserID, businessName string) (*Merchant, error) {
	name := strings.TrimSpace(businessName)
	if n := utf8.RuneCountInString(name); n < 2 || n > 150 {
		return nil, ErrInvalidBusinessName
	}
	return &Merchant{id: id, ownerUserID: ownerUserID, name: name, status: StatusPending, version: 1}, nil
}

// Rehydrate rebuilds a merchant loaded from storage.
func Rehydrate(id, ownerUserID, name string, status MerchantStatus, version int) *Merchant {
	return &Merchant{id: id, ownerUserID: ownerUserID, name: name, status: status, version: version}
}

func (m *Merchant) ID() string             { return m.id }
func (m *Merchant) Status() MerchantStatus { return m.status }
func (m *Merchant) Version() int           { return m.version }
func (m *Merchant) ApprovedBy() *string    { return m.approvedBy }
func (m *Merchant) ApprovedAt() *time.Time { return m.approvedAt }

// CanIssueCredentials reports whether keys/webhooks/orders are allowed (Doc 2 §2.1 table).
func (m *Merchant) CanIssueCredentials() bool { return m.status == StatusApproved }

// Do applies a status action. On error the merchant is unchanged and no event is recorded.
func (m *Merchant) Do(action StatusAction, actor, reason string, now time.Time) error {
	to, ok := transitions[m.status][action]
	if !ok {
		return InvalidTransitionError{From: m.status, Action: action}
	}
	reason = strings.TrimSpace(reason)
	if reasonRequired[action] {
		if reason == "" {
			return ErrReasonRequired
		}
		if n := utf8.RuneCountInString(reason); n < 5 || n > 500 {
			return ErrInvalidReason
		}
	} else {
		reason = ""
	}
	from := m.status
	m.status = to
	if action == ActionApprove {
		a, t := actor, now
		m.approvedBy, m.approvedAt = &a, &t
	}
	m.events = append(m.events, StatusChanged{
		MerchantID: m.id, From: from, To: to, Action: action, Actor: actor, Reason: reason, At: now,
	})
	return nil
}

// PullEvents returns and clears recorded events.
func (m *Merchant) PullEvents() []StatusChanged {
	e := m.events
	m.events = nil
	return e
}
