package infrastructure

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
)

var (
	mockDriverOnce sync.Once
	mockHandlers   = make(map[string]mockHandler)
	mockMu         sync.RWMutex
)

type mockHandler func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error)

type testDriver struct{}

func (d *testDriver) Open(name string) (driver.Conn, error) {
	return &testConn{dbName: name}, nil
}

type testConn struct {
	dbName string
}

func (c *testConn) Prepare(query string) (driver.Stmt, error) {
	return &testStmt{query: query, conn: c}, nil
}

func (c *testConn) Close() error { return nil }

func (c *testConn) Begin() (driver.Tx, error) {
	return &testTx{}, nil
}

type testTx struct{}

func (t *testTx) Commit() error   { return nil }
func (t *testTx) Rollback() error { return nil }

type testStmt struct {
	query string
	conn  *testConn
}

func (s *testStmt) Close() error { return nil }

func (s *testStmt) NumInput() int { return -1 }

func (s *testStmt) Exec(args []driver.Value) (driver.Result, error) {
	named := make([]driver.NamedValue, len(args))
	for i, a := range args {
		named[i] = driver.NamedValue{Ordinal: i + 1, Value: a}
	}
	mockMu.RLock()
	handler, ok := mockHandlers[s.conn.dbName]
	mockMu.RUnlock()
	if !ok {
		return nil, errors.New("no mock handler registered")
	}
	_, res, err := handler(s.query, named)
	return res, err
}

func (s *testStmt) Query(args []driver.Value) (driver.Rows, error) {
	named := make([]driver.NamedValue, len(args))
	for i, a := range args {
		named[i] = driver.NamedValue{Ordinal: i + 1, Value: a}
	}
	mockMu.RLock()
	handler, ok := mockHandlers[s.conn.dbName]
	mockMu.RUnlock()
	if !ok {
		return nil, errors.New("no mock handler registered")
	}
	rows, _, err := handler(s.query, named)
	return rows, err
}

type testRows struct {
	columns []string
	data    [][]driver.Value
	idx     int
}

func (r *testRows) Columns() []string { return r.columns }
func (r *testRows) Close() error      { return nil }
func (r *testRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.idx])
	r.idx++
	return nil
}

type testResult struct {
	rowsAffected int64
}

func (r testResult) LastInsertId() (int64, error) { return 0, nil }
func (r testResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

func registerMockDB(name string, h mockHandler) *sql.DB {
	mockDriverOnce.Do(func() {
		sql.Register("mock_parcel_infra_driver", &testDriver{})
	})
	mockMu.Lock()
	mockHandlers[name] = h
	mockMu.Unlock()
	database, err := sql.Open("mock_parcel_infra_driver", name)
	if err != nil {
		panic(err)
	}
	return database
}
