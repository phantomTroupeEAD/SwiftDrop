package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
)

// generateUUIDv7 produces a time-ordered UUIDv7 string.
func generateUUIDv7() string {
	var uuid [16]byte
	nowMs := time.Now().UnixMilli()

	uuid[0] = byte(nowMs >> 40)
	uuid[1] = byte(nowMs >> 32)
	uuid[2] = byte(nowMs >> 24)
	uuid[3] = byte(nowMs >> 16)
	uuid[4] = byte(nowMs >> 8)
	uuid[5] = byte(nowMs)

	if _, err := rand.Read(uuid[6:]); err != nil {
		panic(err)
	}

	uuid[6] = (uuid[6] & 0x0f) | 0x70 // Version 7
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant RFC 4122

	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(uuid[0:4]),
		hex.EncodeToString(uuid[4:6]),
		hex.EncodeToString(uuid[6:8]),
		hex.EncodeToString(uuid[8:10]),
		hex.EncodeToString(uuid[10:16]),
	)
}

type fixtureData struct {
	EventID          string                 `json:"event_id"`
	EventType        string                 `json:"event_type"`
	AggregateType    string                 `json:"aggregate_type"`
	AggregateID      string                 `json:"aggregate_id"`
	AggregateVersion int                    `json:"aggregate_version"`
	SchemaVersion    string                 `json:"schema_version"`
	OccurredAt       string                 `json:"occurred_at"`
	CorrelationID    string                 `json:"correlation_id"`
	Producer         string                 `json:"producer"`
	Payload          map[string]interface{} `json:"payload"`
}

func main() {
	eventType := flag.String("type", "parcel.booked.v1", "Event type to emit from fixtures")
	count := flag.Int("count", 1, "Number of events to emit")
	merchantID := flag.String("merchant-id", "", "Optional override for payload merchant_id")
	email := flag.String("email", "", "Optional override for recipient_email")
	dbURL := flag.String("db", "", "PostgreSQL connection string (defaults to DATABASE_URL env)")
	flag.Parse()

	connStr := *dbURL
	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
	}
	if connStr == "" {
		connStr = "postgres://swiftdrop_user:swiftdrop_password@localhost:5432/swiftdrop_db?sslmode=disable"
	}

	// Locate fixture file
	// Search in ./tests/fixtures/events, ../tests/fixtures/events, etc.
	var fixtureBytes []byte
	candidates := []string{
		filepath.Join("tests", "fixtures", "events", *eventType+".json"),
		filepath.Join("services", "messaging", "tests", "fixtures", "events", *eventType+".json"),
		filepath.Join("..", "tests", "fixtures", "events", *eventType+".json"),
	}

	for _, cand := range candidates {
		data, err := os.ReadFile(cand)
		if err == nil {
			fixtureBytes = data
			break
		}
	}

	if len(fixtureBytes) == 0 {
		fmt.Fprintf(os.Stderr, "Error: fixture for event type %q not found\n", *eventType)
		os.Exit(1)
	}

	var base fixtureData
	if err := json.Unmarshal(fixtureBytes, &base); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid fixture JSON: %v\n", err)
		os.Exit(1)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open postgres: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to database: %v\n", err)
		os.Exit(1)
	}

	const insertQuery = `
		INSERT INTO outbox_events (
			event_id, event_type, aggregate_type, aggregate_id, aggregate_version,
			schema_version, occurred_at, correlation_id, producer, payload, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'PENDING'
		);
	`

	fmt.Printf("[devemit] Emitting %d outbox event(s) of type %s...\n", *count, *eventType)

	for i := 0; i < *count; i++ {
		eventID := generateUUIDv7()
		aggregateID := base.AggregateID
		if aggregateID == "" || *count > 1 {
			aggregateID = generateUUIDv7()
		}
		corrID := generateUUIDv7()

		payloadCopy := make(map[string]interface{})
		for k, v := range base.Payload {
			payloadCopy[k] = v
		}

		if *merchantID != "" {
			payloadCopy["merchant_id"] = *merchantID
		}

		if *email != "" {
			notifyObj, ok := payloadCopy["notify"].(map[string]interface{})
			if !ok {
				notifyObj = make(map[string]interface{})
			}
			notifyObj["recipient_email"] = *email
			payloadCopy["notify"] = notifyObj
		}

		payloadBytes, err := json.Marshal(payloadCopy)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to marshal payload: %v\n", err)
			os.Exit(1)
		}

		_, err = db.ExecContext(ctx, insertQuery,
			eventID,
			*eventType,
			base.AggregateType,
			aggregateID,
			base.AggregateVersion,
			base.SchemaVersion,
			time.Now().UTC(),
			corrID,
			base.Producer,
			payloadBytes,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting outbox event: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("  -> Inserted event_id=%s aggregate_id=%s\n", eventID, aggregateID)
	}

	fmt.Printf("[devemit] Done. Emitted %d event(s).\n", *count)
}
