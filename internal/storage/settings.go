package storage

import (
	"context"
	"database/sql"
	"time"
)

// settingsTable is the one table db-core owns. Keys are the contract; renaming
// one needs a migration because the stored name is what other modules read
// (docs/05 §5.6.3 rule 1).
const settingsTable = "app_settings"

// SettingsRepo is the typed key/value access layer over app_settings.
//
// It deliberately does NOT normalize or clamp values. docs/05 §5.6.3 rule 2
// places normalization in internal/settings, and doing it here as well would
// create a second source of truth for what a valid value is. This layer stores
// and returns exactly what it is given.
type SettingsRepo struct {
	store *Store
}

// Settings returns the repository bound to this store.
func (s *Store) Settings() *SettingsRepo {
	if s == nil {
		return nil
	}
	return &SettingsRepo{store: s}
}

// Get returns the raw JSON value stored under key. The second result is false
// when the key has never been written; callers apply their own default rather
// than this layer inventing one, because defaults belong to the feature that
// owns the setting (docs/09 §9.5).
func (r *SettingsRepo) Get(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := r.store.Read(ctx, "settings get", func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			"SELECT value FROM "+settingsTable+" WHERE key = ?", key)
		return row.Scan(&value)
	})
	if err != nil {
		if IsKind(err, KindNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return value, true, nil
}

// GetAll returns every stored setting. Keys never written are absent from the
// result; the caller merges its defaults.
func (r *SettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string)
	err := r.store.Read(ctx, "settings get all", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, "SELECT key, value FROM "+settingsTable)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var key, value string
			if err := rows.Scan(&key, &value); err != nil {
				return err
			}
			out[key] = value
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Set writes one key. Writing the same value again updates updated_at, so a
// caller that re-sends an unchanged setting is not silently ignored.
func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	at := r.store.now()
	if err := r.store.Write(ctx, "settings set", func(ctx context.Context, tx *sql.Tx) error {
		return upsertSetting(ctx, tx, key, value, at)
	}); err != nil {
		return err
	}
	// Notify only after the transaction committed: an event for a write that
	// rolled back would send consumers to re-read a value that never changed.
	r.store.notifySettings([]string{key}, at)
	return nil
}

// SetMany applies several keys in ONE transaction. docs/03 §3.1 chooses the
// database for settings precisely so a setting can change in the same
// transaction as the data it affects; splitting a related group across
// transactions would give that up.
//
// Either every pair lands or none does. An empty map is a no-op, not an error,
// so a patch that touched nothing still succeeds.
func (r *SettingsRepo) SetMany(ctx context.Context, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	// Sorted application order keeps the transaction deterministic, which makes
	// a failing case reproducible.
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sortStrings(keys)

	at := r.store.now()
	if err := r.store.Write(ctx, "settings set many", func(ctx context.Context, tx *sql.Tx) error {
		for _, key := range keys {
			if err := upsertSetting(ctx, tx, key, values[key], at); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	r.store.notifySettings(keys, at)
	return nil
}

// Delete removes one key. Removing an absent key is not an error: the caller
// asked for the key to be gone and it is gone.
func (r *SettingsRepo) Delete(ctx context.Context, key string) error {
	at := r.store.now()
	err := r.store.Write(ctx, "settings delete", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "DELETE FROM "+settingsTable+" WHERE key = ?", key)
		return err
	})
	if err != nil {
		return err
	}
	r.store.notifySettings([]string{key}, at)
	return nil
}

// upsertSetting writes one row, replacing any previous value.
func upsertSetting(ctx context.Context, tx *sql.Tx, key, value string, now time.Time) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO `+settingsTable+` (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, now.Unix())
	return err
}

// SettingsChanged is one notification that settings were written.
type SettingsChanged struct {
	// Keys lists the keys the transaction wrote.
	Keys []string
	// At is when the write committed.
	At time.Time
}

// Watch returns a channel of change notifications.
//
// The channel is closed when ctx is done, and this implementation owns that:
// docs/05 §5.6.3 rule 3 requires consumers to select on ctx.Done() and the
// producer to close, so a consumer ranging over the channel terminates.
//
// Notifications are best-effort. A consumer that falls behind may miss
// intermediate events; it should re-read the settings rather than treat the
// channel as a complete log. A buffer keeps an ordinary burst from blocking a
// writer.
func (r *SettingsRepo) Watch(ctx context.Context) <-chan SettingsChanged {
	out := make(chan SettingsChanged, watchBuffer)
	r.store.addSubscriber(out)

	go func() {
		<-ctx.Done()
		r.store.removeSubscriber(out)
	}()

	return out
}

// watchBuffer bounds how many change events may queue before a slow consumer
// starts missing them.
const watchBuffer = 16

// sortStrings is a small insertion sort so this file does not pull in sort for
// one call. Key sets are tiny: one settings group at most.
func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
