package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"
	"time"
)

type mockDBDriver struct {
	mu           sync.Mutex
	commitCalled bool
	rbCalled     bool
}

var testDriver = &mockDBDriver{}

func init() {
	sql.Register("test_platform_db_driver", testDriver)
}

func (d *mockDBDriver) Open(name string) (driver.Conn, error) {
	return &mockDBConn{d: d}, nil
}

type mockDBConn struct {
	d *mockDBDriver
}

func (c *mockDBConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *mockDBConn) Close() error { return nil }

func (c *mockDBConn) Begin() (driver.Tx, error) {
	return &mockDBTx{d: c.d}, nil
}

type mockDBTx struct {
	d *mockDBDriver
}

func (t *mockDBTx) Commit() error {
	t.d.mu.Lock()
	defer t.d.mu.Unlock()
	t.d.commitCalled = true
	return nil
}

func (t *mockDBTx) Rollback() error {
	t.d.mu.Lock()
	defer t.d.mu.Unlock()
	t.d.rbCalled = true
	return nil
}

func (d *mockDBDriver) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.commitCalled = false
	d.rbCalled = false
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.MaxOpenConns != 10 {
		t.Errorf("expected 10 MaxOpenConns, got %d", opts.MaxOpenConns)
	}
	if opts.MaxIdleConns != 5 {
		t.Errorf("expected 5 MaxIdleConns, got %d", opts.MaxIdleConns)
	}
	if opts.ConnMaxLifetime != 30*time.Minute {
		t.Errorf("expected 30m ConnMaxLifetime, got %v", opts.ConnMaxLifetime)
	}
	if opts.ConnMaxIdleTime != 5*time.Minute {
		t.Errorf("expected 5m ConnMaxIdleTime, got %v", opts.ConnMaxIdleTime)
	}
	if opts.ConnectTimeout != 5*time.Second {
		t.Errorf("expected 5s ConnectTimeout, got %v", opts.ConnectTimeout)
	}
}

func TestWithTx_NilDB(t *testing.T) {
	err := WithTx(context.Background(), nil, func(tx *sql.Tx) error {
		return nil
	})
	if !errors.Is(err, ErrNilDB) {
		t.Fatalf("expected ErrNilDB, got: %v", err)
	}
}

func TestWithTx_SuccessCommit(t *testing.T) {
	testDriver.reset()
	db, err := sql.Open("test_platform_db_driver", "")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	err = WithTx(context.Background(), db, func(tx *sql.Tx) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	testDriver.mu.Lock()
	committed := testDriver.commitCalled
	rolledBack := testDriver.rbCalled
	testDriver.mu.Unlock()

	if !committed {
		t.Error("expected transaction to be committed")
	}
	if rolledBack {
		t.Error("expected transaction not to be rolled back")
	}
}

func TestWithTx_ErrorRollback(t *testing.T) {
	testDriver.reset()
	db, err := sql.Open("test_platform_db_driver", "")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	expectedErr := errors.New("business error")
	err = WithTx(context.Background(), db, func(tx *sql.Tx) error {
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}

	testDriver.mu.Lock()
	committed := testDriver.commitCalled
	rolledBack := testDriver.rbCalled
	testDriver.mu.Unlock()

	if committed {
		t.Error("expected transaction not to be committed")
	}
	if !rolledBack {
		t.Error("expected transaction to be rolled back")
	}
}

func TestWithTx_PanicRollback(t *testing.T) {
	testDriver.reset()
	db, err := sql.Open("test_platform_db_driver", "")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	defer func() {
		p := recover()
		if p == nil {
			t.Fatal("expected panic to be re-thrown, but got nil")
		}
		if p != "fatal explosion" {
			t.Fatalf("expected 'fatal explosion', got %v", p)
		}

		testDriver.mu.Lock()
		committed := testDriver.commitCalled
		rolledBack := testDriver.rbCalled
		testDriver.mu.Unlock()

		if committed {
			t.Error("expected transaction not to be committed on panic")
		}
		if !rolledBack {
			t.Error("expected transaction to be rolled back on panic")
		}
	}()

	_ = WithTx(context.Background(), db, func(tx *sql.Tx) error {
		panic("fatal explosion")
	})
}
