package storage

import (
	"context"
	"database/sql"
	"os"
	"sync/atomic"
)

// Stats is the storage-side part of DiagnosticsDTO (docs/05 §5.5.2). Every
// field is a count, a size or a path — never user content, and never a value
// derived from screen data or LLM payloads (docs/07 §7.5).
type Stats struct {
	// DatabasePath is the file this store opened.
	DatabasePath string
	// DatabaseBytes is the size of the database file, excluding the WAL. The
	// WAL is reported separately because a large WAL is itself a symptom.
	DatabaseBytes int64
	// WALBytes is the size of the -wal file, or 0 when it does not exist.
	WALBytes int64
	// SkippedCards is how many clock strings this process failed to parse.
	// docs/05 §5.6.2 rule 4 makes silently dropping cards a defect, so the
	// counter is part of the contract rather than an optional extra.
	SkippedCards int64

	// RecordingsBytes sums screenshots.file_size over live rows (docs/03 §3.4).
	// An empty recording schema is an available source and reports zero.
	RecordingsBytes int64
	// RecordingsAvailable reports whether the screenshots table exists. When it
	// does not, RecordingsBytes is meaningless rather than zero.
	RecordingsAvailable bool

	// LastCaptureAtTs is the newest committed frame's timestamp, nil when no
	// frame has been committed yet. It is a pointer because "no capture has
	// happened" and "the newest capture was at the epoch" are different facts.
	LastCaptureAtTs *int64

	// PendingBatches counts batches still queued or in flight.
	PendingBatches int
	// FailedBatches counts batches that ended in a failure state and can be
	// retried (docs/03 §3.3.1).
	FailedBatches int
	// BatchesAvailable reports whether the analysis_batches table exists.
	BatchesAvailable bool
}

// Batch status values this package filters on. They mirror the closed set in
// docs/03 §3.3.1; the query strings below are the only place they appear.
const (
	pendingBatchFilter = `status IN ('pending', 'processing')`
	failedBatchFilter  = `status IN ('failed', 'failed_empty')`
)

// skippedCards counts cards dropped because their clock string could not be
// parsed. It is process-local: the pipeline that produces them is also
// process-local, and persisting the counter would need a table this slice does
// not own.
var skippedCards int64

// NoteSkippedCards records dropped cards so GetDiagnostics can report them.
// Callers must invoke it rather than silently discarding a parse failure.
func NoteSkippedCards(n int64) {
	if n <= 0 {
		return
	}
	atomic.AddInt64(&skippedCards, n)
}

// SkippedCards reports how many cards this process dropped, and resets nothing.
// "Today" is the caller's framing: the counter is process-local and a process
// restart is the natural bound.
func SkippedCards() int64 {
	return atomic.LoadInt64(&skippedCards)
}

// Stats collects the storage-side diagnostics.
//
// A missing optional table is not an error: it means the feature that owns it
// has not shipped, and the caller reports that rather than a wrong number.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	if s == nil {
		return Stats{}, newError(KindEnvironment, "stats: no store")
	}

	stats := Stats{
		DatabasePath: s.path,
		SkippedCards: SkippedCards(),
	}

	stats.DatabaseBytes = fileSize(s.path)
	stats.WALBytes = fileSize(s.path + "-wal")

	var err error
	stats.RecordingsAvailable, err = s.tableExists(ctx, "screenshots")
	if err != nil {
		return Stats{}, err
	}
	if stats.RecordingsAvailable {
		if err := s.db.QueryRowContext(ctx,
			"SELECT COALESCE(SUM(file_size), 0) FROM screenshots WHERE is_deleted = 0",
		).Scan(&stats.RecordingsBytes); err != nil {
			return Stats{}, wrap("sum recordings bytes", err)
		}

		// MAX over an empty table is NULL, which is the honest answer for "no
		// frame has been committed" and must not be flattened to zero.
		var last sql.NullInt64
		if err := s.db.QueryRowContext(ctx,
			"SELECT MAX(captured_at) FROM screenshots WHERE is_deleted = 0",
		).Scan(&last); err != nil {
			return Stats{}, wrap("read last capture", err)
		}
		if last.Valid {
			stats.LastCaptureAtTs = &last.Int64
		}
	}

	stats.BatchesAvailable, err = s.tableExists(ctx, "analysis_batches")
	if err != nil {
		return Stats{}, err
	}
	if stats.BatchesAvailable {
		if err := s.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM analysis_batches WHERE "+pendingBatchFilter,
		).Scan(&stats.PendingBatches); err != nil {
			return Stats{}, wrap("count pending batches", err)
		}
		if err := s.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM analysis_batches WHERE "+failedBatchFilter,
		).Scan(&stats.FailedBatches); err != nil {
			return Stats{}, wrap("count failed batches", err)
		}
	}

	return stats, nil
}

// tableExists reports whether a table is present. It lets a caller distinguish
// "the feature is not built yet" from "the value is genuinely zero".
func (s *Store) tableExists(ctx context.Context, table string) (bool, error) {
	var name string
	err := s.db.QueryRowContext(ctx,
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, wrap("look up table "+table, err)
	}
	return true, nil
}

// fileSize returns the size of a file, or 0 when it does not exist or cannot be
// read. A missing WAL is normal, and a size that cannot be read should not fail
// the whole diagnostics call.
func fileSize(path string) int64 {
	if path == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
