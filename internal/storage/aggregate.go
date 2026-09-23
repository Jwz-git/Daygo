package storage

import (
	"context"
	"database/sql"
	"sort"
	"time"

	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// CategoryMinutes is one row of the per-category minute aggregation: the
// summed overlap of a category's cards with a window, plus the category's
// is_idle flag and color resolved from the categories table.
type CategoryMinutes struct {
	Name     string
	IsIdle   bool
	ColorHex string
	Minutes  float64
}

// CategoryMinutesInRange aggregates non-deleted card minutes per category for
// cards overlapping [from, to). Each card contributes only its intersection
// with the window, so adjacent days or weeks never count the same minutes.
// Categories without a categories row (stale names after
// a rename race) still aggregate, with IsIdle false and an empty color.
func (r *CardRepo) CategoryMinutesInRange(ctx context.Context, from, to time.Time) ([]CategoryMinutes, error) {
	var out []CategoryMinutes
	err := r.store.Read(ctx, "category minutes in range", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT c.category,
			       COALESCE(cat.is_idle, 0),
			       COALESCE(cat.color_hex, ''),
			       SUM(MIN(c.end_ts, ?) - MAX(c.start_ts, ?)) / 60.0
			FROM timeline_cards c
			LEFT JOIN categories cat ON cat.name = c.category
			WHERE c.start_ts < ? AND c.end_ts > ?
			  AND c.is_deleted = 0
			  AND c.end_ts > c.start_ts
			  AND (c.end_ts - c.start_ts) <= 14400
			GROUP BY c.category, COALESCE(cat.is_idle, 0), COALESCE(cat.color_hex, '')
			ORDER BY c.category`,
			to.Unix(), from.Unix(), to.Unix(), from.Unix())
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var row CategoryMinutes
			var isIdle int
			if err := rows.Scan(&row.Name, &isIdle, &row.ColorHex, &row.Minutes); err != nil {
				return err
			}
			row.IsIdle = isIdle != 0
			out = append(out, row)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CardSpan is one non-deleted card's time span with the category flags the
// weekly detail charts need: the card's logical day, its timestamps, and the
// category name and color resolved against the categories table.
type CardSpan struct {
	Day      string
	StartTs  int64
	EndTs    int64
	Category string
	ColorHex string
	IsIdle   bool
}

// CardSpansInRange returns card intersections with [from, to), split at each
// local 4 AM boundary and ordered by (day, start_ts). System placeholder
// cards are included: callers (insight) own the exclusion policy. The
// overlap predicate matches CategoryMinutesInRange.
func (r *CardRepo) CardSpansInRange(ctx context.Context, from, to time.Time) ([]CardSpan, error) {
	var out []CardSpan
	err := r.store.Read(ctx, "card spans in range", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT c.day, c.start_ts, c.end_ts, c.category, COALESCE(cat.color_hex, ''), COALESCE(cat.is_idle, 0)
			FROM timeline_cards c
			LEFT JOIN categories cat ON cat.name = c.category
			WHERE c.start_ts < ? AND c.end_ts > ?
			  AND c.is_deleted = 0
			  AND c.end_ts > c.start_ts
			  AND (c.end_ts - c.start_ts) <= 14400
			ORDER BY c.day, c.start_ts`,
			to.Unix(), from.Unix())
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var row CardSpan
			var isIdle int
			if err := rows.Scan(&row.Day, &row.StartTs, &row.EndTs, &row.Category, &row.ColorHex, &isIdle); err != nil {
				return err
			}
			row.IsIdle = isIdle != 0
			start := max(row.StartTs, from.Unix())
			end := min(row.EndTs, to.Unix())
			for start < end {
				day := timeutil.LogicalDay(time.Unix(start, 0), r.store.location())
				_, dayEnd, err := timeutil.DayWindow(day, r.store.location())
				if err != nil {
					return err
				}
				partEnd := min(end, dayEnd.Unix())
				row.Day, row.StartTs, row.EndTs = day, start, partEnd
				out = append(out, row)
				start = partEnd
			}
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	// Query order is by persisted start day. A card crossing the boundary can
	// place its second slice before a later card of the first day.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Day != out[j].Day {
			return out[i].Day < out[j].Day
		}
		return out[i].StartTs < out[j].StartTs
	})
	return out, nil
}

// FailedBatch is one analysis_batches row in a non-success terminal state,
// flattened for the timeline failure panel (docs/05 §5.5.2).
type FailedBatch struct {
	ID          int64
	StartTs     int64
	EndTs       int64
	Status      string
	FailureKind string
	FailureNote string
	Attempts    int
}

// FailedBatchesInRange returns failed batches overlapping [from, to). The
// statuses are the failed terminal states of the batch state machine
// (docs/03 §3.3.1) — pending and processing batches are reported separately
// as processing ranges by the pipeline slice, not here. skipped_short is a
// normal outcome, not a failure, so it does not appear in the failure panel.
func (r *CardRepo) FailedBatchesInRange(ctx context.Context, from, to time.Time) ([]FailedBatch, error) {
	var out []FailedBatch
	err := r.store.Read(ctx, "failed batches in range", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, start_ts, end_ts, status,
			       COALESCE(failure_kind, ''), COALESCE(failure_note, ''), attempts
			FROM analysis_batches
			WHERE status IN ('failed', 'failed_empty')
			  AND is_deleted = 0
			  AND start_ts < ? AND end_ts > ?
			ORDER BY start_ts`,
			to.Unix(), from.Unix())
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var row FailedBatch
			if err := rows.Scan(&row.ID, &row.StartTs, &row.EndTs, &row.Status, &row.FailureKind, &row.FailureNote, &row.Attempts); err != nil {
				return err
			}
			out = append(out, row)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
