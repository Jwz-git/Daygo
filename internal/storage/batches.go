package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// BatchStatus is the closed enum of analysis_batches.status (docs/03 §3.3.1).
// succeeded is the ONLY success terminal state; a batch that produced no
// cards because the window was too short is skipped_short, and one whose
// provider returned nothing usable is failed_empty.
type BatchStatus string

const (
	BatchPending      BatchStatus = "pending"
	BatchProcessing   BatchStatus = "processing"
	BatchSucceeded    BatchStatus = "succeeded"
	BatchFailed       BatchStatus = "failed"
	BatchFailedEmpty  BatchStatus = "failed_empty"
	BatchSkippedShort BatchStatus = "skipped_short"
)

// validTransitions is the batch state machine. Every arrow here is one the
// scheduler legitimately drives; anything else is a programming error and is
// rejected with KindConstraint rather than silently accepted. The
// pending-return edges from failed states are requeue/adopt paths.
var validTransitions = map[BatchStatus][]BatchStatus{
	BatchPending:      {BatchProcessing, BatchSkippedShort},
	BatchProcessing:   {BatchSucceeded, BatchFailed, BatchFailedEmpty, BatchPending},
	BatchSucceeded:    {},
	BatchFailed:       {BatchPending},
	BatchFailedEmpty:  {BatchPending},
	BatchSkippedShort: {},
}

