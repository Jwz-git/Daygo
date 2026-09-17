package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"time"
)

// CleanupResult reports what one recordings-cleanup pass did.
type CleanupResult struct {
	// Deleted is how many screenshots were soft-deleted and their files removed.
	Deleted int
	// FreedBytes is the file_size the deleted rows carried.
	FreedBytes int64
	// SkippedRented is how many live frames remain over the limit after the
	// pass: they are rented by an in-flight analysis batch or are a pending
	// capture's staging file, and converging must never force-delete them.
	SkippedRented int
}

// cleanupStaleFileAge gates the orphan sweep. A file under the recordings root
// that no database row references is a crash leftover of the two-phase delete;
// one hour of age keeps the sweep away from any window where the recorder is
// still writing a frame whose pending intent has not landed yet.
const cleanupStaleFileAge = time.Hour

// CleanupRecordings brings the recordings directory under limitBytes by
// deleting the oldest frames first — the DB-9 boundary (docs/08 §8.4) and the
// rules of docs/03 §3.6:
//
//   - usage is SUM(file_size) over live screenshots rows, never a directory
//     scan (03 §3.4: file_size is the per-frame share of storage);
//   - a pending capture's staging file is the ACTIVE segment and is never a
//     candidate, nor is any frame rented by a pending or processing batch;
//   - deletion is two-phase so a crash cannot leave a live row pointing at a
//     missing file: the rows are soft-deleted first (the intent), then the
//     files go outside any transaction;
//   - timeline cards are untouched — after cleanup the user still sees what
//     they did in that span, just with no frames to view.
//
// In the current pipeline every frame is a single-frame segment (one JPEG per
// screenshots row), so per-row cleanup IS per-segment cleanup; when the
// segment builder lands, this migrates to per-segment without changing the
// boundary rules. limitBytes <= 0 means unlimited and is a no-op.
func (s *Store) CleanupRecordings(ctx context.Context, root string, limitBytes int64) (CleanupResult, error) {
	var result CleanupResult
	if s == nil {
		return result, newError(KindEnvironment, "cleanup: no store")
	}
	if err := s.requireWritable("cleanup"); err != nil {
		return result, err
	}
	if limitBytes <= 0 {
		// 0 is the documented "no limit": nothing to enforce.
		return result, nil
	}

	usage, err := s.recordingsUsage(ctx)
	if err != nil {
		return result, err
	}
	if usage <= limitBytes {
		return result, nil
	}

	candidates, err := s.cleanupCandidates(ctx)
	if err != nil {
		return result, err
	}

	// Oldest first, stopping the moment the limit would be met.
	var selected []cleanupSegment
	freed := int64(0)
	for _, seg := range candidates {
		if usage-freed <= limitBytes {
			break
		}
		selected = append(selected, seg)
		freed += seg.fileSize
	}
	result.SkippedRented = len(candidates) - len(selected)
	if len(selected) == 0 {
		// Everything old enough to delete is rented or active: converge as far
		// as the boundary allows and report rather than force.
		result.SkippedRented, err = s.remainingOverLimit(ctx)
		return result, err
	}

	// Phase 1: the intent. Soft-delete exactly these rows, re-asserting
	// is_deleted = 0 so a concurrent pass cannot double-delete; only rows the
	// update actually touched count toward the result.
	deleted, freed, err := s.softDeleteSegments(ctx, selected)
	if err != nil {
		return result, err
	}
	result.Deleted = deleted
	result.FreedBytes = freed

	// Phase 2: the files, outside any transaction. A missing file is fine —
	// the row is already soft-deleted either way, and a removal failure only
	// leaves an orphan the next pass's sweep will catch.
	for _, seg := range selected {
		if seg.segmentPath == "" {
			continue
		}
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(seg.segmentPath))); err != nil && !os.IsNotExist(err) {
			continue
		}
	}
	if err := s.sweepOrphanFiles(ctx, root); err != nil {
		return result, err
	}
	// After the pass, report how much is STILL over the limit: those are the
	// frames the boundary protected (rented or active), not an under-count.
	usage, err = s.recordingsUsage(ctx)
	if err != nil {
		return result, err
	}
	if usage > limitBytes {
		if result.SkippedRented, err = s.remainingOverLimit(ctx); err != nil {
			return result, err
		}
	}
	return result, nil
}

// remainingOverLimit counts live frames left when usage is still over the
// limit — the frames the exclusion rules protected.
func (s *Store) remainingOverLimit(ctx context.Context) (int, error) {
	var n int
	err := s.Read(ctx, "cleanup remaining", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM screenshots WHERE is_deleted = 0").Scan(&n)
	})
	return n, err
}

