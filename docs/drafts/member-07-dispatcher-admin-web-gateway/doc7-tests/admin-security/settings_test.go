package adminsec

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
		"Colombo 3 prefixes moved by ops": true,
		strings.Repeat("x", 501):           false,
		"මුදල් වෙනස් කිරීම සඳහා හේතුව": true,
	} {
		if err := ValidateReason(reason); (err == nil) != ok {
			t.Errorf("%.20q: got %v want ok=%v", reason, err, ok)
		}
	}
}

// SC-D01: secrets never enter audit_logs before/after snapshots.
func TestRedactForAudit(t *testing.T) {
	in := map[string]any{
		"code":          "ZONE_COLOMBO_01",
		"api_key":       "sd_live_abc",
		"Authorization": "Bearer x",
		"webhook":       map[string]any{"url": "https://m.example", "signingSecret": "whsec_1"},
		"history":       []any{map[string]any{"refresh_token": "rt_1", "at": "t"}, "plain"},
	}
	out := RedactForAudit(in)
	b, _ := json.Marshal(out)
	s := string(b)
	for _, leak := range []string{"sd_live_abc", "Bearer x", "whsec_1", "rt_1"} {
		if strings.Contains(s, leak) {
			t.Fatalf("leaked %q in %s", leak, s)
		}
	}
	if out["code"] != "ZONE_COLOMBO_01" || !strings.Contains(s, "https://m.example") || !strings.Contains(s, "plain") {
		t.Fatalf("normal fields must stay: %s", s)
	}
	if in["api_key"] != "sd_live_abc" {
		t.Fatal("input must not be changed")
	}
}
