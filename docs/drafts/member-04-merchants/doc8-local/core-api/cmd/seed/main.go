// Command seed inserts local demo users (stand-ins for Member 1's seed). Idempotent.
// LOCAL USE ONLY: refuses to run when APP_ENV is staging or production (Doc 5 §11.1).
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"swiftdrop.local/m4/core-api/internal/platform/db"
)

var users = []struct{ id, email string }{
	{"00000000-0000-4000-8000-0000000000ad", "admin@swiftdrop.local"},
	{"00000000-0000-4000-8000-0000000000a1", "owner.a@ceylontea.local"},
	{"00000000-0000-4000-8000-0000000000b1", "owner.b@spiceshop.local"},
}

func main() {
	if env := os.Getenv("APP_ENV"); env == "staging" || env == "production" {
		fmt.Fprintln(os.Stderr, "seed: refused, APP_ENV="+env)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	defer pool.Close()
	for _, u := range users {
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email) VALUES ($1,$2) ON CONFLICT (id) DO NOTHING`, u.id, u.email); err != nil {
			fmt.Fprintln(os.Stderr, "seed:", err)
			os.Exit(1)
		}
	}
	fmt.Printf("seed: %d demo users present\n", len(users))
}
