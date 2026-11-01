package migrations_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Allowed columns per table defined in migrations 20261101000050 and 20261101000500.
var tableColumns = map[string]map[string]bool{
	"outbox_events": {
		"id":                true,
		"producer":          true,
		"occurred_at":       true,
		"correlation_id":    true,
		"aggregate_type":    true,
		"aggregate_id":      true,
		"aggregate_version": true,
		"event_type":        true,
		"schema_version":    true,
		"payload":           true,
		"status":            true,
		"retry_count":       true,
		"next_retry_at":     true,
		"lease_expires_at":  true,
		"published_at":      true,
		"last_error":        true,
		"created_at":        true,
	},
	"notification_deliveries": {
		"id":                  true,
		"event_id":            true,
		"event_type":          true,
		"aggregate_id":        true,
		"aggregate_version":   true,
		"payload":             true,
		"recipient":           true,
		"channel":             true,
		"template_name":       true,
		"status":              true,
		"attempt_count":       true,
		"next_attempt_at":     true,
		"lease_expires_at":    true,
		"provider_message_id": true,
		"last_error":          true,
		"created_at":          true,
		"delivered_at":        true,
	},
	"webhook_deliveries": {
		"id":                true,
		"endpoint_id":       true,
		"event_id":          true,
		"event_type":        true,
		"aggregate_id":      true,
		"aggregate_version": true,
		"occurred_at":       true,
		"payload":           true,
		"status":            true,
		"attempt_count":     true,
		"next_attempt_at":   true,
		"lease_expires_at":  true,
		"last_status_code":  true,
		"last_error":        true,
		"created_at":        true,
		"delivered_at":      true,
	},
	"webhook_delivery_attempts": {
		"id":                    true,
		"delivery_id":           true,
		"attempt_no":            true,
		"started_at":            true,
		"response_status_code":  true,
		"response_body_snippet": true,
		"error_class":           true,
		"error_message":         true,
		"next_attempt_at":       true,
		"created_at":            true,
	},
	"processed_events": {
		"consumer_name": true,
		"event_id":      true,
		"processed_at":  true,
	},
}

// Specifically forbidden column names identified during audits.
var forbiddenColumnPatterns = []struct {
	Table       string
	BadColumn   string
	Description string
}{
	{"outbox_events", "event_id", "outbox_events primary key is 'id', NOT 'event_id'"},
	{"outbox_events", "updated_at", "outbox_events does not have an 'updated_at' column"},
	{"notification_deliveries", "updated_at", "notification_deliveries does not have 'updated_at' (use delivered_at or created_at)"},
	{"notification_deliveries", "recipient_email", "notification_deliveries recipient column is 'recipient', NOT 'recipient_email'"},
	{"webhook_deliveries", "updated_at", "webhook_deliveries does not have 'updated_at' (use delivered_at or created_at)"},
}

func findRepoRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}

func TestSQLColumnGuard_NonTestGoFiles(t *testing.T) {
	repoRoot, err := findRepoRoot(".")
	if err != nil {
		t.Fatalf("failed to find repo root from cwd: %v", err)
	}

	dirsToScan := []string{
		filepath.Join(repoRoot, "core-api", "internal", "events"),
		filepath.Join(repoRoot, "services", "messaging", "cmd"),
		filepath.Join(repoRoot, "services", "messaging", "internal"),
	}

	var goFiles []string
	for _, dir := range dirsToScan {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				goFiles = append(goFiles, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("failed to walk dir %s: %v", dir, err)
		}
	}

	if len(goFiles) == 0 {
		t.Fatalf("no Go files found to scan in %v", dirsToScan)
	}

	fset := token.NewFileSet()
	for _, file := range goFiles {
		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("failed to parse %s: %v", file, err)
		}

		ast.Inspect(node, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}

			raw := lit.Value
			if len(raw) >= 2 && ((raw[0] == '`' && raw[len(raw)-1] == '`') || (raw[0] == '"' && raw[len(raw)-1] == '"')) {
				raw = raw[1 : len(raw)-1]
			}
			valUpper := strings.ToUpper(raw)

			isSQL := strings.Contains(valUpper, "SELECT ") ||
				strings.Contains(valUpper, "INSERT INTO ") ||
				strings.Contains(valUpper, "UPDATE ") ||
				strings.Contains(valUpper, "DELETE FROM ")

			if !isSQL {
				return true
			}

			pos := fset.Position(lit.Pos())

			// 1. Check specific forbidden patterns
			for _, f := range forbiddenColumnPatterns {
				if strings.Contains(strings.ToLower(raw), f.Table) && strings.Contains(strings.ToLower(raw), f.BadColumn) {
					t.Errorf("%s:%d: SQL contains forbidden column %q on table %q: %s\nQuery snippet:\n%s",
						pos.Filename, pos.Line, f.BadColumn, f.Table, f.Description, raw)
				}
			}

			// 2. Validate columns in INSERT INTO <table> (<col1>, <col2>, ...)
			insertRegex := regexp.MustCompile(`(?i)INSERT\s+INTO\s+([a-z0-9_]+)\s*\(([^)]+)\)`)
			for _, match := range insertRegex.FindAllStringSubmatch(raw, -1) {
				tbl := strings.ToLower(match[1])
				allowedCols, ok := tableColumns[tbl]
				if !ok {
					continue
				}
				cols := strings.Split(match[2], ",")
				for _, c := range cols {
					col := strings.TrimSpace(strings.ToLower(c))
					if col == "" {
						continue
					}
					if !allowedCols[col] {
						t.Errorf("%s:%d: INSERT INTO %s references unknown column %q", pos.Filename, pos.Line, tbl, col)
					}
				}
			}

			return true
		})
	}
}

