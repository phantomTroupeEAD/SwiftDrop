// Command seed adds demo rows (Doc 5 section 10.1). Safe to run many times.
// It refuses to run when APP_ENV=production.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

// Fixed ids so tokens and tests can refer to them. Users are a stand-in for Member 1's seed
// (the users table here is scripts/dev-stub-users.sql, never a real migration).
const (
	AdminID      = "00000000-0000-4000-8000-000000000001"
	DispatcherID = "00000000-0000-4000-8000-000000000002"
	CourierID    = "00000000-0000-4000-8000-000000000003"
)

// Zones carry no money: Member 2's pricing seed adds fees for these zone ids.
var zones = []struct {
	id, code, name string
	prefixes       []string
	enabled        bool
}{
	{"ZONE_COLOMBO_01", "CMB-01", "Colombo Central", []string{"00100", "00200", "00300"}, true},
	{"ZONE_COLOMBO_02", "CMB-02", "Colombo South", []string{"00400", "00500", "00600"}, true},
	{"ZONE_COLOMBO_03", "CMB-03", "Colombo East", []string{"00700", "00800"}, true},
	{"ZONE_NEGOMBO_01", "NEG-01", "Negombo", []string{"11500"}, true},
	{"ZONE_KANDY_01", "KDY-01", "Kandy", []string{"20000"}, true},
	{"ZONE_GALLE_01", "GAL-01", "Galle", []string{"80000"}, true},
	{"ZONE_DEMO_DISABLED", "DEMO-OFF", "Demo Disabled Zone", []string{"99999"}, false},
}

func main() {
	if os.Getenv("APP_ENV") == "production" {
		fmt.Fprintln(os.Stderr, "seed: refused, APP_ENV=production")
		os.Exit(2)
	}
	if err := seed(context.Background(), os.Getenv("DATABASE_URL")); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	fmt.Println("seed: done (7 zones, 3 settings, 3 demo users)")
}

func seed(ctx context.Context, url string) error {
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		for id, u := range map[string][2]string{AdminID: {"admin@swiftdrop.io", "ADMIN"}, DispatcherID: {"dispatcher@swiftdrop.io", "DISPATCHER"}, CourierID: {"courier1@swiftdrop.io", "COURIER"}} {
			if _, err := tx.Exec(ctx, "INSERT INTO users (id, email, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING", id, u[0], u[1]); err != nil {
				return err
			}
		}
		for _, z := range zones {
			if _, err := tx.Exec(ctx, `INSERT INTO delivery_zones (id, code, name, enabled) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
				z.id, z.code, z.name, z.enabled); err != nil {
				return err
			}
			for _, p := range z.prefixes {
				if _, err := tx.Exec(ctx, "INSERT INTO zone_postal_codes (prefix, zone_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", p, z.id); err != nil {
					return err
				}
			}
		}
		for _, s := range [][4]string{
			{"reports.max_range_days", "92", "INTEGER", "Longest report period in days"},
			{"ops.board_refresh_seconds", "30", "INTEGER", "Dispatch board polling interval"},
			{"ops.maintenance_banner", `""`, "STRING", "Optional message shown on ops screens"},
		} {
			if _, err := tx.Exec(ctx, "INSERT INTO system_settings (key, value, value_type, description) VALUES ($1, $2::jsonb, $3, $4) ON CONFLICT DO NOTHING", s[0], s[1], s[2], s[3]); err != nil {
				return err
			}
		}
		return nil
	})
}
