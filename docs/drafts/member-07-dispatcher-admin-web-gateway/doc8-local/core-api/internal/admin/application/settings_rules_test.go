package application

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"swiftdrop.local/m7/core-api/internal/platform/apperr"
)

// SC-V01..V04: system_settings allowlist, types, bounds and size.
func TestValidateSetting(t *testing.T) {
	cases := []struct {
		key, val string
		want     error
	}{
		{"reports.max_range_days", `92`, nil},
		{"reports.max_range_days", `0`, apperr.ErrValidation},
		{"reports.max_range_days", `367`, apperr.ErrValidation},
		{"reports.max_range_days", `"92"`, apperr.ErrValidation},
		{"reports.max_range_days", `92.5`, apperr.ErrValidation},
		{"ops.board_refresh_seconds", `5`, apperr.ErrValidation},
		{"ops.board_refresh_seconds", `30`, nil},
		{"ops.maintenance_banner", `"System update at 22:00"`, nil},
		{"ops.maintenance_banner", `""`, nil},
		{"ops.maintenance_banner", `true`, apperr.ErrValidation},
		{"ops.maintenance_banner", `"` + strings.Repeat("a", 201) + `"`, apperr.ErrValidation},
		{"jwt.private_key", `"-----BEGIN"`, apperr.ErrUnknownSetting},
		{"", `1`, apperr.ErrUnknownSetting},
		{"reports.max_range_days", ``, apperr.ErrValidation},
		{"reports.max_range_days", strings.Repeat(" ", MaxSettingValueBytes) + "1", apperr.ErrValidation},
	}
	for _, c := range cases {
		err := ValidateSetting(c.key, json.RawMessage(c.val))
		if (c.want == nil && err != nil) || (c.want != nil && !errors.Is(err, c.want)) {
			t.Errorf("%s=%.20q: got %v, want %v", c.key, c.val, err, c.want)
		}
	}
}

// SC-V05: risky changes need a real reason (audit_logs.reason, Doc 5 U-38).
func TestValidateReason(t *testing.T) {
	for reason, ok := range map[string]bool{
		"":                                 false,
		"   fix     ":                      false,
		"Colombo 3 surcharge raised by M2": true,
		strings.Repeat("x", 501):           false,
		"මුදල් වෙනස් කිරීම සඳහා හේතුව": true,
	} {
		if err := ValidateReason(reason); (err == nil) != ok {
			t.Errorf("%.20q: got %v want ok=%v", reason, err, ok)
		}
	}
}
