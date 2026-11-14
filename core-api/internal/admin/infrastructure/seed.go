package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
)

// DemoZone is one seeded delivery zone (Member 7 Doc 5 section 10.1).
type DemoZone struct {
	ID, Code, Name string
	Prefixes       []string
	Enabled        bool
}

// DemoZones are the 7 Sri Lankan zones for local, demo and E2E use. No fees here:
// Member 2's pricing seed adds prices for these ids.
var DemoZones = []DemoZone{
	{"ZONE_COLOMBO_01", "CMB-01", "Colombo Central", []string{"00100", "00200", "00300"}, true},
	{"ZONE_COLOMBO_02", "CMB-02", "Colombo South", []string{"00400", "00500", "00600"}, true},
	{"ZONE_COLOMBO_03", "CMB-03", "Colombo East", []string{"00700", "00800"}, true},
	{"ZONE_NEGOMBO_01", "NEG-01", "Negombo", []string{"11500"}, true},
	{"ZONE_KANDY_01", "KDY-01", "Kandy", []string{"20000"}, true},
	{"ZONE_GALLE_01", "GAL-01", "Galle", []string{"80000"}, true},
	{"ZONE_DEMO_DISABLED", "DEMO-OFF", "Demo Disabled Zone", []string{"99999"}, false},
}

// demoSettings mirror the allowlist in application/settings_rules.go. The migrations
// already insert them; repeating them here keeps a seed-only database complete.
var demoSettings = []struct {
	key, value, valueType, description string
}{
	{"reports.max_range_days", "92", "INTEGER", "Longest report period in days"},
	{"ops.board_refresh_seconds", "30", "INTEGER", "Dispatch board polling interval"},
	{"ops.maintenance_banner", `""`, "STRING", "Optional message shown on ops screens"},
	{"parcels.max_weight_kg", "25", "DECIMAL", "Heaviest parcel accepted, in kilograms (0.5 to 30)"},
	{"parcels.max_cod_cents", "3500000", "INTEGER", "Highest cash-on-delivery amount, in LKR cents (0 to 10000000)"},
}

// SeedDemo inserts the demo zones, their prefixes and the settings. It is safe to
// run many times: an existing zone is left exactly as an admin left it (including
// its prefixes), and an existing setting keeps its value. Callers must refuse to
// run it in production.
func SeedDemo(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("seed: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, z := range DemoZones {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO delivery_zones (id, code, name, enabled) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
			z.ID, z.Code, z.Name, z.Enabled)
		if err != nil {
			return fmt.Errorf("seed: zone %s: %w", z.ID, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue // zone exists (or its code is taken): do not touch its prefixes
		}
		for _, p := range z.Prefixes {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO zone_postal_codes (prefix, zone_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, p, z.ID); err != nil {
				return fmt.Errorf("seed: prefix %s: %w", p, err)
			}
		}
	}
	for _, s := range demoSettings {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO system_settings (key, value, value_type, description) VALUES ($1, $2::jsonb, $3, $4) ON CONFLICT (key) DO NOTHING`,
			s.key, s.value, s.valueType, s.description); err != nil {
			return fmt.Errorf("seed: setting %s: %w", s.key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("seed: commit: %w", err)
	}
	return nil
}
