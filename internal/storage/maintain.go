package storage

import (
	"context"
	"path/filepath"
	"time"
)

// Maintenance intervals from docs/03 §3.6.
const (
	checkpointInterval      = 300 * time.Second
	backupInitialDelay      = time.Hour
	backupInterval          = 24 * time.Hour
	maintenanceHourlyPeriod = time.Hour
)

// Maintainer runs periodic storage maintenance. It owns a goroutine, so it must
// be started and stopped explicitly: docs/modules/data.md requires the
// goroutine to be owned by the app lifecycle and cancellable on exit, with no
// global database singleton.
type Maintainer struct {
	store           *Store
	backupAt        string
	retain          int
	observer        Observer
	recordingsRoot  string
	recordingsLimit func() int64

	// now is injectable so tests can drive the schedule without waiting.
	now func() time.Time
}

// MaintainerOptions configures Run.
type MaintainerOptions struct {
	// RecordingsRoot is the recordings directory the cleanup pass trims.
	// Empty disables recording cleanup.
	RecordingsRoot string
	// RecordingsLimit returns storage.recordingsLimitBytes live. A nil or
	// zero-returning source means "no limit" and skips the pass: 0 is the
	// documented unlimited, and a failed settings read must fail toward NOT
	// deleting user files.
	RecordingsLimit func() int64
	// BackupDir is where backups are written. Empty disables backup.
	BackupDir string
	// BackupRetention is how many backups to keep. Zero means
	// DefaultBackupRetention.
	BackupRetention int
	// Observer receives maintenance breadcrumbs and errors.
	Observer Observer
	// Now overrides the clock. Nil means time.Now.
	Now func() time.Time
}

// NewMaintainer builds a maintainer for a store.
func NewMaintainer(store *Store, opts MaintainerOptions) *Maintainer {
	observer := opts.Observer
	if observer == nil {
		observer = NopObserver{}
	}
	retain := opts.BackupRetention
	if retain <= 0 {
		retain = DefaultBackupRetention
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Maintainer{
		store:           store,
		backupAt:        opts.BackupDir,
		retain:          retain,
		observer:        observer,
		recordingsRoot:  opts.RecordingsRoot,
		recordingsLimit: opts.RecordingsLimit,
		now:             now,
	}
}

// Run performs maintenance until ctx is cancelled. It blocks, so callers run it
// in their own goroutine and are responsible for cancelling ctx at shutdown.
//
// The schedule follows docs/03 §3.6: checkpoint every 300 seconds, and backups
// one hour after startup and every 24 hours after that.
//
// Recording cleanup (§3.6's third row) runs on the hourly tick. It works at
// the current pipeline's granularity — one JPEG per screenshots row, so the
// "active segment" is a pending capture's staging file — and never needs
// platform.Media: the boundaries come from the tables, per the image-storage
// decision's rule that cleanup computes from recorded bytes, not scans.
//
// A read-only instance still runs the loop. Its maintenance actions are refused
// by the store, and a refusal is reported once per action rather than treated as
// a fatal error: a second instance is an expected state, not a fault.
func (m *Maintainer) Run(ctx context.Context) {
	if m == nil || m.store == nil {
		return
	}

	checkpoint := time.NewTicker(checkpointInterval)
	defer checkpoint.Stop()

	// The hourly pass also has a one-hour initial delay, so a single ticker
	// started after that delay serves both.
	hourly := time.NewTicker(maintenanceHourlyPeriod)
	defer hourly.Stop()

	initialBackup := time.NewTimer(backupInitialDelay)
	defer initialBackup.Stop()

	lastBackup := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-checkpoint.C:
			m.runCheckpoint(ctx)
		case <-initialBackup.C:
			lastBackup = m.now()
			m.runBackup(ctx)
		case <-hourly.C:
			// Cleanup runs hourly per docs/03 §3.6; the same tick also backs
			// up, but only when a day has actually passed, so a restart does
			// not produce a backup per hour.
			m.runCleanup(ctx)
			if m.now().Sub(lastBackup) >= backupInterval {
				lastBackup = m.now()
				m.runBackup(ctx)
			}
		}
	}
}

func (m *Maintainer) runCheckpoint(ctx context.Context) {
	if err := m.store.Checkpoint(ctx); err != nil {
		// A read-only instance refusing to checkpoint is expected; anything
		// else is worth a breadcrumb.
		if !IsKind(err, KindReadOnly) {
			m.observer.ObserveBreadcrumb("storage.checkpoint.failed")
		}
		return
	}
	m.observer.ObserveBreadcrumb("storage.checkpoint.ok")
}

func (m *Maintainer) runBackup(ctx context.Context) {
	if m.backupAt == "" {
		return
	}
	dir := filepath.Join(m.backupAt, BackupDirName)
	if _, err := m.store.Backup(ctx, dir, m.retain); err != nil {
		if !IsKind(err, KindReadOnly) {
			m.observer.ObserveBreadcrumb("storage.backup.failed")
		}
		return
	}
	m.observer.ObserveBreadcrumb("storage.backup.ok")
}

func (m *Maintainer) runCleanup(ctx context.Context) {
	if m.recordingsRoot == "" || m.recordingsLimit == nil {
		return
	}
	limit := m.recordingsLimit()
	if limit <= 0 {
		return
	}
	result, err := m.store.CleanupRecordings(ctx, m.recordingsRoot, limit)
	if err != nil {
		if !IsKind(err, KindReadOnly) {
			m.observer.ObserveBreadcrumb("storage.cleanup.failed")
		}
		return
	}
	if result.Deleted > 0 {
		m.observer.ObserveBreadcrumb("storage.cleanup.ok")
	}
}
