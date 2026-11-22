package infrastructure

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDeliveryStatsReader_DailyDeliveryCounts(t *testing.T) {
	from := time.Date(2026, 11, 15, 18, 30, 0, 0, time.UTC) // 2026-11-16 00:00 Colombo
	to := from.Add(7 * 24 * time.Hour)

	var gotQuery string
	var gotArgs []driver.NamedValue
	database := registerMockDB("delivery_stats_ok", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		gotQuery, gotArgs = query, args
		return &testRows{
			columns: []string{"day", "delivered", "failed", "returned"},
			data: [][]driver.Value{
				{"2026-11-17", int64(4), int64(1), int64(0)},
				{"2026-11-20", int64(2), int64(0), int64(1)},
			},
		}, nil, nil
	})
	defer database.Close()

	got, err := NewDeliveryStatsReader(database).DailyDeliveryCounts(context.Background(), from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []DailyDeliveryCount{
		{Date: "2026-11-17", Delivered: 4, Failed: 1, Returned: 0},
		{Date: "2026-11-20", Delivered: 2, Failed: 0, Returned: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	for _, part := range []string{"parcel_status_history", "'DELIVERED'", "'ATTEMPT_FAILED'", "'RETURNED'", "5 hours 30 minutes", "occurred_at >= $1 AND occurred_at < $2"} {
		if !strings.Contains(gotQuery, part) {
			t.Errorf("query is missing %q:\n%s", part, gotQuery)
		}
	}
	if len(gotArgs) != 2 || !gotArgs[0].Value.(time.Time).Equal(from) || !gotArgs[1].Value.(time.Time).Equal(to) {
		t.Errorf("unexpected args: %+v", gotArgs)
	}
}

func TestDeliveryStatsReader_NoRowsReturnsEmpty(t *testing.T) {
	database := registerMockDB("delivery_stats_empty", func(string, []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return &testRows{columns: []string{"day", "delivered", "failed", "returned"}}, nil, nil
	})
	defer database.Close()

	got, err := NewDeliveryStatsReader(database).DailyDeliveryCounts(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, %v; want no rows and no error", got, err)
	}
}

func TestDeliveryStatsReader_DBErrorIsReturned(t *testing.T) {
	database := registerMockDB("delivery_stats_err", func(string, []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return nil, nil, errors.New("connection refused")
	})
	defer database.Close()

	if _, err := NewDeliveryStatsReader(database).DailyDeliveryCounts(context.Background(), time.Now().Add(-time.Hour), time.Now()); err == nil {
		t.Fatal("expected an error when the database fails")
	}
}

func TestDeliveryStatsReader_NilQuerier(t *testing.T) {
	if _, err := NewDeliveryStatsReader(nil).DailyDeliveryCounts(context.Background(), time.Now(), time.Now()); err == nil {
		t.Fatal("expected an error for a nil querier")
	}
}
