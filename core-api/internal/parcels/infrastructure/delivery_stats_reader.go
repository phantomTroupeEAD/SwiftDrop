package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/swiftdrop/core-api/internal/platform/db"
)

// DailyDeliveryCount is one Sri Lanka day of delivery outcomes, read from
// parcel_status_history. Failed counts failed delivery attempts, so a parcel that
// failed twice on one day counts twice.
type DailyDeliveryCount struct {
	Date      string // Asia/Colombo day, YYYY-MM-DD
	Delivered int
	Failed    int
	Returned  int
}

// DeliveryStatsReader is the parcels module's read-only view of delivery outcomes
// per day. Other modules (the admin delivery report) use it instead of running SQL
// on the parcels tables themselves.
type DeliveryStatsReader struct {
	querier db.Querier
}

// NewDeliveryStatsReader constructs a DeliveryStatsReader.
func NewDeliveryStatsReader(querier db.Querier) *DeliveryStatsReader {
	return &DeliveryStatsReader{querier: querier}
}

// Sri Lanka is UTC+05:30 all year (no daylight saving), so the day is computed with a
// fixed offset. This avoids depending on the database's time zone files.
const dailyDeliveryCountsQuery = `
	SELECT to_char((occurred_at AT TIME ZONE 'UTC') + INTERVAL '5 hours 30 minutes', 'YYYY-MM-DD') AS day,
	       count(*) FILTER (WHERE to_status = 'DELIVERED')      AS delivered,
	       count(*) FILTER (WHERE to_status = 'ATTEMPT_FAILED') AS failed,
	       count(*) FILTER (WHERE to_status = 'RETURNED')       AS returned
	FROM parcel_status_history
	WHERE occurred_at >= $1 AND occurred_at < $2
	  AND to_status IN ('DELIVERED', 'ATTEMPT_FAILED', 'RETURNED')
	GROUP BY day
	ORDER BY day;
`

// DailyDeliveryCounts returns one row per Sri Lanka day in [from, to) that had at least
// one delivered, failed or returned parcel, oldest first. Days with no outcomes are left out.
func (r *DeliveryStatsReader) DailyDeliveryCounts(ctx context.Context, from, to time.Time) ([]DailyDeliveryCount, error) {
	if r == nil || r.querier == nil {
		return nil, errors.New("delivery stats reader: nil db querier")
	}
	rows, err := r.querier.QueryContext(ctx, dailyDeliveryCountsQuery, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("query daily delivery counts: %w", err)
	}
	defer rows.Close()

	var out []DailyDeliveryCount
	for rows.Next() {
		var d DailyDeliveryCount
		if err := rows.Scan(&d.Date, &d.Delivered, &d.Failed, &d.Returned); err != nil {
			return nil, fmt.Errorf("scan daily delivery counts: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read daily delivery counts: %w", err)
	}
	return out, nil
}
