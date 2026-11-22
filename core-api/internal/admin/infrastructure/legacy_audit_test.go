package infrastructure

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLegacySnapshotsKeepOwnStatesAsTheyAre(t *testing.T) {
	b, a := legacySnapshots([]byte(`{"name":"A"}`), []byte(`{"name":"B"}`), []byte(`{"previous_role":"ADMIN"}`))
	if string(b) != `{"name":"A"}` || string(a) != `{"name":"B"}` {
		t.Fatalf("own snapshots must win over metadata: %s %s", b, a)
	}
}

func TestLegacySnapshotsFromMetadata(t *testing.T) {
	b, a := legacySnapshots(nil, nil, []byte(`{"previous_role":"ADMIN","new_role":"COURIER"}`))
	var before, after map[string]any
	if err := json.Unmarshal(b, &before); err != nil || before["role"] != "ADMIN" || len(before) != 1 {
		t.Fatalf("before = %s (%v)", b, err)
	}
	if err := json.Unmarshal(a, &after); err != nil || after["role"] != "COURIER" || len(after) != 1 {
		t.Fatalf("after = %s (%v)", a, err)
	}
}

func TestLegacySnapshotsStatusChange(t *testing.T) {
	b, a := legacySnapshots(nil, nil, []byte(`{"previous_status":"ACTIVE","new_status":"SUSPENDED"}`))
	if !strings.Contains(string(b), `"status":"ACTIVE"`) || !strings.Contains(string(a), `"status":"SUSPENDED"`) {
		t.Fatalf("status change lost: %s %s", b, a)
	}
}

func TestLegacySnapshotsEmptyOrUnusableMetadataGivesNothing(t *testing.T) {
	for _, m := range []string{``, `{}`, `null`, `not json`, `{"note":"no prefixes here"}`} {
		b, a := legacySnapshots(nil, nil, []byte(m))
		if b != nil || a != nil {
			t.Errorf("metadata %q: want no snapshots, got %s %s", m, b, a)
		}
	}
}

// Secrets in another module's metadata never reach the screen.
func TestLegacySnapshotsRedactSecrets(t *testing.T) {
	_, a := legacySnapshots(nil, nil, []byte(`{"new_api_key":"sd_live_abc","new_role":"MERCHANT"}`))
	if strings.Contains(string(a), "sd_live_abc") || !strings.Contains(string(a), "[REDACTED]") {
		t.Fatalf("secret leaked: %s", a)
	}
}

// The merchants module nests its snapshots under "previous" and "new", with the reason inside.
func TestLegacySnapshotsNestedPreviousAndNew(t *testing.T) {
	meta := []byte(`{"previous":{"status":"APPROVED","version":1},"new":{"status":"DISABLED","version":2,"reason":"Contract ended."}}`)
	b, a := legacySnapshots(nil, nil, meta)
	if !strings.Contains(string(b), `"status":"APPROVED"`) || strings.Contains(string(b), "DISABLED") {
		t.Fatalf("before = %s", b)
	}
	if !strings.Contains(string(a), `"status":"DISABLED"`) || !strings.Contains(string(a), "Contract ended.") {
		t.Fatalf("after = %s", a)
	}
	if r := reasonFromSnapshot(a); r == nil || *r != "Contract ended." {
		t.Fatalf("reason = %v", r)
	}
}

func TestReasonFromSnapshotWithoutReason(t *testing.T) {
	for _, in := range []string{``, `{}`, `{"status":"X"}`, `{"reason":"   "}`, `not json`, `{"reason":5}`} {
		if r := reasonFromSnapshot([]byte(in)); r != nil {
			t.Errorf("%q: want nil, got %q", in, *r)
		}
	}
}

func TestLegacySnapshotsNestedSecretsAreRedacted(t *testing.T) {
	_, a := legacySnapshots(nil, nil, []byte(`{"new":{"api_key":"sd_live_abc","status":"APPROVED"}}`))
	if strings.Contains(string(a), "sd_live_abc") {
		t.Fatalf("secret leaked from nested snapshot: %s", a)
	}
}