type cleanupSegment struct {
	segmentPath string
	oldestAt    int64
	fileSize    int64
}

// recordingsUsage mirrors Stats.RecordingsBytes: SUM over live rows.
func (s *Store) recordingsUsage(ctx context.Context) (int64, error) {
	var usage int64
	err := s.Read(ctx, "cleanup usage", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			"SELECT COALESCE(SUM(file_size), 0) FROM screenshots WHERE is_deleted = 0",
		).Scan(&usage)
	})
	return usage, err
}

// cleanupCandidates lists deletable segments oldest-first: live rows grouped by
// segment_path that are neither a pending capture's active segment nor rented by
// a pending or processing analysis batch.
func (s *Store) cleanupCandidates(ctx context.Context) ([]cleanupSegment, error) {
	var out []cleanupSegment
	err := s.Read(ctx, "cleanup candidates", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT s.segment_path, MIN(s.captured_at) AS oldest_at, SUM(s.file_size) AS total_size
			FROM screenshots s
			WHERE s.is_deleted = 0
			  AND s.file_size IS NOT NULL
			  AND NOT EXISTS (
			      SELECT 1 FROM pending_captures p
			      WHERE p.state = 'pending' AND p.relative_path = s.segment_path)
			  AND NOT EXISTS (
			      SELECT 1 FROM batch_screenshots bs
			      JOIN analysis_batches b ON b.id = bs.batch_id
			      JOIN screenshots s2 ON s2.id = bs.screenshot_id
			      WHERE s2.segment_path = s.segment_path AND b.status IN ('pending', 'processing'))
			GROUP BY s.segment_path
			ORDER BY oldest_at ASC, s.segment_path ASC`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var seg cleanupSegment
			if err := rows.Scan(&seg.segmentPath, &seg.oldestAt, &seg.fileSize); err != nil {
				return wrap("scan cleanup candidate", err)
			}
			out = append(out, seg)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Store) softDeleteSegments(ctx context.Context, segments []cleanupSegment) (deleted int, freed int64, err error) {
	err = s.Write(ctx, "cleanup soft-delete segments", func(ctx context.Context, tx *sql.Tx) error {
		for _, seg := range segments {
			var segFreed int64
			if err := tx.QueryRowContext(ctx,
				"SELECT COALESCE(SUM(file_size), 0) FROM screenshots WHERE segment_path = ? AND is_deleted = 0", seg.segmentPath).Scan(&segFreed); err != nil {
				return wrap("sum segment file size", err)
			}
			res, execErr := tx.ExecContext(ctx,
				"UPDATE screenshots SET is_deleted = 1 WHERE segment_path = ? AND is_deleted = 0", seg.segmentPath)
			if execErr != nil {
				return wrap("soft-delete segment", execErr)
			}
			if n, rowsErr := res.RowsAffected(); rowsErr == nil && n > 0 {
				deleted += int(n)
				freed += segFreed
			}
		}
		return nil
	})
	return deleted, freed, err
}

// sweepOrphanFiles removes files under root that no database row claims
// anymore — crash leftovers from a cleanup that died between its phases, or
// from a deleted row whose file removal never ran. Files a pending capture
// may still be writing, and anything young, are left alone. Best-effort: one
// unreadable entry never fails the pass, the next one retries.
func (s *Store) sweepOrphanFiles(ctx context.Context, root string) error {
	referenced := make(map[string]bool)
	if err := s.Read(ctx, "cleanup sweep refs", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			"SELECT segment_path FROM screenshots WHERE is_deleted = 0")
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				return wrap("scan sweep refs", err)
			}
			referenced[p] = true
		}
		if err := rows.Err(); err != nil {
			return err
		}
		pending, err := tx.QueryContext(ctx,
			"SELECT relative_path FROM pending_captures WHERE state = 'pending'")
		if err != nil {
			return err
		}
		defer func() { _ = pending.Close() }()
		for pending.Next() {
			var p string
			if err := pending.Scan(&p); err != nil {
				return wrap("scan sweep pending", err)
			}
			referenced[p] = true
		}
		return pending.Err()
	}); err != nil {
		return err
	}

	staleBefore := s.now().Add(-cleanupStaleFileAge)
	return filepath.WalkDir(root, func(p string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return nil
		}
		if referenced[filepath.ToSlash(rel)] {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil || info.ModTime().After(staleBefore) {
			return nil
		}
		_ = ctx
		_ = os.Remove(p)
		return nil
	})
}
