package web

import (
	"encoding/json"
	"strings"
	"testing"
)

// SC-D01: secrets never enter audit_logs before/after snapshots.
func TestRedactForAudit(t *testing.T) {
	in := map[string]any{
		"code":          "ZONE_COLOMBO_CENTRAL",
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
	if out["code"] != "ZONE_COLOMBO_CENTRAL" || !strings.Contains(s, "https://m.example") || !strings.Contains(s, "plain") {
		t.Fatalf("normal fields must stay: %s", s)
	}
	if in["api_key"] != "sd_live_abc" {
		t.Fatal("input must not be changed")
	}
}
