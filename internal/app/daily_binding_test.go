package app

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/domain"
)

// seedWeeklyCards seeds a Coding category and cards with explicit timestamps
// (ReplaceCardsInRange derives timestamps from one anchor day, which cannot
// span a week), inside the week of 2026-09-07 (Monday).
func seedWeeklyCards(t *testing.T, b *Backend) {
	t.Helper()
	store := b.store()
	err := store.Write(context.Background(), "seed weekly fixtures", func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO categories (id, name, color_hex, sort_order, is_system, is_idle, created_at, updated_at)
			 VALUES ('cat-coding', 'Coding', '#123456', 0, 0, 0, 0, 0)`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at)
			 VALUES (1, 0, 0, 'succeeded', 0, 0)`); err != nil {
			return err
		}
		// 2026-09-09 10:00–11:00 local, Coding: 60 focus minutes.
		_, err := tx.ExecContext(ctx,
			`INSERT INTO timeline_cards (batch_id, day, start, end, start_ts, end_ts, category, title, summary, created_at, updated_at)
			 VALUES (1, '2026-09-09', '10:00 AM', '11:00 AM', ?, ?, 'Coding', 'c1', 's', 0, 0)`,
			time.Date(2026, 9, 9, 10, 0, 0, 0, time.Local).Unix(),
			time.Date(2026, 9, 9, 11, 0, 0, 0, time.Local).Unix())
		if err != nil {
			return err
		}
		// 2026-09-10 14:00–14:30 local, Idle: 30 idle minutes.
		_, err = tx.ExecContext(ctx,
			`INSERT INTO timeline_cards (batch_id, day, start, end, start_ts, end_ts, category, title, summary, created_at, updated_at)
			 VALUES (1, '2026-09-10', '2:00 PM', '2:30 PM', ?, ?, 'Idle', 'i1', 's', 0, 0)`,
			time.Date(2026, 9, 10, 14, 0, 0, 0, time.Local).Unix(),
			time.Date(2026, 9, 10, 14, 30, 0, 0, time.Local).Unix())
		return err
	})
	if err != nil {
		t.Fatalf("seed weekly fixtures: %v", err)
	}
}

func TestGetWeeklyDashboard(t *testing.T) {
	backend, _ := backendWithStore(t)
	seedWeeklyCards(t, backend)

	dto, err := backend.GetWeeklyDashboard("2026-09-07")
	if err != nil {
		t.Fatalf("GetWeeklyDashboard: %v", err)
	}
	if dto.WeekStart != "2026-09-07" {
		t.Fatalf("weekStart = %q", dto.WeekStart)
	}
	if dto.TrackedMinutes != 90 { // 60 Coding + 30 Idle; System excluded
		t.Fatalf("tracked = %v, want 90", dto.TrackedMinutes)
	}
	if dto.FocusMinutes != 60 { // Idle excluded from focus
		t.Fatalf("focus = %v, want 60", dto.FocusMinutes)
	}
	if len(dto.Categories) != 2 || dto.Categories[0].Name != "Coding" {
		t.Fatalf("categories = %+v, want Coding first", dto.Categories)
	}
	if dto.Categories[0].Share != 60.0/90 {
		t.Fatalf("coding share = %v", dto.Categories[0].Share)
	}

	// Detail charts: days carry per-day totals and segments, insights carry
	// the derived week facts.
	if len(dto.Days) != 7 {
		t.Fatalf("days = %d, want 7", len(dto.Days))
	}
	wednesday := dto.Days[2]
	if wednesday.Day != "2026-09-09" || wednesday.TrackedMinutes != 60 || wednesday.FocusMinutes != 60 {
		t.Fatalf("wednesday = %+v, want 2026-09-09 60/60", wednesday)
	}
	if len(wednesday.Segments) != 1 || wednesday.Segments[0].Category != "Coding" {
		t.Fatalf("wednesday segments = %+v", wednesday.Segments)
	}
	thursday := dto.Days[3]
	if thursday.Day != "2026-09-10" || thursday.TrackedMinutes != 30 || thursday.FocusMinutes != 0 {
		t.Fatalf("thursday = %+v, want 2026-09-10 30 tracked / 0 focus (idle)", thursday)
	}
	if len(thursday.Segments) != 1 || !thursday.Segments[0].IsIdle {
		t.Fatalf("thursday segments = %+v, want one idle segment", thursday.Segments)
	}

	ins := dto.Insights
	if ins.ActiveDays != 2 || ins.LongestFocusMinutes != 60 || ins.LongestFocusDay != "2026-09-09" {
		t.Fatalf("insights = %+v, want 2 active days and 60m longest focus on Wed", ins)
	}
	if ins.MostActiveDay != "2026-09-09" || ins.MostActiveDayMinutes != 60 {
		t.Fatalf("most active = %+v", ins)
	}
	if ins.AvgDailyFocusMinutes != 30 {
		t.Fatalf("avg daily focus = %v, want 60/2", ins.AvgDailyFocusMinutes)
	}
}

