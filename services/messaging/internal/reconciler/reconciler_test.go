package reconciler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockDriver struct {
	mu                  sync.Mutex
	queries             []string
	execs               []string
	staleCount          int
	notifLeases         int64
	hookLeases          int64
	overdueNotif        int
	overdueHook         int
	retentionOutboxRows []int64
	retentionNotifRows  []int64
	retentionHookRows   []int64
	retentionEventRows  []int64
	retentionIdx        map[string]int
}

var testDriver = &mockDriver{
	retentionIdx: make(map[string]int),
}

func init() {
	sql.Register("reconciler_mock_driver", testDriver)
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

func (s *mockStmt) Close() error  { return nil }
func (s *mockStmt) NumInput() int { return -1 }

func (s *mockStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.c.d.mu.Lock()
	defer s.c.d.mu.Unlock()
	s.c.d.execs = append(s.c.d.execs, s.query)

	q := strings.ToUpper(s.query)
	if strings.Contains(q, "SET LOCAL STATEMENT_TIMEOUT") {
		return driver.RowsAffected(0), nil
	}

	if strings.Contains(q, "UPDATE NOTIFICATION_DELIVERIES") {
		return driver.RowsAffected(s.c.d.notifLeases), nil
	}
	if strings.Contains(q, "UPDATE WEBHOOK_DELIVERIES") {
		return driver.RowsAffected(s.c.d.hookLeases), nil
	}

	if strings.Contains(q, "DELETE FROM OUTBOX_EVENTS") {
		idx := s.c.d.retentionIdx["outbox"]
		if idx < len(s.c.d.retentionOutboxRows) {
			res := s.c.d.retentionOutboxRows[idx]
			s.c.d.retentionIdx["outbox"]++
			return driver.RowsAffected(res), nil
		}
		return driver.RowsAffected(0), nil
	}

	if strings.Contains(q, "DELETE FROM NOTIFICATION_DELIVERIES") {
		idx := s.c.d.retentionIdx["notif"]
		if idx < len(s.c.d.retentionNotifRows) {
			res := s.c.d.retentionNotifRows[idx]
			s.c.d.retentionIdx["notif"]++
			return driver.RowsAffected(res), nil
		}
		return driver.RowsAffected(0), nil
	}

	if strings.Contains(q, "DELETE FROM WEBHOOK_DELIVERIES") {
		idx := s.c.d.retentionIdx["hook"]
		if idx < len(s.c.d.retentionHookRows) {
			res := s.c.d.retentionHookRows[idx]
			s.c.d.retentionIdx["hook"]++
			return driver.RowsAffected(res), nil
		}
		return driver.RowsAffected(0), nil
	}

	if strings.Contains(q, "DELETE FROM PROCESSED_EVENTS") {
		idx := s.c.d.retentionIdx["event"]
		if idx < len(s.c.d.retentionEventRows) {
			res := s.c.d.retentionEventRows[idx]
			s.c.d.retentionIdx["event"]++
			return driver.RowsAffected(res), nil
		}
		return driver.RowsAffected(0), nil
	}

	return driver.RowsAffected(0), nil
}

func (s *mockStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.c.d.mu.Lock()
	defer s.c.d.mu.Unlock()
	s.c.d.queries = append(s.c.d.queries, s.query)

	q := strings.ToUpper(s.query)
	if strings.Contains(q, "FROM OUTBOX_EVENTS") && strings.Contains(q, "COUNT(*)") {
		return &singleIntRows{val: int64(s.c.d.staleCount)}, nil
	}
	if strings.Contains(q, "FROM NOTIFICATION_DELIVERIES") && strings.Contains(q, "COUNT(*)") {
		return &singleIntRows{val: int64(s.c.d.overdueNotif)}, nil
	}
	if strings.Contains(q, "FROM WEBHOOK_DELIVERIES") && strings.Contains(q, "COUNT(*)") {
		return &singleIntRows{val: int64(s.c.d.overdueHook)}, nil
	}
	if strings.Contains(q, "FROM WEBHOOK_DELIVERY_ATTEMPTS") {
		return &singleIntRows{val: 0}, nil
	}
	if strings.Contains(q, "INFORMATION_SCHEMA.TABLES") {
		return &singleBoolRows{val: false}, nil
	}

	return &singleIntRows{val: 0}, nil
}

type singleIntRows struct {
	val  int64
	read bool
}

func (r *singleIntRows) Columns() []string { return []string{"count"} }
func (r *singleIntRows) Close() error      { return nil }
func (r *singleIntRows) Next(dest []driver.Value) error {
	if r.read {
		return io.EOF
	}
	r.read = true
	dest[0] = r.val
	return nil
}

type singleBoolRows struct {
	val  bool
	read bool
}

func (r *singleBoolRows) Columns() []string { return []string{"exists"} }
func (r *singleBoolRows) Close() error      { return nil }
func (r *singleBoolRows) Next(dest []driver.Value) error {
	if r.read {
		return io.EOF
	}
	r.read = true
	dest[0] = r.val
	return nil
}

func TestCheckStaleOutbox_DetectedNotRepublished(t *testing.T) {
	testDriver.mu.Lock()
	testDriver.queries = nil
	testDriver.execs = nil
	testDriver.staleCount = 7
	testDriver.mu.Unlock()

	db, err := sql.Open("reconciler_mock_driver", "")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	rec := NewReconciler(db, slog.Default(), Options{
		StaleThreshold: 5 * time.Minute,
	})

	count, err := rec.CheckStaleOutbox(context.Background(), 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 7 {
		t.Errorf("expected count 7, got %d", count)
	}

	testDriver.mu.Lock()
	defer testDriver.mu.Unlock()

	// Verify that ONLY SELECT was performed; NO UPDATE or DELETE on outbox_events
	for _, exec := range testDriver.execs {
		if strings.Contains(strings.ToUpper(exec), "OUTBOX_EVENTS") {
			t.Errorf("forbidden write on outbox_events during stale check: %s", exec)
		}
	}
}

func TestResetExpiredLeases(t *testing.T) {
	testDriver.mu.Lock()
	testDriver.execs = nil
	testDriver.notifLeases = 3
	testDriver.hookLeases = 5
	testDriver.mu.Unlock()

	db, err := sql.Open("reconciler_mock_driver", "")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	rec := NewReconciler(db, slog.Default(), Options{})
	notifs, hooks, err := rec.ResetExpiredLeases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if notifs != 3 {
		t.Errorf("expected 3 reset notification leases, got %d", notifs)
	}
	if hooks != 5 {
		t.Errorf("expected 5 reset webhook leases, got %d", hooks)
	}
}

func TestReportOverdueDeliveries(t *testing.T) {
	testDriver.mu.Lock()
	testDriver.overdueNotif = 2
	testDriver.overdueHook = 4
	testDriver.mu.Unlock()

	db, err := sql.Open("reconciler_mock_driver", "")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	rec := NewReconciler(db, slog.Default(), Options{})
	notifs, hooks, err := rec.ReportOverdueDeliveries(context.Background(), 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if notifs != 2 {
		t.Errorf("expected 2 overdue notifs, got %d", notifs)
	}
	if hooks != 4 {
		t.Errorf("expected 4 overdue hooks, got %d", hooks)
	}
}

func TestRunRetention_BoundedBatch(t *testing.T) {
	testDriver.mu.Lock()
	testDriver.retentionIdx = make(map[string]int)
	// Simulate 2 batches of 1000 then 500
	testDriver.retentionOutboxRows = []int64{1000, 500}
	testDriver.retentionNotifRows = []int64{250}
	testDriver.retentionHookRows = []int64{120}
	testDriver.retentionEventRows = []int64{300}
	testDriver.mu.Unlock()

	db, err := sql.Open("reconciler_mock_driver", "")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	rec := NewReconciler(db, slog.Default(), Options{BatchLimit: 1000})
	report, err := rec.RunRetention(context.Background(), 1000)
	if err != nil {
		t.Fatalf("retention run failed: %v", err)
	}

	if report.DeletedOutbox != 1500 {
		t.Errorf("expected 1500 deleted outbox events, got %d", report.DeletedOutbox)
	}
	if report.DeletedNotifications != 250 {
		t.Errorf("expected 250 deleted notifications, got %d", report.DeletedNotifications)
	}
	if report.DeletedWebhooks != 120 {
		t.Errorf("expected 120 deleted webhooks, got %d", report.DeletedWebhooks)
	}
	if report.DeletedEvents != 300 {
		t.Errorf("expected 300 deleted events, got %d", report.DeletedEvents)
	}
}

func TestRunRetention_RunTwiceNoOp(t *testing.T) {
	testDriver.mu.Lock()
	testDriver.retentionIdx = make(map[string]int)
	testDriver.retentionOutboxRows = []int64{0}
	testDriver.retentionNotifRows = []int64{0}
	testDriver.retentionHookRows = []int64{0}
	testDriver.retentionEventRows = []int64{0}
	testDriver.mu.Unlock()

	db, err := sql.Open("reconciler_mock_driver", "")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	rec := NewReconciler(db, slog.Default(), Options{BatchLimit: 1000})

	// Run 1: 0 rows
	report1, err := rec.RunRetention(context.Background(), 1000)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	if report1.DeletedOutbox != 0 {
		t.Errorf("expected 0 on run 1, got %d", report1.DeletedOutbox)
	}

	// Run 2: immediately 0 rows (idempotent)
	report2, err := rec.RunRetention(context.Background(), 1000)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if report2.DeletedOutbox != 0 {
		t.Errorf("expected 0 on run 2, got %d", report2.DeletedOutbox)
	}
}

func TestReconciler_FullRun(t *testing.T) {
	testDriver.mu.Lock()
	testDriver.staleCount = 1
	testDriver.notifLeases = 2
	testDriver.hookLeases = 1
	testDriver.overdueNotif = 0
	testDriver.overdueHook = 0
	testDriver.mu.Unlock()

	db, err := sql.Open("reconciler_mock_driver", "")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	rec := NewReconciler(db, slog.Default(), Options{})
	summary, err := rec.Run(context.Background())
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if summary.StaleOutboxCount != 1 {
		t.Errorf("expected 1 stale outbox, got %d", summary.StaleOutboxCount)
	}
	if summary.ResetNotificationLeases != 2 {
		t.Errorf("expected 2 reset notification leases, got %d", summary.ResetNotificationLeases)
	}
	if summary.ResetWebhookLeases != 1 {
		t.Errorf("expected 1 reset webhook lease, got %d", summary.ResetWebhookLeases)
	}
}
