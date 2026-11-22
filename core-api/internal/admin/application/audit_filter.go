package application

import (
	"fmt"
	"regexp"
	"time"
)

// AuditFilter narrows the audit list. An empty field does not filter. Action, TargetType,
// TargetID and ActorRole match exactly. From and To are UTC instants: a row matches when
// created_at >= From and created_at < To.
type AuditFilter struct {
	Action, TargetType, TargetID, ActorRole string
	From, To                                *time.Time
}

// IsZero reports whether the filter lets every row through.
func (f AuditFilter) IsZero() bool {
	return f.Action == "" && f.TargetType == "" && f.TargetID == "" && f.ActorRole == "" && f.From == nil && f.To == nil
}

var (
	auditActionPattern     = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
	auditTargetTypePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,31}$`)
	auditTargetIDPattern   = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,64}$`)
)

// auditActorRoles match the actor_role CHECK constraint on audit_logs.
var auditActorRoles = map[string]bool{
	"ADMIN": true, "DISPATCHER": true, "COURIER": true, "MERCHANT": true, "CUSTOMER": true, "SYSTEM": true,
}

// ParseAuditFilter checks the query values and builds a filter. Dates are Sri Lanka days
// (YYYY-MM-DD) and both ends are inclusive, the same convention as the delivery report.
// Anything wrong is an ErrValidation, so the handler answers 422 VALIDATION_FAILED.
func ParseAuditFilter(action, targetType, targetID, actorRole, from, to string) (AuditFilter, error) {
	var f AuditFilter
	if action != "" {
		if !auditActionPattern.MatchString(action) {
			return f, fmt.Errorf("%w: action must be capital letters, digits and underscores, for example ZONE_CREATED", ErrValidation)
		}
		f.Action = action
	}
	if targetType != "" {
		if !auditTargetTypePattern.MatchString(targetType) {
			return f, fmt.Errorf("%w: target_type must be capital letters, digits and underscores, for example ZONE", ErrValidation)
		}
		f.TargetType = targetType
	}
	if targetID != "" {
		if !auditTargetIDPattern.MatchString(targetID) {
			return f, fmt.Errorf("%w: target_id must be 1 to 64 letters, digits or _ . : -", ErrValidation)
		}
		f.TargetID = targetID
	}
	if actorRole != "" {
		if !auditActorRoles[actorRole] {
			return f, fmt.Errorf("%w: actor_role must be ADMIN, DISPATCHER, COURIER, MERCHANT, CUSTOMER or SYSTEM", ErrValidation)
		}
		f.ActorRole = actorRole
	}

	var fromDay, toDay time.Time
	if from != "" {
		d, err := time.ParseInLocation("2006-01-02", from, colombo)
		if err != nil {
			return f, fmt.Errorf("%w: from must be a date like 2026-11-14", ErrValidation)
		}
		fromDay = d
		start := d.UTC()
		f.From = &start
	}
	if to != "" {
		d, err := time.ParseInLocation("2006-01-02", to, colombo)
		if err != nil {
			return f, fmt.Errorf("%w: to must be a date like 2026-11-14", ErrValidation)
		}
		toDay = d
		end := d.Add(24 * time.Hour).UTC() // the whole "to" day is included
		f.To = &end
	}
	if f.From != nil && f.To != nil && fromDay.After(toDay) {
		return AuditFilter{}, fmt.Errorf("%w: from must not be after to", ErrValidation)
	}
	return f, nil
}
