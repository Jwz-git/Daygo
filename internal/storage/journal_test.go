package storage

import (
	"context"
	"database/sql"
	"testing"
)

func TestJournalUpsertRoundTripPreservesSummary(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()

	intent := "ship the weekly slice"
	goals := "close two tickets"
	entry, found, err := store.Journal().Get(ctx, "2026-09-12")
	if err != nil {
		t.Fatalf("get before insert: %v", err)
	}
	if found {
		t.Fatal("empty database reported a journal entry")
	}
	if entry.Day != "" || entry.Intentions != nil || entry.Notes != nil ||
		entry.Goals != nil || entry.Reflections != nil || entry.Summary != nil || entry.Status != "" {
		t.Fatalf("zero entry = %+v, want empty fields", entry)
	}

	if err := store.Journal().Upsert(ctx, JournalEntry{
		Day: "2026-09-12", Intentions: &intent, Goals: &goals,
		Status: JournalStatusIntentionsSet,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Simulate the (future) AI summary write directly; user saves must never
	// clobber it.
	summary := "AI generated summary"
	err = store.Write(ctx, "seed journal summary", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE journal_entries SET summary = ? WHERE day = ?`, summary, "2026-09-12")
		return err
	})
	if err != nil {
		t.Fatalf("seed summary: %v", err)
	}

	if err := store.Journal().Upsert(ctx, JournalEntry{
		Day: "2026-09-12", Status: JournalStatusComplete,
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, found, err := store.Journal().Get(ctx, "2026-09-12")
	if err != nil || !found {
		t.Fatalf("get after upserts = %v, %v", found, err)
	}
	if got.Intentions != nil {
		t.Fatalf("intentions = %q after second save, want cleared to NULL", *got.Intentions)
	}
	if got.Goals != nil {
		t.Fatalf("goals = %q after second save, want cleared to NULL", *got.Goals)
	}
	if got.Status != JournalStatusComplete {
		t.Fatalf("status = %q, want complete", got.Status)
	}
	if got.Summary == nil || *got.Summary != summary {
		t.Fatalf("summary = %v after user save, want the AI value preserved", got.Summary)
	}
}
