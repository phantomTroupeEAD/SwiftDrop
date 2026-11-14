// Command seed loads demo data owned by the admin module: 7 Sri Lankan zones with
// postal prefixes and the system settings. It is safe to run repeatedly and refuses
// to run when APP_ENV=production. Run `go run ./cmd/migrate` first.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/swiftdrop/core-api/internal/admin/infrastructure"
)

func main() {
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") {
		log.Fatal("seed refuses to run when APP_ENV=production")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("could not connect to PostgreSQL: %v", err)
	}
	if err := infrastructure.SeedDemo(ctx, db); err != nil {
		log.Fatalf("seed failed: %v", err)
	}
	fmt.Printf("Seed complete: %d zones, 5 settings (existing rows kept).\n", len(infrastructure.DemoZones))
}