func TestGetWeeklyDashboardSplitsCardAtMondayFourAM(t *testing.T) {
	backend, _ := backendWithStore(t)
	store := backend.store()
	ctx := context.Background()
	if err := store.Write(ctx, "seed crossing batch", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at) VALUES (1, 0, 0, 'succeeded', 0, 0)`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	loc := store.Location()
	from := time.Date(2026, 9, 14, 3, 30, 0, 0, loc)
	if _, err := store.Cards().ReplaceCardsInRange(ctx, from, from.Add(time.Hour), []domain.CardShell{{
		Start: "3:30 AM", End: "4:30 AM", Category: "Focus Work", Title: "week boundary", Summary: "s",
	}}, 1); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		week, day string
		index     int
	}{
		{"2026-09-07", "2026-09-13", 6},
		{"2026-09-14", "2026-09-14", 0},
	} {
		got, err := backend.GetWeeklyDashboard(tc.week)
		if err != nil {
			t.Fatal(err)
		}
		if got.TrackedMinutes != 30 || got.FocusMinutes != 30 || got.Days[tc.index].Day != tc.day || got.Days[tc.index].TrackedMinutes != 30 || len(got.Days[tc.index].Segments) != 1 {
			t.Fatalf("week %s = %+v; want one 30-minute slice on %s", tc.week, got, tc.day)
		}
	}
}

func TestGetWeeklyDashboardEmptyWeek(t *testing.T) {
	backend, _ := backendWithStore(t)

	dto, err := backend.GetWeeklyDashboard("2026-09-07")
	if err != nil {
		t.Fatalf("GetWeeklyDashboard(empty): %v", err)
	}
	if dto.TrackedMinutes != 0 || len(dto.Categories) != 0 {
		t.Fatalf("empty week = %+v", dto)
	}
}

func TestGetWeeklyDashboardRejectsNonMonday(t *testing.T) {
	backend, _ := backendWithStore(t)
	_, err := backend.GetWeeklyDashboard("2026-09-08")
	assertAppCode(t, err, apperr.InvalidArgument)
	_, err = backend.GetWeeklyDashboard("")
	assertAppCode(t, err, apperr.InvalidArgument)
}

func TestJournalBindingsRoundTrip(t *testing.T) {
	backend, emitter := backendWithStore(t)

	// No entry yet: zero DTO, not an error.
	entry, err := backend.GetJournalDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetJournalDay(empty): %v", err)
	}
	if entry.UpdatedAtTs != nil || entry.Status != "" {
		t.Fatalf("empty journal = %+v", entry)
	}

	intent := "finish the bindings slice"
	if err := backend.SaveJournalDay(JournalDayDTO{
		Day: "2026-09-12", Intentions: &intent, Status: "intentions_set",
	}); err != nil {
		t.Fatalf("SaveJournalDay: %v", err)
	}
	if emitter.count(EventJournalUpdated) != 1 {
		t.Fatalf("journal:updated count = %d", emitter.count(EventJournalUpdated))
	}

	// An invalid status is rejected.
	assertAppCode(t, backend.SaveJournalDay(JournalDayDTO{
		Day: "2026-09-12", Status: "nonsense",
	}), apperr.InvalidArgument)

	got, err := backend.GetJournalDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetJournalDay: %v", err)
	}
	if got.Intentions == nil || *got.Intentions != intent {
		t.Fatalf("intentions = %v", got.Intentions)
	}
	if got.Status != "intentions_set" || got.UpdatedAtTs == nil {
		t.Fatalf("entry = %+v", got)
	}
}

func TestGoalBindingsRoundTrip(t *testing.T) {
	backend, emitter := backendWithStore(t)
	seedWeeklyCards(t, backend) // provides the Coding category

	goal, err := backend.GetDayGoal("2026-09-12")
	if err != nil {
		t.Fatalf("GetDayGoal(empty): %v", err)
	}
	if goal.Exists {
		t.Fatalf("empty goal = %+v, want exists=false", goal)
	}

	// Unknown category id is a clear invalid_argument before any write.
	assertAppCode(t, backend.SaveDayGoal(DayGoalDTO{
		Day: "2026-09-12", FocusCategories: []GoalCategoryRefDTO{{CategoryID: "nope"}},
	}), apperr.InvalidArgument)

	if err := backend.SaveDayGoal(DayGoalDTO{
		Day:                "2026-09-12",
		FocusTargetMinutes: 240,
		FocusCategories:    []GoalCategoryRefDTO{{CategoryID: "cat-coding", Name: "Coding"}},
	}); err != nil {
		t.Fatalf("SaveDayGoal: %v", err)
	}
	if emitter.count(EventGoalUpdated) != 1 {
		t.Fatalf("goal:updated count = %d", emitter.count(EventGoalUpdated))
	}

	got, err := backend.GetDayGoal("2026-09-12")
	if err != nil {
		t.Fatalf("GetDayGoal: %v", err)
	}
	if !got.Exists || got.FocusTargetMinutes != 240 {
		t.Fatalf("goal = %+v", got)
	}
	if len(got.FocusCategories) != 1 || got.FocusCategories[0].Name != "Coding" {
		t.Fatalf("focus categories = %+v", got.FocusCategories)
	}
	if len(got.DistractionCategories) != 0 {
		t.Fatalf("distraction categories = %+v, want empty list not nil", got.DistractionCategories)
	}
}

func TestJournalAndGoalWritesRefuseReadOnly(t *testing.T) {
	dir := t.TempDir()
	writerBackendWithStore(t, dir)

	readerStore := openTestStore(t, dir, true)
	backend := newBackend(fixedClock{}, nil, readerStore, false, false)
	backend.setEventEmitter(&recordingEmitter{})

	assertAppCode(t, backend.SaveJournalDay(JournalDayDTO{Day: "2026-09-12"}), apperr.NotCaptureOwner)
	assertAppCode(t, backend.SaveDayGoal(DayGoalDTO{Day: "2026-09-12"}), apperr.NotCaptureOwner)
}
