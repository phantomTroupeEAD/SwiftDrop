// Package merchants is a STAND-IN for Member 4's merchants facade. Only the
// AdminService interface below is the contract (Member 4 Doc 4 section 4); in
// the real repo Member 4's module implements it and this folder holds their code.
// The admin module hosts the /api/v1/admin/merchants/** routes and calls it.
package merchants

import (
	"context"
	"errors"
	"time"
)

type AdminService interface {
	ListMerchants(ctx context.Context, f ListFilter, p Page) (MerchantPage, error)
	GetMerchant(ctx context.Context, id string) (MerchantSummary, error)
	ChangeStatus(ctx context.Context, cmd ChangeStatusCommand) (MerchantSummary, error)
}

type ListFilter struct {
	Status *string // PENDING | APPROVED | REJECTED | SUSPENDED | DISABLED
	Query  string  // business name or contact email, case-insensitive, max 100 characters
}

type Page struct{ Number, Size int } // Size 1..100

type MerchantPage struct {
	Items []MerchantSummary
	Total int
}

type ChangeStatusCommand struct {
	MerchantID      string
	Action          string // APPROVE | REJECT | SUSPEND | REINSTATE | DISABLE
	ExpectedVersion int
	Reason          string // required for REJECT, SUSPEND, DISABLE (5..500 characters)
	Actor           Actor
}

type Actor struct {
	UserID, Role, RequestID, ClientIP, UserAgent string
}

// MerchantSummary uses the field names of Member 4's OpenAPI Merchant schema.
type MerchantSummary struct {
	MerchantID   string     `json:"merchant_id"`
	BusinessName string     `json:"business_name"`
	Status       string     `json:"status"`
	ContactEmail string     `json:"contact_email"`
	ContactPhone string     `json:"contact_phone"`
	ApprovedAt   *time.Time `json:"approved_at"`
	Version      int        `json:"version"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Errors Member 4 returns; the admin handler passes their codes through unchanged.
var (
	ErrNotFound          = errors.New("MERCHANT_NOT_FOUND")
	ErrInvalidTransition = errors.New("MERCHANT_INVALID_TRANSITION")
	ErrVersionConflict   = errors.New("MERCHANT_VERSION_CONFLICT")
	ErrReasonRequired    = errors.New("REASON_REQUIRED")
	ErrValidation        = errors.New("VALIDATION_FAILED")
)
