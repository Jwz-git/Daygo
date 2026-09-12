package storage

import (
	"context"
	"database/sql"
	"time"
)

// CategoryMinutes is one row of the per-category minute aggregation: the
// summed overlap of a category's cards with a window, plus the category's
// is_idle flag resolved from the categories table.
type CategoryMinutes struct {
	Name    string
	IsIdle  bool
	Minutes float64
}

// CategoryMinutesInRange aggregates non-deleted card minutes per category for
// cards overlapping [from, to), using the same overlap predicate as
// TotalMinutesTracked (docs/03 §3.5). Like that query it does not clip card
// spans to the window. Categories without a categories row (stale names after
// a rename race) still aggregate, with IsIdle false.
func (r *CardRepo) CategoryMinutesInRange(ctx context.Context, from, to time.Time) ([]CategoryMinutes, error) {
	var out []CategoryMinutes
	err := r.store.Read(ctx, "category minutes in range", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT c.category,
			       COALESCE(cat.is_idle, 0),
			       SUM(CASE WHEN c.end_ts > c.start_ts THEN (c.end_ts - c.start_ts) ELSE 0 END) / 60.0
			FROM timeline_cards c
			LEFT JOIN categories cat ON cat.name = c.category
			WHERE ((c.start_ts < ? AND c.end_ts > ?) OR (c.start_ts >= ? AND c.start_ts < ?))
			  AND c.is_deleted = 0
			GROUP BY c.category, COALESCE(cat.is_idle, 0)
			ORDER BY c.category`,
			to.Unix(), from.Unix(), from.Unix(), to.Unix())
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var row CategoryMinutes
			var isIdle int
			if err := rows.Scan(&row.Name, &isIdle, &row.Minutes); err != nil {
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

// FailedBatch is one analysis_batches row in a non-success terminal state,
// flattened for the timeline failure panel (docs/05 §5.5.2).
type FailedBatch struct {
	ID          int64
	StartTs     int64
	EndTs       int64
	Status      string
	FailureKind string
	FailureNote string
}

// FailedBatchesInRange returns failed batches overlapping [from, to). The
// statuses are the non-success terminal states of the batch state machine
// (docs/03 §3.3.1); pending and processing batches are reported separately as
// processing ranges by the pipeline slice, not here.
func (r *CardRepo) FailedBatchesInRange(ctx context.Context, from, to time.Time) ([]FailedBatch, error) {
	var out []FailedBatch
	err := r.store.Read(ctx, "failed batches in range", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, start_ts, end_ts, status,
			       COALESCE(failure_kind, ''), COALESCE(failure_note, '')
			FROM analysis_batches
			WHERE status IN ('failed', 'failed_empty', 'skipped_short')
			  AND start_ts < ? AND end_ts > ?
			ORDER BY start_ts`,
			to.Unix(), from.Unix())
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var row FailedBatch
			if err := rows.Scan(&row.ID, &row.StartTs, &row.EndTs, &row.Status, &row.FailureKind, &row.FailureNote); err != nil {
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
