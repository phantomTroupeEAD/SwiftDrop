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
}
