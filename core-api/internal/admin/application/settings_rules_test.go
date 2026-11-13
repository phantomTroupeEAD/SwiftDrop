package application

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// SC-V01..V04: system_settings allowlist, types, bounds and size.
func TestValidateSetting(t *testing.T) {
	cases := []struct {
		key, val string
		want     error
	}{
		{"reports.max_range_days", `92`, nil},
		{"reports.max_range_days", `0`, ErrValidation},
		{"reports.max_range_days", `367`, ErrValidation},
		{"reports.max_range_days", `"92"`, ErrValidation},
		{"reports.max_range_days", `92.5`, ErrValidation},
		{"ops.board_refresh_seconds", `5`, ErrValidation},
		{"ops.board_refresh_seconds", `30`, nil},
		{"ops.maintenance_banner", `"System update at 22:00"`, nil},
		{"ops.maintenance_banner", `""`, nil},
		{"ops.maintenance_banner", `true`, ErrValidation},
		{"ops.maintenance_banner", `"` + strings.Repeat("a", 201) + `"`, ErrValidation},
		{"parcels.max_weight_kg", `0.5`, nil},
		{"parcels.max_weight_kg", `30`, nil},
		{"parcels.max_weight_kg", `25.0`, nil},
		{"parcels.max_weight_kg", `0.49`, ErrValidation},
		{"parcels.max_weight_kg", `30.01`, ErrValidation},
		{"parcels.max_weight_kg", `"25"`, ErrValidation},
		{"parcels.max_cod_cents", `0`, nil},
		{"parcels.max_cod_cents", `10000000`, nil},
		{"parcels.max_cod_cents", `10000001`, ErrValidation},
		{"parcels.max_cod_cents", `-1`, ErrValidation},
		{"parcels.max_cod_cents", `3500000.5`, ErrValidation},
		{"jwt.private_key", `"-----BEGIN"`, ErrUnknownSetting},
		{"", `1`, ErrUnknownSetting},
		{"reports.max_range_days", ``, ErrValidation},
		{"reports.max_range_days", strings.Repeat(" ", MaxSettingValueBytes) + "1", ErrValidation},
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
