package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/swiftdrop/core-api/migrations"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/swiftdrop?sslmode=disable"
	}

	fmt.Println("==================================================================")
	fmt.Println("   SwiftDrop Database Auto-Migrator (CLI)")
	fmt.Printf("   Target: %s\n", dbURL)
	fmt.Println("==================================================================")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v\n", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Could not connect to PostgreSQL: %v\nPlease ensure PostgreSQL is running.\n", err)
	}

	if err := migrations.RunMigrations(ctx, db); err != nil {
		log.Fatalf("Migration failed: %v\n", err)
	}

	fmt.Println("==================================================================")
	fmt.Println("   Migration Complete: Tables and seed users are ready in PostgreSQL!")
	fmt.Println("==================================================================")
}
