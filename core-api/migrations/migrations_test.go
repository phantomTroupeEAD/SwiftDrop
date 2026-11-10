package migrations

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var versionRegex = regexp.MustCompile(`^(\d{14})_([a-zA-Z0-9_]+)\.(up|down)\.sql$`)
var forbiddenPatterns = []string{
	"BEGIN;",
	"COMMIT;",
	"CONCURRENTLY",
}

func TestMigrationFiles_SanityAndRules(t *testing.T) {
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	upFiles := make(map[string]string)   // version -> filename
	downFiles := make(map[string]string) // version -> filename
	var versions []string

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		matches := versionRegex.FindStringSubmatch(file.Name())
		if len(matches) != 4 {
			t.Errorf("migration filename %q does not match required 14-digit format: NNNNNNNNNNNNNN_name.(up|down).sql", file.Name())
			continue
		}

		version := matches[1]
		direction := matches[3]

		if direction == "up" {
			if _, exists := upFiles[version]; exists {
				t.Errorf("duplicate up migration version: %s", version)
			}
			upFiles[version] = file.Name()
			versions = append(versions, version)
		} else if direction == "down" {
			if _, exists := downFiles[version]; exists {
				t.Errorf("duplicate down migration version: %s", version)
			}
			downFiles[version] = file.Name()
		}

		// Check forbidden SQL constructs (Decision D28)
		contentBytes, err := os.ReadFile(file.Name())
		if err != nil {
			t.Fatalf("failed to read %s: %v", file.Name(), err)
		}
		contentUpper := strings.ToUpper(string(contentBytes))

		for _, forbidden := range forbiddenPatterns {
			if strings.Contains(contentUpper, forbidden) {
				t.Errorf("migration file %s contains forbidden construct %q (migrator manages its own transactions; CONCURRENTLY is unsafe in transactional migrator)", file.Name(), forbidden)
			}
		}
	}

	// 1. Every up migration must have a matching down migration
	for v, upName := range upFiles {
		if _, hasDown := downFiles[v]; !hasDown {
			t.Errorf("migration %s is missing its corresponding .down.sql file", upName)
		}
	}

	// 2. Every down migration must have a matching up migration
	for v, downName := range downFiles {
		if _, hasUp := upFiles[v]; !hasUp {
			t.Errorf("migration %s is missing its corresponding .up.sql file", downName)
		}
	}

	// 3. Verify known dependency order
	sort.Strings(versions)
	outboxIdx := -1
	workerIdx := -1
	for idx, v := range versions {
		if v == "20261101000050" {
			outboxIdx = idx
		}
		if v == "20261101000500" {
			workerIdx = idx
		}
	}

	if outboxIdx == -1 {
		t.Errorf("missing outbox migration 20261101000050")
	}
	if workerIdx == -1 {
		t.Errorf("missing worker migration 20261101000500")
	}
	if outboxIdx != -1 && workerIdx != -1 && outboxIdx >= workerIdx {
		t.Errorf("outbox migration (20261101000050) must precede worker migration (20261101000500)")
	}

	// 4. Verify Member 2 pricing, parcels, and idempotency migration order
	pricingIdx := -1
	parcelsIdx := -1
	idempotencyIdx := -1
	for idx, v := range versions {
		switch v {
		case "20261110000001":
			pricingIdx = idx
		case "20261110000002":
			parcelsIdx = idx
		case "20261110000003":
			idempotencyIdx = idx
		}
	}
	if pricingIdx == -1 {
		t.Errorf("missing pricing migration 20261110000001")
	}
	if parcelsIdx == -1 {
		t.Errorf("missing parcel migration 20261110000002")
	}
	if idempotencyIdx == -1 {
		t.Errorf("missing idempotency migration 20261110000003")
	}
	if pricingIdx != -1 && parcelsIdx != -1 && pricingIdx >= parcelsIdx {
		t.Errorf("pricing migration (20261110000001) must precede parcel migration (20261110000002)")
	}
	if parcelsIdx != -1 && idempotencyIdx != -1 && parcelsIdx >= idempotencyIdx {
		t.Errorf("parcel migration (20261110000002) must precede idempotency migration (20261110000003)")
	}
}

func TestIdempotencyMigration_SchemaContract(t *testing.T) {
	upBytes, err := os.ReadFile("20261110000003_create_idempotency_keys.up.sql")
	if err != nil {
		t.Fatalf("failed to read idempotency up migration: %v", err)
	}
	upSQL := string(upBytes)

	requiredUpTokens := []string{
		"CREATE TABLE IF NOT EXISTS idempotency_keys",
		"key",
		"caller_scope",
		"operation",
		"request_hash",
		"response_status",
		"response_body",
		"created_resource_id",
		"created_at",
		"expires_at",
		"PRIMARY KEY (key, caller_scope)",
		"idx_idempotency_keys_expires_at",
	}
	for _, token := range requiredUpTokens {
		if !strings.Contains(upSQL, token) {
			t.Errorf("idempotency up migration missing expected schema token %q", token)
		}
	}

	downBytes, err := os.ReadFile("20261110000003_create_idempotency_keys.down.sql")
	if err != nil {
		t.Fatalf("failed to read idempotency down migration: %v", err)
	}
	downSQL := string(downBytes)
	if !strings.Contains(downSQL, "DROP TABLE IF EXISTS idempotency_keys;") {
		t.Errorf("idempotency down migration must drop table idempotency_keys")
	}
}
