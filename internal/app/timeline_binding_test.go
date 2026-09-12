package app

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/domain"
)

// seedTimelineDay inserts anonymous cards through the real repositories, so
// binding tests exercise the same write path the analysis pipeline will use.
func seedTimelineDay(t *testing.T, b *Backend, shells []domain.CardShell) {
	t.Helper()
	store := b.store()
	err := store.Write(context.Background(), "seed batch", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at)
			 VALUES (1, 0, 0, 'succeeded', 0, 0)`)
		return err
	})
	if err != nil {
		t.Fatalf("seed batch: %v", err)
	}
	from := time.Date(2026, 9, 12, 4, 0, 0, 0, time.Local)
	to := time.Date(2026, 9, 13, 4, 0, 0, 0, time.Local)
	if _, err := store.Cards().ReplaceCardsInRange(context.Background(), from, to, shells, 1); err != nil {
		t.Fatalf("seed cards: %v", err)
	}
}

// writerBackendWithStore returns a backend that actually holds the write lock.
func writerBackendWithStore(t *testing.T, dir string) (*Backend, *recordingEmitter) {
	t.Helper()
	store := openTestStore(t, dir, true)
	backend := newBackend(fixedClock{now: time.Date(2026, 9, 12, 12, 0, 0, 0, time.Local)}, nil, store, true, true)
	emitter := &recordingEmitter{}
	backend.setEventEmitter(emitter)
	return backend, emitter
}

func TestGetTimelineDayEmpty(t *testing.T) {
	backend, _ := backendWithStore(t)

	dto, err := backend.GetTimelineDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetTimelineDay: %v", err)
	}
	if len(dto.Cards) != 0 || len(dto.Failures) != 0 || len(dto.ProcessingRanges) != 0 {
		t.Fatalf("empty day = %+v", dto)
	}
	// Built-in categories ride along even with no cards.
	if len(dto.Categories) != 2 {
		t.Fatalf("categories = %d, want the two built-ins", len(dto.Categories))
	}
	if dto.TrackedMinutes != 0 || dto.IdleMinutes != 0 {
		t.Fatalf("totals = %v/%v, want 0/0", dto.TrackedMinutes, dto.IdleMinutes)
	}
}

func TestGetTimelineDayRejectsInvalidDay(t *testing.T) {
	backend, _ := backendWithStore(t)
	_, err := backend.GetTimelineDay("2026-9-12")
	assertAppCode(t, err, apperr.InvalidArgument)
	_, err = backend.GetTimelineDay("")
	assertAppCode(t, err, apperr.InvalidArgument)
}

func TestGetTimelineDayCardsAndTotals(t *testing.T) {
	backend, _ := backendWithStore(t)
	seedTimelineDay(t, backend, []domain.CardShell{
		{Start: "10:00 AM", End: "10:30 AM", Category: "Coding", Title: "c1", Summary: "s", Metadata: `{"appSites":{"primary":"Xcode"}}`},
		{Start: "10:30 AM", End: "11:00 AM", Category: "Idle", Title: "i1", Summary: "s"},
		{Start: "11:00 AM", End: "11:15 AM", Category: "System", Title: "sys", Summary: "s"},
		{Start: "11:15 AM", End: "11:20 AM", Category: "Coding", Title: "c2", Summary: "s", Metadata: `not json`},
	})

	dto, err := backend.GetTimelineDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetTimelineDay: %v", err)
	}
	if len(dto.Cards) != 4 {
		t.Fatalf("cards = %d, want 4", len(dto.Cards))
	}
	if dto.TrackedMinutes != 35 { // 30 Coding + 5 Coding; Idle separate, System excluded
		t.Fatalf("tracked = %v, want 35", dto.TrackedMinutes)
	}
	if dto.IdleMinutes != 30 {
		t.Fatalf("idle = %v, want 30", dto.IdleMinutes)
	}
	first := dto.Cards[0]
	if first.AppSites == nil || first.AppSites.Primary == nil || *first.AppSites.Primary != "Xcode" {
		t.Fatalf("appSites = %+v, want primary Xcode from metadata", first.AppSites)
	}
	// Idle card flags isIdle via the built-in category.
	idleCard := dto.Cards[1]
	if !idleCard.IsIdle {
		t.Fatal("Idle card not flagged isIdle")
	}
	// Malformed metadata degrades to no decorations, not an error.
	if dto.Cards[3].AppSites != nil {
		t.Fatal("malformed metadata produced appSites")
	}
}

func TestCardWritesValidateAndEmit(t *testing.T) {
	dir := t.TempDir()
	backend, emitter := writerBackendWithStore(t, dir)
	seedTimelineDay(t, backend, []domain.CardShell{
		{Start: "10:00 AM", End: "10:30 AM", Category: "Coding", Title: "c1", Summary: "s"},
	})

	var cardID int64
	store := backend.store()
	cards, err := store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if err != nil || len(cards) != 1 {
		t.Fatalf("seed read back: %v (%d cards)", err, len(cards))
	}
	cardID = cards[0].ID

	// Unknown category is rejected and never creates one implicitly.
	assertAppCode(t, backend.UpdateCardCategory(cardID, "Nope"), apperr.InvalidArgument)
	// Empty title is rejected.
	assertAppCode(t, backend.UpdateCardTitle(cardID, "  "), apperr.InvalidArgument)
	// Unknown card id is not_found.
	assertAppCode(t, backend.UpdateCardTitle(9999, "x"), apperr.NotFound)

	if err := backend.UpdateCardCategory(cardID, "Idle"); err != nil {
		t.Fatalf("UpdateCardCategory: %v", err)
	}
	if err := backend.UpdateCardTitle(cardID, "renamed"); err != nil {
		t.Fatalf("UpdateCardTitle: %v", err)
	}
	if err := backend.DeleteCard(cardID); err != nil {
		t.Fatalf("DeleteCard: %v", err)
	}

	// The merge window collapses these into one timeline:updated for the day.
	waitForTimelineEmit(t, emitter)
	timelineEmits := emitter.count(EventTimelineUpdated)
	if timelineEmits != 1 {
		t.Fatalf("timeline:updated count = %d, want 1 (merged)", timelineEmits)
	}
	last := emitter.events[len(emitter.events)-1]
	payload, ok := last.payload.(TimelineUpdatedPayload)
	if !ok || payload.Day != "2026-09-12" {
		t.Fatalf("payload = %+v, want {day: 2026-09-12}", last.payload)
	}

	after, err := store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("cards after delete = %d, want 0 (soft-deleted)", len(after))
	}
}

// waitForTimelineEmit lets the 200 ms merge timer fire before assertions.
func waitForTimelineEmit(t *testing.T, emitter *recordingEmitter) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if emitter.count(EventTimelineUpdated) > 0 {
			// Give sibling timers on the same day the same chance.
			time.Sleep(50 * time.Millisecond)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCardWritesRefuseReadOnlyInstance(t *testing.T) {
	dir := t.TempDir()
	writerBackendWithStore(t, dir) // holds the write lock

	// A second instance on the same directory is read-only at the connection layer.
	readerStore := openTestStore(t, dir, true)
	backend := newBackend(fixedClock{}, nil, readerStore, false, false)
	backend.setEventEmitter(&recordingEmitter{})

	updates := map[string]func() error{
		"UpdateCardCategory": func() error { return backend.UpdateCardCategory(1, "Idle") },
		"UpdateCardTitle":    func() error { return backend.UpdateCardTitle(1, "x") },
		"DeleteCard":         func() error { return backend.DeleteCard(1) },
	}
	for _, fn := range updates {
		assertAppCode(t, fn(), apperr.NotCaptureOwner)
	}
}

func TestMergeFailuresTolerance(t *testing.T) {
	got := mergeFailures([]failedBatchView{
		{ID: 1, StartTs: 100, EndTs: 200, FailureKind: "llm_error", FailureNote: "a"},
		{ID: 2, StartTs: 260, EndTs: 300, FailureKind: "llm_error", FailureNote: "b"}, // 60s after 200: merges
		{ID: 3, StartTs: 400, EndTs: 500, FailureKind: "media", FailureNote: "c"},     // 100s after 300: splits
	})
	if len(got) != 2 {
		t.Fatalf("groups = %d, want 2", len(got))
	}
	if len(got[0].BatchIDs) != 2 || got[0].EndTs != 300 {
		t.Fatalf("group 1 = %+v, want ids [1 2] end 300", got[0])
	}
	if len(got[1].BatchIDs) != 1 || got[1].Kind != "media" {
		t.Fatalf("group 2 = %+v, want id [3] kind media", got[1])
	}
	if strings.Contains(got[0].Message, "\n") {
		t.Fatal("message must be the sanitized note verbatim")
	}
}
