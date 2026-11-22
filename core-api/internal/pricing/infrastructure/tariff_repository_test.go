package infrastructure

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/pricing/application"
	"github.com/swiftdrop/core-api/internal/pricing/domain"
)

func TestPostgresTariffRepository_FindApplicable_Success(t *testing.T) {
	effectiveFrom := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	effectiveUntil := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	evalTime := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	columns := []string{
		"id",
		"origin_zone_id",
		"destination_zone_id",
		"base_fee_cents",
		"per_kg_rate_cents",
		"effective_from",
		"effective_until",
		"version",
		"is_active",
		"created_at",
		"updated_at",
	}

	data := [][]driver.Value{
		{
			"rule-001",
			"ZONE_WESTERN_01",
			"ZONE_CENTRAL_01",
			int64(35000),
			int64(8000),
			effectiveFrom,
			effectiveUntil,
			int64(2),
			true,
			effectiveFrom,
			effectiveFrom,
		},
	}

	database := registerMockDB("find_applicable_success", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return &testRows{columns: columns, data: data}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresTariffRepository(database)

	tariff, err := repo.FindApplicable(
		context.Background(),
		"ZONE_WESTERN_01",
		"ZONE_CENTRAL_01",
		evalTime,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tariff.ID != "rule-001" {
		t.Errorf("expected ID rule-001, got %s", tariff.ID)
	}
	if tariff.OriginZoneID != "ZONE_WESTERN_01" {
		t.Errorf("expected origin zone ZONE_WESTERN_01, got %s", tariff.OriginZoneID)
	}
	if tariff.DestinationZoneID != "ZONE_CENTRAL_01" {
		t.Errorf("expected destination zone ZONE_CENTRAL_01, got %s", tariff.DestinationZoneID)
	}
	if tariff.BaseFee.Amount != 35000 {
		t.Errorf("expected base fee 35000, got %d", tariff.BaseFee.Amount)
	}
	if tariff.PerKgRate.Amount != 8000 {
		t.Errorf("expected per kg rate 8000, got %d", tariff.PerKgRate.Amount)
	}
	if tariff.Version != 2 {
		t.Errorf("expected version 2, got %d", tariff.Version)
	}
	if !tariff.IsActive {
		t.Error("expected tariff to be active")
	}
}

func TestPostgresTariffRepository_FindApplicable_OptionAQueryContract(t *testing.T) {
	evalTime := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	var capturedQuery string
	var capturedArgs []driver.NamedValue

	columns := []string{
		"id", "origin_zone_id", "destination_zone_id", "base_fee_cents", "per_kg_rate_cents",
		"effective_from", "effective_until", "version", "is_active", "created_at", "updated_at",
	}
	data := [][]driver.Value{
		{
			"rule-001", "ZONE_COLOMBO_01", "ZONE_GAMPAHA_01", int64(30000), int64(5000),
			evalTime.Add(-time.Hour), evalTime.Add(time.Hour), int64(1), true, evalTime, evalTime,
		},
	}

	database := registerMockDB("find_applicable_query_contract", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		capturedQuery = query
		capturedArgs = args
		return &testRows{columns: columns, data: data}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresTariffRepository(database)
	_, err := repo.FindApplicable(context.Background(), "ZONE_COLOMBO_01", "ZONE_GAMPAHA_01", evalTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Verify Option A geographic isolation: MUST NOT contain service_level filter
	normalizedQuery := strings.ToUpper(capturedQuery)
	if strings.Contains(normalizedQuery, "SERVICE_LEVEL") {
		t.Errorf("Option A violation: query must NOT filter by service_level, got: %s", capturedQuery)
	}

	// 2. Verify active tariffs only
	if !strings.Contains(normalizedQuery, "IS_ACTIVE = TRUE") {
		t.Errorf("query must enforce is_active = TRUE, got: %s", capturedQuery)
	}

	// 3. Verify temporal boundaries: effective_from <= $3 AND (effective_until IS NULL OR effective_until > $3)
	if !strings.Contains(normalizedQuery, "EFFECTIVE_FROM <=") {
		t.Errorf("query must check effective_from <=, got: %s", capturedQuery)
	}
	if !strings.Contains(normalizedQuery, "EFFECTIVE_UNTIL >") {
		t.Errorf("query must check effective_until >, got: %s", capturedQuery)
	}

	// 4. Verify versioning precedence: ORDER BY version DESC, effective_from DESC LIMIT 1
	if !strings.Contains(normalizedQuery, "ORDER BY VERSION DESC") {
		t.Errorf("query must order by version DESC, got: %s", capturedQuery)
	}
	if !strings.Contains(normalizedQuery, "LIMIT 1") {
		t.Errorf("query must include LIMIT 1, got: %s", capturedQuery)
	}

	// 5. Verify parameters passed
	if len(capturedArgs) != 3 {
		t.Fatalf("expected 3 arguments, got %d", len(capturedArgs))
	}
	if capturedArgs[0].Value != "ZONE_COLOMBO_01" {
		t.Errorf("expected arg 1 to be ZONE_COLOMBO_01, got %v", capturedArgs[0].Value)
	}
	if capturedArgs[1].Value != "ZONE_GAMPAHA_01" {
		t.Errorf("expected arg 2 to be ZONE_GAMPAHA_01, got %v", capturedArgs[1].Value)
	}
	if capturedArgs[2].Value != evalTime {
		t.Errorf("expected arg 3 to be evalTime, got %v", capturedArgs[2].Value)
	}
}

func TestPostgresTariffRepository_FindApplicable_NotFound(t *testing.T) {
	evalTime := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	database := registerMockDB("find_applicable_not_found", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		// Return empty rows to simulate sql.ErrNoRows
		return &testRows{
			columns: []string{"id"},
			data:    [][]driver.Value{},
		}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresTariffRepository(database)

	_, err := repo.FindApplicable(
		context.Background(),
		"ZONE_WESTERN_01",
		"ZONE_CENTRAL_01",
		evalTime,
	)
	if !errors.Is(err, domain.ErrTariffNotFound) {
		t.Fatalf("expected ErrTariffNotFound, got: %v", err)
	}
}

func TestPostgresTariffRepository_FindApplicable_DatabaseError(t *testing.T) {
	evalTime := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	dbErr := errors.New("connection reset by peer")

	database := registerMockDB("find_applicable_dberr", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return nil, nil, dbErr
	})
	defer database.Close()

	repo := NewPostgresTariffRepository(database)

	_, err := repo.FindApplicable(
		context.Background(),
		"ZONE_WESTERN_01",
		"ZONE_CENTRAL_01",
		evalTime,
	)
	if err == nil || !errors.Is(err, dbErr) {
		t.Fatalf("expected dbErr, got: %v", err)
	}
}

func TestPostgresTariffRepository_FindApplicable_InputValidation(t *testing.T) {
	evalTime := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	repo := NewPostgresTariffRepository(&sql.DB{})

	// Empty origin zone
	_, err := repo.FindApplicable(context.Background(), "", "ZONE_CENTRAL_01", evalTime)
	if !errors.Is(err, domain.ErrInvalidTariff) {
		t.Errorf("expected ErrInvalidTariff for empty origin zone, got: %v", err)
	}

	// Empty destination zone
	_, err = repo.FindApplicable(context.Background(), "ZONE_WESTERN_01", "   ", evalTime)
	if !errors.Is(err, domain.ErrInvalidTariff) {
		t.Errorf("expected ErrInvalidTariff for empty destination zone, got: %v", err)
	}

	// Zero timestamp
	_, err = repo.FindApplicable(context.Background(), "ZONE_WESTERN_01", "ZONE_CENTRAL_01", time.Time{})
	if !errors.Is(err, domain.ErrInvalidTariff) {
		t.Errorf("expected ErrInvalidTariff for zero evaluation timestamp, got: %v", err)
	}

	// Nil querier
	nilRepo := NewPostgresTariffRepository(nil)
	_, err = nilRepo.FindApplicable(context.Background(), "ZONE_WESTERN_01", "ZONE_CENTRAL_01", evalTime)
	if err == nil {
		t.Error("expected error for nil querier, got nil")
	}
}

func TestPostgresTariffRepository_List_Success(t *testing.T) {
	effectiveFrom := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	effectiveUntil := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	countCols := []string{"count"}
	countData := [][]driver.Value{{int64(1)}}

	listCols := []string{
		"id",
		"origin_zone_id",
		"destination_zone_id",
		"base_fee_cents",
		"per_kg_rate_cents",
		"effective_from",
		"effective_until",
		"version",
		"is_active",
		"created_at",
		"updated_at",
	}
	listData := [][]driver.Value{
		{
			"rule-001",
			"ZONE_WESTERN_01",
			"ZONE_CENTRAL_01",
			int64(35000),
			int64(8000),
			effectiveFrom,
			effectiveUntil,
			int64(2),
			true,
			effectiveFrom,
			effectiveFrom,
		},
	}

	database := registerMockDB("list_success", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		if strings.Contains(strings.ToUpper(query), "COUNT(*)") {
			return &testRows{columns: countCols, data: countData}, nil, nil
		}
		return &testRows{columns: listCols, data: listData}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresTariffRepository(database)
	rules, total, err := repo.List(context.Background(), application.ListTariffsFilter{}, 0, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ID != "rule-001" {
		t.Errorf("expected rule ID rule-001, got %s", rules[0].ID)
	}
}

func TestPostgresTariffRepository_List_Empty(t *testing.T) {
	countCols := []string{"count"}
	countData := [][]driver.Value{{int64(0)}}

	database := registerMockDB("list_empty", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return &testRows{columns: countCols, data: countData}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresTariffRepository(database)
	rules, total, err := repo.List(context.Background(), application.ListTariffsFilter{}, 0, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
	if len(rules) != 0 {
		t.Errorf("expected empty rules, got %d", len(rules))
	}
}

func TestPostgresTariffRepository_List_Filters(t *testing.T) {
	active := true
	var capturedCountQuery, capturedListQuery string
	var capturedCountArgs, capturedListArgs []driver.NamedValue

	countCols := []string{"count"}
	countData := [][]driver.Value{{int64(1)}}
	listCols := []string{
		"id", "origin_zone_id", "destination_zone_id", "base_fee_cents", "per_kg_rate_cents",
		"effective_from", "effective_until", "version", "is_active", "created_at", "updated_at",
	}
	effectiveFrom := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	listData := [][]driver.Value{
		{
			"rule-002", "ZONE_WESTERN_01", "ZONE_CENTRAL_01", int64(30000), int64(6000),
			effectiveFrom, nil, int64(1), true, effectiveFrom, effectiveFrom,
		},
	}

	database := registerMockDB("list_filters", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		if strings.Contains(strings.ToUpper(query), "COUNT(*)") {
			capturedCountQuery = query
			capturedCountArgs = args
			return &testRows{columns: countCols, data: countData}, nil, nil
		}
		capturedListQuery = query
		capturedListArgs = args
		return &testRows{columns: listCols, data: listData}, nil, nil
	})
	defer database.Close()

	repo := NewPostgresTariffRepository(database)
	filter := application.ListTariffsFilter{
		OriginZoneID:      "ZONE_WESTERN_01",
		DestinationZoneID: "ZONE_CENTRAL_01",
		ActiveOnly:        &active,
	}

	rules, total, err := repo.List(context.Background(), filter, 10, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || len(rules) != 1 {
		t.Fatalf("expected 1 result, got total=%d len=%d", total, len(rules))
	}

	if len(capturedCountArgs) != 3 {
		t.Fatalf("expected 3 args for count query, got %d", len(capturedCountArgs))
	}
	if !strings.Contains(capturedCountQuery, "origin_zone_id = $1") ||
		!strings.Contains(capturedCountQuery, "destination_zone_id = $2") ||
		!strings.Contains(capturedCountQuery, "is_active = $3") {
		t.Errorf("count query missing expected filter clauses: %s", capturedCountQuery)
	}

	if !strings.Contains(capturedListQuery, "LIMIT $4 OFFSET $5") {
		t.Errorf("list query missing LIMIT/OFFSET: %s", capturedListQuery)
	}

	if len(capturedListArgs) != 5 {
		t.Fatalf("expected 5 args for list query, got %d", len(capturedListArgs))
	}
	if capturedListArgs[3].Value != int64(50) || capturedListArgs[4].Value != int64(10) {
		t.Errorf("expected limit=50 offset=10, got limit=%v offset=%v", capturedListArgs[3].Value, capturedListArgs[4].Value)
	}
}

func TestPostgresTariffRepository_List_Errors(t *testing.T) {
	// Nil querier
	nilRepo := NewPostgresTariffRepository(nil)
	_, _, err := nilRepo.List(context.Background(), application.ListTariffsFilter{}, 0, 20)
	if err == nil {
		t.Error("expected error for nil querier")
	}

	// Count error
	dbErr := errors.New("db count failed")
	database := registerMockDB("list_err", func(query string, args []driver.NamedValue) (driver.Rows, driver.Result, error) {
		return nil, nil, dbErr
	})
	defer database.Close()

	repo := NewPostgresTariffRepository(database)
	_, _, err = repo.List(context.Background(), application.ListTariffsFilter{}, 0, 20)
	if !errors.Is(err, dbErr) {
		t.Errorf("expected dbErr, got %v", err)
	}
}
