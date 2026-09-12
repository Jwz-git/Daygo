package storage

import (
	"context"
	"database/sql"
	"time"
)

// Journal statuses are the closed set from docs/05 §5.5.2 (JournalDayDTO).
const (
	JournalStatusDraft         = "draft"
	JournalStatusIntentionsSet = "intentions_set"
	JournalStatusComplete      = "complete"
)

// JournalEntry is one row of journal_entries. The text fields are nil when the
// column is NULL; Summary is AI-generated and read-only for users.
type JournalEntry struct {
	Day         string // logical day yyyy-MM-dd
	Intentions  *string
	Notes       *string
	Goals       *string
	Reflections *string
	Summary     *string
	Status      string
	UpdatedAt   time.Time
}

// JournalRepo is the typed access layer over journal_entries (docs/03 §3.3.4).
type JournalRepo struct {
	store *Store
}

// Journal returns the repository bound to this store.
func (s *Store) Journal() *JournalRepo {
	if s == nil {
		return nil
	}
	return &JournalRepo{store: s}
}

// Get returns the journal entry for one logical day. found is false when no
// row exists; that is an empty draft, not an error.
func (r *JournalRepo) Get(ctx context.Context, day string) (JournalEntry, bool, error) {
	var entry JournalEntry
	var updatedAt int64
	found := false
	err := r.store.Read(ctx, "journal get", func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`SELECT day, intentions, notes, goals, reflections, summary, status, updated_at
			 FROM journal_entries WHERE day = ?`, day)
		if err := row.Scan(&entry.Day, &entry.Intentions, &entry.Notes, &entry.Goals,
			&entry.Reflections, &entry.Summary, &entry.Status, &updatedAt); err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}
		found = true
		return nil
	})
	if err != nil {
		return JournalEntry{}, false, err
	}
	entry.UpdatedAt = time.Unix(updatedAt, 0)
	return entry, found, nil
}

// Upsert inserts or replaces the user-editable fields of one day's entry.
// Summary is intentionally not written: it is AI-generated and user-read-only,
// so a user save must never clobber it. The summary generation slice will add
// its own write path.
func (r *JournalRepo) Upsert(ctx context.Context, entry JournalEntry) error {
	at := r.store.now().Unix()
	return r.store.Write(ctx, "journal upsert", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO journal_entries (day, intentions, notes, goals, reflections, summary, status, updated_at)
			VALUES (?, ?, ?, ?, ?, NULL, ?, ?)
			ON CONFLICT(day) DO UPDATE SET
				intentions = excluded.intentions,
				notes      = excluded.notes,
				goals      = excluded.goals,
				reflections = excluded.reflections,
				status     = excluded.status,
				updated_at = excluded.updated_at`,
			entry.Day, entry.Intentions, entry.Notes, entry.Goals, entry.Reflections,
			entry.Status, at)
		return err
	})
}
