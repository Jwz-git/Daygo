package storage

import (
	"context"
	"testing"
)

func TestJournalUpsertRoundTripClearsOmittedFields(t *testing.T) {
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
		entry.Goals != nil || entry.Reflections != nil || entry.Status != "" {
		t.Fatalf("zero entry = %+v, want empty fields", entry)
	}

	if err := store.Journal().Upsert(ctx, JournalEntry{
		Day: "2026-09-12", Intentions: &intent, Goals: &goals,
		Status: JournalStatusIntentionsSet,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// A second save omitting intentions/goals clears them to NULL.
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
}
