package agentread

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/storage"
)

// seedDB creates a migrated business database at a temp path, seeds one logical
// day (2026-09-20) of journal + goal, closes the writer, and returns the file
// path. The writer is closed so the read-only reader opens a settled database.
func seedDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()

	w, err := storage.Open(ctx, storage.Options{Dir: dir, Location: time.UTC})
	if err != nil {
		t.Fatalf("open writer: %v", err)
	}

	// Raw insert pins updated_at; the repo Upsert would stamp store.now, which
	// this external package cannot override. Only intentions is set so the
	// null-omission of the other text fields is observable.
	updated := time.Date(2026, 9, 20, 9, 30, 0, 0, time.UTC).Unix()
	if err := w.Write(ctx, "seed journal", func(ctx context.Context, tx *sql.Tx) error {
		_, e := tx.ExecContext(ctx, `
			INSERT INTO journal_entries
			  (day, intentions, notes, goals, reflections, status, updated_at)
			VALUES (?, ?, NULL, NULL, NULL, ?, ?)`,
			"2026-09-20", "focus on agent CLI", storage.JournalStatusIntentionsSet, updated)
		return e
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}

	if err := w.Goals().Save(ctx, storage.DayGoal{
		Day:                     "2026-09-20",
		FocusTargetMinutes:      120,
		DistractionLimitMinutes: 30,
	}, nil); err != nil {
		t.Fatalf("seed goal: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return filepath.Join(dir, storage.DatabaseFileName)
}

// openReader opens the seeded database read-only in UTC.
func openReader(t *testing.T, path string) *Reader {
	t.Helper()
	r, err := OpenAt(context.Background(), path, time.UTC)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func marshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestStatusEnvelope(t *testing.T) {
	r := openReader(t, seedDB(t))
	r.now = func() time.Time { return time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC) }

	res, err := r.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	out := marshal(t, res)
	// schema_version must be the first key and equal 1 (docs/05 §5.9.1).
	if !strings.HasPrefix(out, "{\n  \"schema_version\": 1,") {
		t.Fatalf("schema_version is not the leading key:\n%s", out)
	}
	if res.GeneratedAt != "2026-09-20T12:00:00Z" {
		t.Fatalf("generated_at = %q, want RFC3339 UTC", res.GeneratedAt)
	}
	if res.DBUserVersion <= 0 {
		t.Fatalf("db_user_version = %d, want a migrated version", res.DBUserVersion)
	}
}

func TestDailyNullOmission(t *testing.T) {
	r := openReader(t, seedDB(t))
	res, err := r.Daily(context.Background(), "2026-09-20")
	if err != nil {
		t.Fatalf("Daily: %v", err)
	}
	out := marshal(t, res)

	for _, want := range []string{
		`"intentions": "focus on agent CLI"`,
		`"status": "intentions_set"`,
		`"updated_at": "2026-09-20T09:30:00Z"`,
		`"exists": true`,
		`"focus_target_minutes": 120`,
		`"focus_categories": []`,
		`"distraction_categories": []`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("daily output missing %s:\n%s", want, out)
		}
	}
	// The unset text fields must be omitted, not rendered as null.
	for _, absent := range []string{`"notes"`, `"goals"`, `"reflections"`} {
		if strings.Contains(out, absent) {
			t.Errorf("daily output should omit %s:\n%s", absent, out)
		}
	}
}

func TestDailyEmptyDay(t *testing.T) {
	r := openReader(t, seedDB(t))
	res, err := r.Daily(context.Background(), "2026-09-19")
	if err != nil {
		t.Fatalf("Daily: %v", err)
	}
	out := marshal(t, res)
	for _, want := range []string{
		`"status": ""`,
		`"exists": false`,
		`"focus_categories": []`,
		`"distraction_categories": []`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("empty daily missing %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, `"updated_at"`) {
		t.Errorf("empty daily should omit updated_at:\n%s", out)
	}
}

func TestTimelineEmpty(t *testing.T) {
	r := openReader(t, seedDB(t))
	res, err := r.Timeline(context.Background(), "2026-09-20")
	if err != nil {
		t.Fatalf("Timeline: %v", err)
	}
	if res.TrackedMinutes != 0 || res.IdleMinutes != 0 {
		t.Fatalf("empty day totals = tracked %v idle %v, want 0/0", res.TrackedMinutes, res.IdleMinutes)
	}
	if !strings.Contains(marshal(t, res), `"cards": []`) {
		t.Fatalf("empty timeline must render cards as []")
	}
}

func TestCardNotFound(t *testing.T) {
	r := openReader(t, seedDB(t))
	_, err := r.Card(context.Background(), 9999)
	assertFault(t, err, CodeNotFound)
}

func TestResolveDayInvalid(t *testing.T) {
	r := openReader(t, seedDB(t))
	_, err := r.Timeline(context.Background(), "not-a-day")
	assertFault(t, err, CodeInvalidArgument)
}

func TestCategoriesIncludesBuiltins(t *testing.T) {
	r := openReader(t, seedDB(t))
	res, err := r.Categories(context.Background())
	if err != nil {
		t.Fatalf("Categories: %v", err)
	}
	var sawSystem, sawIdle bool
	for _, c := range res.Categories {
		if c.Name == "System" && c.IsSystem {
			sawSystem = true
		}
		if c.Name == "Idle" && c.IsIdle {
			sawIdle = true
		}
	}
	if !sawSystem || !sawIdle {
		t.Fatalf("categories must include built-in System and Idle (system=%t idle=%t)", sawSystem, sawIdle)
	}
}

func assertFault(t *testing.T, err error, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected fault %q, got nil", wantCode)
	}
	f, ok := err.(*Fault)
	if !ok {
		t.Fatalf("error %v is not a *Fault", err)
	}
	if f.Code != wantCode {
		t.Fatalf("fault code = %q, want %q", f.Code, wantCode)
	}
}
