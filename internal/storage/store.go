package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrLockBusy reports that an exclusive instance lock is held elsewhere. Open
// treats it as a signal to downgrade rather than as a failure.
var ErrLockBusy = errors.New("storage: instance lock busy")

// Mode is how this instance opened the database. It is decided by which locks
// were taken, never by caller preference.
type Mode string

const (
	// ModeReadWrite is the single writer instance. It may migrate and write.
	ModeReadWrite Mode = "read_write"
	// ModeReadOnly is a degraded instance. The connection layer refuses writes
	// and migrations do not run (docs/07 §7.5).
	ModeReadOnly Mode = "read_only"
)

// Valid reports whether m belongs to the closed mode set.
func (m Mode) Valid() bool { return m == ModeReadWrite || m == ModeReadOnly }

// Instance describes what this process owns. Exactly one process holds the
// write lock; exactly one process holds the capture-owner lock. They are not
// necessarily the same process, which is why they are reported separately
// (docs/05 §5.6.2 rule 7).
type Instance struct {
	Mode Mode
	// CaptureOwner is true when this process holds the capture-owner lock.
	// Binding-layer write methods that drive capture return not_capture_owner
	// when it is false (docs/05 §5.4.1).
	CaptureOwner bool
}

// Store is an open connection to the business database together with the
// instance locks this process holds. It is the only handle to SQLite; no other
// package holds a *sql.DB (docs/05 §5.6.2 rule 1).
//
// A Store belongs to one owner and is not shared through a global. Closing it
// releases the locks, so the process that closes last leaves no trace.
type Store struct {
	db       *sql.DB
	path     string
	mode     Mode
	owner    bool
	observer Observer
	writeLk  *fileLock
	ownerLk  *fileLock
}

// Mode reports how this instance opened the database.
func (s *Store) Mode() Mode {
	if s == nil {
		return ""
	}
	return s.mode
}

// Instance reports what this process owns.
func (s *Store) Instance() Instance {
	if s == nil {
		return Instance{}
	}
	return Instance{Mode: s.mode, CaptureOwner: s.owner}
}

// Path reports the database file path. It is diagnostic context, not a handle:
// pixel and file access travels through platform.Media, never through callers
// of this package.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Close releases the connection and both instance locks. It is safe to call on
// a nil receiver and more than once.
//
// The database is closed before the locks are released so no other instance can
// take the write lock while this one still holds an open descriptor.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	var errs []error
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			errs = append(errs, wrap("close database", err))
		}
		s.db = nil
	}
	if err := s.ownerLk.release(); err != nil {
		errs = append(errs, wrap("release capture-owner lock", err))
	}
	s.ownerLk = nil
	if err := s.writeLk.release(); err != nil {
		errs = append(errs, wrap("release write lock", err))
	}
	s.writeLk = nil
	return errors.Join(errs...)
}

// observeQuery records a completed statement and reports whether it was slow.
// The wrapper calls it; repositories never do.
func (s *Store) observeQuery(op string, start time.Time, err error) {
	if s == nil || s.observer == nil {
		return
	}
	s.observer.ObserveQuery(op, time.Since(start), err)
}

// observeBusy records lock contention.
func (s *Store) observeBusy(op string) {
	if s == nil || s.observer == nil {
		return
	}
	s.observer.ObserveBusy(op)
}

// observeBreadcrumb records a low-cardinality diagnostic marker.
func (s *Store) observeBreadcrumb(name string) {
	if s == nil || s.observer == nil {
		return
	}
	s.observer.ObserveBreadcrumb(name)
}

// requireWritable returns a classified error when a write is attempted on a
// read-only instance. The connection layer would also refuse the statement;
// failing here produces a clear cause instead of a driver error.
func (s *Store) requireWritable(op string) error {
	if s.mode == ModeReadWrite {
		return nil
	}
	return newError(KindReadOnly, op)
}

// Read runs fn inside a read transaction with the read timeout applied when ctx
// carries no earlier deadline. The transaction is always rolled back: this
// helper exists for reads, and a caller needing an atomic commit uses Write.
func (s *Store) Read(ctx context.Context, op string, fn func(context.Context, *sql.Tx) error) error {
	ctx, cancel := withDefaultTimeout(ctx, readTimeout)
	defer cancel()

	start := time.Now()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return s.finishTx("begin read "+op, start, err)
	}
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback()
		return s.finishTx(op, start, err)
	}
	// A read transaction is rolled back rather than committed; nothing was
	// written, and rollback avoids taking the write lock.
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return s.finishTx("rollback read "+op, start, err)
	}
	s.observeQuery(op, start, nil)
	return nil
}

// Write runs fn inside a write transaction with the write timeout applied. The
// transaction commits only when fn returns nil, so a partial write can never
// reach disk.
func (s *Store) Write(ctx context.Context, op string, fn func(context.Context, *sql.Tx) error) error {
	if err := s.requireWritable(op); err != nil {
		return err
	}
	ctx, cancel := withDefaultTimeout(ctx, writeTimeout)
	defer cancel()

	start := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return s.finishTx("begin write "+op, start, err)
	}
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback()
		return s.finishTx(op, start, err)
	}
	if err := tx.Commit(); err != nil {
		return s.finishTx("commit "+op, start, err)
	}
	s.observeQuery(op, start, nil)
	return nil
}

// finishTx classifies a failed transaction step, reports contention, and
// returns the classified error.
func (s *Store) finishTx(op string, start time.Time, err error) error {
	classified := wrap(op, err)
	if IsKind(classified, KindBusy) {
		s.observeBusy(op)
	}
	s.observeQuery(op, start, classified)
	return classified
}

// withDefaultTimeout applies timeout when ctx has no deadline of its own, so a
// caller that passes context.Background() still gets a bounded statement.
func withDefaultTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}
