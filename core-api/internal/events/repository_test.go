package events

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync"
	"testing"
)

// Mock Driver to test multi-row SQL generation without live Postgres or external mock libraries
type mockDriver struct {
	mu            sync.Mutex
	executedQuery string
	executedArgs  []driver.Value
}

var testMockDriver = &mockDriver{}

func init() {
	sql.Register("mock_outbox_driver", testMockDriver)
}

func (d *mockDriver) Open(name string) (driver.Conn, error) {
	return &mockConn{d: d}, nil
}

type mockConn struct {
	d *mockDriver
}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return &mockStmt{c: c, query: query}, nil
}

func (c *mockConn) Close() error { return nil }

func (c *mockConn) Begin() (driver.Tx, error) {
	return &mockTx{}, nil
}

type mockTx struct{}

func (t *mockTx) Commit() error   { return nil }
func (t *mockTx) Rollback() error { return nil }

type mockStmt struct {
	c     *mockConn
	query string
}

func (s *mockStmt) Close() error { return nil }

func (s *mockStmt) NumInput() int { return -1 }

func (s *mockStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.c.d.mu.Lock()
	defer s.c.d.mu.Unlock()
	s.c.d.executedQuery = s.query
	s.c.d.executedArgs = args
	return driver.RowsAffected(1), nil
}

func (s *mockStmt) Query(args []driver.Value) (driver.Rows, error) {
	return nil, errors.New("not implemented")
}

func (s *mockStmt) CheckNamedValue(nv *driver.NamedValue) error {
	return nil
}

// Verify mockConn implements driver.ExecerContext
var _ driver.ExecerContext = (*mockConn)(nil)

func (c *mockConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	c.d.executedQuery = query
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	c.d.executedArgs = values
	return driver.RowsAffected(1), nil
}

type dummyPayload struct {
	TrackingNumber string `json:"tracking_number"`
}

func TestPostgresOutboxWriter_NilTx(t *testing.T) {
	writer := NewPostgresOutboxWriter()
	err := writer.Write(context.Background(), nil)
	if !errors.Is(err, ErrMissingTransaction) {
		t.Fatalf("expected ErrMissingTransaction, got: %v", err)
	}
}

func TestPostgresOutboxWriter_EmptyEnvelopes(t *testing.T) {
	writer := NewPostgresOutboxWriter()
	dummyTx := &sql.Tx{}
	err := writer.Write(context.Background(), dummyTx)
	if err != nil {
		t.Fatalf("expected nil error on empty envelopes, got: %v", err)
	}
}

func TestPostgresOutboxWriter_ValidationErrorStopsExecution(t *testing.T) {
	writer := NewPostgresOutboxWriter()
	dummyTx := &sql.Tx{}

	invalidEnv := EventEnvelope[dummyPayload]{
		EventID:   "not-a-uuid",
		EventType: "invalid",
	}

	err := writer.Write(context.Background(), dummyTx, invalidEnv)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidEventID) {
		t.Fatalf("expected ErrInvalidEventID, got: %v", err)
	}
}

func TestPostgresOutboxWriter_MultiRowInsertExecution(t *testing.T) {
	db, err := sql.Open("mock_outbox_driver", "test")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback() //nolint:errcheck

	writer := NewPostgresOutboxWriter()

	aggID1, _ := GenerateUUIDv7()
	aggID2, _ := GenerateUUIDv7()

	env1, err := NewEnvelope(
		ParcelBookedV1,
		"parcel",
		aggID1,
		1,
		"corr-001",
		"core-api.parcels",
		dummyPayload{TrackingNumber: "SD-001"},
	)
	if err != nil {
		t.Fatalf("failed to create env1: %v", err)
	}

	env2, err := NewEnvelope(
		ParcelAssignedV1,
		"parcel",
		aggID1,
		2,
		"corr-002",
		"core-api.dispatch",
		dummyPayload{TrackingNumber: "SD-001"},
	)
	if err != nil {
		t.Fatalf("failed to create env2: %v", err)
	}

	env3, err := NewEnvelope(
		CourierUnassignedV1,
		"courier",
		aggID2,
		1,
		"corr-003",
		"core-api.dispatch",
		dummyPayload{TrackingNumber: "SD-002"},
	)
	if err != nil {
		t.Fatalf("failed to create env3: %v", err)
	}

	ctx := context.Background()
	if err := writer.Write(ctx, tx, env1, env2, env3); err != nil {
		t.Fatalf("expected write to succeed, got: %v", err)
	}

	testMockDriver.mu.Lock()
	query := testMockDriver.executedQuery
	args := testMockDriver.executedArgs
	testMockDriver.mu.Unlock()

	// Verify multi-row query shape
	if !strings.Contains(query, "INSERT INTO outbox_events") {
		t.Errorf("query does not contain INSERT INTO outbox_events: %s", query)
	}

	// 3 envelopes * 11 columns = 33 arguments
	expectedArgsCount := 3 * 11
	if len(args) != expectedArgsCount {
		t.Fatalf("expected %d query arguments, got %d", expectedArgsCount, len(args))
	}

	// Check first envelope args (0..10)
	if args[0] != env1.EventID {
		t.Errorf("expected arg[0] == %s, got %v", env1.EventID, args[0])
	}
	if args[7] != ParcelBookedV1 {
		t.Errorf("expected arg[7] == %s, got %v", ParcelBookedV1, args[7])
	}
	if args[10] != "PENDING" {
		t.Errorf("expected arg[10] == 'PENDING', got %v", args[10])
	}

	// Check second envelope args (11..21)
	if args[11] != env2.EventID {
		t.Errorf("expected arg[11] == %s, got %v", env2.EventID, args[11])
	}
	if args[18] != ParcelAssignedV1 {
		t.Errorf("expected arg[18] == %s, got %v", ParcelAssignedV1, args[18])
	}

	// Check third envelope args (22..32)
	if args[22] != env3.EventID {
		t.Errorf("expected arg[22] == %s, got %v", env3.EventID, args[22])
	}
	if args[29] != CourierUnassignedV1 {
		t.Errorf("expected arg[29] == %s, got %v", CourierUnassignedV1, args[29])
	}
}

// Ensure mock driver satisfies driver.Driver interface
var _ driver.Driver = (*mockDriver)(nil)