// Batch is one analysis_batches row.
type Batch struct {
	ID          int64
	Start, End  time.Time
	Status      BatchStatus
	FailureKind string
	FailureNote string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AnalysisFrame is a screenshots row as the scheduler sees it: identity plus
// the fields batching and idle detection consume.
type AnalysisFrame struct {
	ID          int64
	SegmentPath string
	FrameIndex  int
	CapturedAt  time.Time
	IdleSeconds *int
	Redacted    bool
	FileSize    int64
}

// AnalysisRepo implements the analysis scheduler's storage needs over
// analysis_batches, batch_screenshots, and observations. SQL lives here and
// only here; the pipeline in internal/analysis drives it through interfaces.
type AnalysisRepo struct {
	store *Store
}

// Analysis returns the repository bound to this store.
func (s *Store) Analysis() *AnalysisRepo {
	if s == nil {
		return nil
	}
	return &AnalysisRepo{store: s}
}

// UnbatchedFrames returns committed, non-deleted screenshots in [since, until)
// that belong to no batch at all. Frames of failed or skipped batches do NOT
// resurface here: retrying is the batch's job (requeue), so a re-created batch
// would otherwise duplicate the join rows.
func (r *AnalysisRepo) UnbatchedFrames(ctx context.Context, since, until time.Time) ([]AnalysisFrame, error) {
	if r == nil || r.store == nil {
		return nil, fmt.Errorf("analysis: store unavailable")
	}
	var out []AnalysisFrame
	err := r.store.Read(ctx, "analysis unbatched frames", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT s.id, s.segment_path, s.frame_index, s.captured_at, s.idle_seconds_at_capture,
			       s.redacted, s.file_size
			FROM screenshots s
			WHERE s.is_deleted = 0
			  AND s.captured_at >= ? AND s.captured_at < ?
			  AND NOT EXISTS (SELECT 1 FROM batch_screenshots bs WHERE bs.screenshot_id = s.id)
			ORDER BY s.captured_at, s.id`,
			since.Unix(), until.Unix())
		if err != nil {
			return wrap("select unbatched frames", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			f, err := scanAnalysisFrame(rows)
			if err != nil {
				return err
			}
			out = append(out, f)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateBatch inserts one analysis_batches row and its full batch_screenshots
// membership in a single transaction. status is the initial state — pending
// for batches entering the pipeline, skipped_short for ones closed below the
// minimum analysis duration.
func (r *AnalysisRepo) CreateBatch(ctx context.Context, frames []AnalysisFrame, status BatchStatus, now time.Time) (Batch, error) {
	if r == nil || r.store == nil {
		return Batch{}, fmt.Errorf("analysis: store unavailable")
	}
	if len(frames) == 0 {
		return Batch{}, newError(KindConstraint, "create batch: no frames")
	}
	if status != BatchPending && status != BatchSkippedShort {
		return Batch{}, newError(KindConstraint, fmt.Sprintf("create batch: invalid initial status %q", status))
	}

	batch := Batch{
		Start:     frames[0].CapturedAt,
		End:       frames[len(frames)-1].CapturedAt,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := r.store.Write(ctx, "analysis create batch", func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO analysis_batches (start_ts, end_ts, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)`,
			batch.Start.Unix(), batch.End.Unix(), status, now.Unix(), now.Unix())
		if err != nil {
			return wrap("insert batch", err)
		}
		batch.ID, err = res.LastInsertId()
		if err != nil {
			return wrap("insert batch id", err)
		}
		for _, f := range frames {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO batch_screenshots (batch_id, screenshot_id) VALUES (?, ?)`,
				batch.ID, f.ID); err != nil {
				return wrap("insert batch frame", err)
			}
		}
		return nil
	})
	if err != nil {
		return Batch{}, err
	}
	return batch, nil
}

// FramesForBatch returns a batch's frames in capture order.
func (r *AnalysisRepo) FramesForBatch(ctx context.Context, batchID int64) ([]AnalysisFrame, error) {
	if r == nil || r.store == nil {
		return nil, fmt.Errorf("analysis: store unavailable")
	}
	var out []AnalysisFrame
	err := r.store.Read(ctx, "analysis frames for batch", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT s.id, s.segment_path, s.frame_index, s.captured_at, s.idle_seconds_at_capture,
			       s.redacted, s.file_size
			FROM batch_screenshots bs
			JOIN screenshots s ON s.id = bs.screenshot_id
			WHERE bs.batch_id = ? AND s.is_deleted = 0
			ORDER BY s.captured_at, s.id`,
			batchID)
		if err != nil {
			return wrap("select frames for batch", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			f, err := scanAnalysisFrame(rows)
			if err != nil {
				return err
			}
			out = append(out, f)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PendingBatches returns batches waiting to enter the pipeline, oldest first.
func (r *AnalysisRepo) PendingBatches(ctx context.Context) ([]Batch, error) {
	return r.batchesByStatus(ctx, "analysis pending batches", BatchPending)
}

// ProcessingBatchesInRange returns batches currently pending or processing
// that overlap [from, to), for the timeline's processing indicator.
func (r *AnalysisRepo) ProcessingBatchesInRange(ctx context.Context, from, to time.Time) ([]Batch, error) {
	return r.batchesInRange(ctx, "analysis processing batches",
		`WHERE status IN (?, ?)
		    AND ((start_ts < ? AND end_ts > ?) OR (start_ts >= ? AND start_ts < ?))
		  ORDER BY start_ts`,
		BatchPending, BatchProcessing, to.Unix(), from.Unix(), from.Unix(), to.Unix())
}

// BatchesInRange returns batches in any state overlapping [from, to) — the
// full view for diagnostics and tests.
func (r *AnalysisRepo) BatchesInRange(ctx context.Context, from, to time.Time) ([]Batch, error) {
	return r.batchesInRange(ctx, "analysis batches in range",
		`WHERE ((start_ts < ? AND end_ts > ?) OR (start_ts >= ? AND start_ts < ?))
		  ORDER BY start_ts`,
		to.Unix(), from.Unix(), from.Unix(), to.Unix())
}

// SetBatchStatus moves a batch through the state machine. failureKind/Note
// are required for the failed states and cleared on any transition that
// leaves them. An invalid transition is a KindConstraint error, not a silent
// no-op.
func (r *AnalysisRepo) SetBatchStatus(ctx context.Context, batchID int64, to BatchStatus, failureKind, failureNote string, now time.Time) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("analysis: store unavailable")
	}
	if !to.IsValid() {
		return newError(KindConstraint, fmt.Sprintf("set batch status: unknown status %q", to))
	}
	if to == BatchFailed || to == BatchFailedEmpty {
		if failureKind == "" {
			return newError(KindConstraint, "set batch status: failed states require a failure kind")
		}
	} else if failureKind != "" || failureNote != "" {
		return newError(KindConstraint, "set batch status: failure info only allowed on failed states")
	}

	return r.store.Write(ctx, "analysis set batch status", func(ctx context.Context, tx *sql.Tx) error {
		var from BatchStatus
		if err := tx.QueryRowContext(ctx,
			`SELECT status FROM analysis_batches WHERE id = ?`, batchID).Scan(&from); err != nil {
			if err == sql.ErrNoRows {
				return newError(KindNotFound, fmt.Sprintf("set batch status: batch %d", batchID))
			}
			return wrap("select batch status", err)
		}
		allowed := validTransitions[from]
		ok := false
		for _, candidate := range allowed {
			if candidate == to {
				ok = true
				break
			}
		}
		if !ok {
			return newError(KindConstraint, fmt.Sprintf("set batch status: %s -> %s is not a valid transition", from, to))
		}
		var failureKindArg, failureNoteArg any
		if to == BatchFailed || to == BatchFailedEmpty {
			failureKindArg, failureNoteArg = failureKind, failureNote
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE analysis_batches
			SET status = ?, failure_kind = ?, failure_note = ?, updated_at = ?
			WHERE id = ?`,
			to, failureKindArg, failureNoteArg, now.Unix(), batchID)
		if err != nil {
			return wrap("update batch status", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return wrap("update batch status", err)
		}
		if n == 0 {
			return newError(KindNotFound, fmt.Sprintf("set batch status: batch %d", batchID))
		}
		return nil
	})
}

// AdoptStaleProcessing moves processing batches back to pending. The pipeline
// runs it once at startup: a batch found in processing was interrupted by a
// crash or quit, and pending is where the next run picks it up.
func (r *AnalysisRepo) AdoptStaleProcessing(ctx context.Context, now time.Time) (int, error) {
	return r.requeue(ctx, "analysis adopt stale processing",
		`UPDATE analysis_batches SET status = ?, updated_at = ? WHERE status = ?`,
		[]any{BatchPending, now.Unix(), BatchProcessing})
}

// RequeueFailed moves failed and failed_empty batches older than olderThan
// back to pending. The cooldown is measured on updated_at, which every state
// change refreshes — so the clock restarts on each failure.
func (r *AnalysisRepo) RequeueFailed(ctx context.Context, olderThan, now time.Time) (int, error) {
	return r.requeue(ctx, "analysis requeue failed",
		`UPDATE analysis_batches SET status = ?, updated_at = ?
		 WHERE status IN (?, ?) AND updated_at < ?`,
		[]any{BatchPending, now.Unix(), BatchFailed, BatchFailedEmpty, olderThan.Unix()})
}

// Observation is one frame-transcription row.
type Observation struct {
	ID          int64
	BatchID     int64
	Start, End  time.Time
	Observation string
	Metadata    string
}

// InsertObservations writes a batch's transcriptions in one transaction.
func (r *AnalysisRepo) InsertObservations(ctx context.Context, batchID int64, obs []Observation, now time.Time) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("analysis: store unavailable")
	}
	if len(obs) == 0 {
		return newError(KindConstraint, "insert observations: none given")
	}
	for _, o := range obs {
		if o.Observation == "" {
			return newError(KindConstraint, "insert observations: empty observation")
		}
	}
	return r.store.Write(ctx, "analysis insert observations", func(ctx context.Context, tx *sql.Tx) error {
		for _, o := range obs {
			var metadata any
			if o.Metadata != "" {
				metadata = o.Metadata
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO observations (batch_id, start_ts, end_ts, observation, metadata, created_at)
				VALUES (?, ?, ?, ?, ?, ?)`,
				batchID, o.Start.Unix(), o.End.Unix(), o.Observation, metadata, now.Unix()); err != nil {
				return wrap("insert observation", err)
			}
		}
		return nil
	})
}

// ObservationsForBatch returns a batch's transcriptions in start order.
func (r *AnalysisRepo) ObservationsForBatch(ctx context.Context, batchID int64) ([]Observation, error) {
	return r.observationsQuery(ctx, "analysis observations for batch",
		`WHERE batch_id = ? ORDER BY start_ts`, batchID)
}

// ObservationsInRange returns transcriptions overlapping [from, to) in start
// order — the sliding-window context the card stage reads.
func (r *AnalysisRepo) ObservationsInRange(ctx context.Context, from, to time.Time) ([]Observation, error) {
	if r == nil || r.store == nil {
		return nil, fmt.Errorf("analysis: store unavailable")
	}
	var out []Observation
	err := r.store.Read(ctx, "analysis observations in range", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, batch_id, start_ts, end_ts, observation, metadata
			FROM observations
			WHERE start_ts < ? AND end_ts > ?
			ORDER BY start_ts`,
			to.Unix(), from.Unix())
		if err != nil {
			return wrap("select observations in range", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			o, err := scanObservation(rows)
			if err != nil {
				return err
			}
			out = append(out, o)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AnalysisRepo) observationsQuery(ctx context.Context, op, where string, args ...any) ([]Observation, error) {
	if r == nil || r.store == nil {
		return nil, fmt.Errorf("analysis: store unavailable")
	}
	var out []Observation
	err := r.store.Read(ctx, op, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT id, batch_id, start_ts, end_ts, observation, metadata FROM observations `+where, args...)
		if err != nil {
			return wrap("select observations", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			o, err := scanObservation(rows)
			if err != nil {
				return err
			}
			out = append(out, o)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AnalysisRepo) batchesByStatus(ctx context.Context, op string, status BatchStatus) ([]Batch, error) {
	if r == nil || r.store == nil {
		return nil, fmt.Errorf("analysis: store unavailable")
	}
	var out []Batch
	err := r.store.Read(ctx, op, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, start_ts, end_ts, status, failure_kind, failure_note, created_at, updated_at
			FROM analysis_batches WHERE status = ? ORDER BY start_ts`, status)
		if err != nil {
			return wrap("select batches", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			b, err := scanBatch(rows)
			if err != nil {
				return err
			}
			out = append(out, b)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AnalysisRepo) batchesInRange(ctx context.Context, op, where string, args ...any) ([]Batch, error) {
	if r == nil || r.store == nil {
		return nil, fmt.Errorf("analysis: store unavailable")
	}
	var out []Batch
	err := r.store.Read(ctx, op, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT id, start_ts, end_ts, status, failure_kind, failure_note, created_at, updated_at
			 FROM analysis_batches `+where, args...)
		if err != nil {
			return wrap("select batches in range", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			b, err := scanBatch(rows)
			if err != nil {
				return err
			}
			out = append(out, b)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AnalysisRepo) requeue(ctx context.Context, op, query string, args []any) (int, error) {
	if r == nil || r.store == nil {
		return 0, fmt.Errorf("analysis: store unavailable")
	}
	var n int64
	err := r.store.Write(ctx, op, func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return wrap(op, err)
		}
		n, err = res.RowsAffected()
		return err
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// IsValid reports whether s is a member of the closed enum.
func (s BatchStatus) IsValid() bool {
	switch s {
	case BatchPending, BatchProcessing, BatchSucceeded, BatchFailed, BatchFailedEmpty, BatchSkippedShort:
		return true
	}
	return false
}

func scanAnalysisFrame(row scanner) (AnalysisFrame, error) {
	var f AnalysisFrame
	var ts int64
	var idle sql.NullInt64
	var red int
	var size sql.NullInt64
	if err := row.Scan(&f.ID, &f.SegmentPath, &f.FrameIndex, &ts, &idle, &red, &size); err != nil {
		return f, wrap("scan analysis frame", err)
	}
	f.CapturedAt = time.Unix(ts, 0)
	if idle.Valid {
		x := int(idle.Int64)
		f.IdleSeconds = &x
	}
	f.Redacted = red != 0
	f.FileSize = size.Int64
	return f, nil
}

func scanBatch(row scanner) (Batch, error) {
	var b Batch
	var start, end, created, updated int64
	var failureKind, failureNote sql.NullString
	if err := row.Scan(&b.ID, &start, &end, &b.Status, &failureKind, &failureNote, &created, &updated); err != nil {
		return b, wrap("scan batch", err)
	}
	b.Start = time.Unix(start, 0)
	b.End = time.Unix(end, 0)
	b.FailureKind = failureKind.String
	b.FailureNote = failureNote.String
	b.CreatedAt = time.Unix(created, 0)
	b.UpdatedAt = time.Unix(updated, 0)
	return b, nil
}

func scanObservation(row scanner) (Observation, error) {
	var o Observation
	var start, end int64
	var metadata sql.NullString
	if err := row.Scan(&o.ID, &o.BatchID, &start, &end, &o.Observation, &metadata); err != nil {
		return o, wrap("scan observation", err)
	}
	o.Start = time.Unix(start, 0)
	o.End = time.Unix(end, 0)
	o.Metadata = metadata.String
	return o, nil
}
