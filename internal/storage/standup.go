package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// DailyStandupEntry is one row of daily_standup_entries.
type DailyStandupEntry struct {
	StandupDay      string // calendar day yyyy-MM-dd
	HighlightsTitle string
	Highlights      []string
	TasksTitle      string
	Tasks           []string
	BlockersTitle   string
	BlockersBody    string
	GeneratedAt     time.Time
}

// StandupRepo is the typed access layer over daily_standup_entries.
type StandupRepo struct {
	store *Store
}

// Standup returns the repository bound to this store.
func (s *Store) Standup() *StandupRepo {
	if s == nil {
		return nil
	}
	return &StandupRepo{store: s}
}

// Get returns the standup entry for one calendar day. found is false when no
// row exists; that is a not-found, not an error.
func (r *StandupRepo) Get(ctx context.Context, standupDay string) (DailyStandupEntry, bool, error) {
	var entry DailyStandupEntry
	var highlightsJSON, tasksJSON string
	var generatedAt int64
	found := false
	err := r.store.Read(ctx, "standup get", func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`SELECT standup_day, highlights_title, highlights, tasks_title, tasks,
			        blockers_title, blockers_body, generated_at
			 FROM daily_standup_entries WHERE standup_day = ?`, standupDay)
		if err := row.Scan(&entry.StandupDay, &entry.HighlightsTitle, &highlightsJSON,
			&entry.TasksTitle, &tasksJSON, &entry.BlockersTitle, &entry.BlockersBody,
			&generatedAt); err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}
		found = true
		return nil
	})
	if err != nil {
		return DailyStandupEntry{}, false, err
	}
	if !found {
		return DailyStandupEntry{}, false, nil
	}
	if err := json.Unmarshal([]byte(highlightsJSON), &entry.Highlights); err != nil {
		return DailyStandupEntry{}, false, err
	}
	if err := json.Unmarshal([]byte(tasksJSON), &entry.Tasks); err != nil {
		return DailyStandupEntry{}, false, err
	}
	entry.GeneratedAt = time.Unix(generatedAt, 0)
	return entry, true, nil
}

// ExistingDays returns the set of calendar days that already have a standup
// entry. The backfill sweep uses it to skip generated days in one query rather
// than probing Get per candidate day.
func (r *StandupRepo) ExistingDays(ctx context.Context) (map[string]bool, error) {
	days := make(map[string]bool)
	err := r.store.Read(ctx, "standup existing days", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT standup_day FROM daily_standup_entries`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var day string
			if err := rows.Scan(&day); err != nil {
				return err
			}
			days[day] = true
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return days, nil
}

// Upsert inserts or replaces one day's standup entry.
func (r *StandupRepo) Upsert(ctx context.Context, entry DailyStandupEntry) error {
	highlightsJSON, err := json.Marshal(entry.Highlights)
	if err != nil {
		return wrap("marshal highlights", err)
	}
	tasksJSON, err := json.Marshal(entry.Tasks)
	if err != nil {
		return wrap("marshal tasks", err)
	}
	at := entry.GeneratedAt.Unix()
	return r.store.Write(ctx, "standup upsert", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO daily_standup_entries
				(standup_day, highlights_title, highlights, tasks_title, tasks,
				 blockers_title, blockers_body, generated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(standup_day) DO UPDATE SET
				highlights_title = excluded.highlights_title,
				highlights       = excluded.highlights,
				tasks_title      = excluded.tasks_title,
				tasks            = excluded.tasks,
				blockers_title   = excluded.blockers_title,
				blockers_body    = excluded.blockers_body,
				generated_at     = excluded.generated_at`,
			entry.StandupDay, entry.HighlightsTitle, highlightsJSON,
			entry.TasksTitle, tasksJSON, entry.BlockersTitle, entry.BlockersBody, at)
		return err
	})
}
