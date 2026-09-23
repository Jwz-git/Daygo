package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
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
	// Built-in categories ride along even with no cards; a fresh database
	// also carries the v12 starter set.
	if len(dto.Categories) != 8 {
		t.Fatalf("categories = %d, want the built-ins plus the starter set", len(dto.Categories))
	}
	if dto.TrackedMinutes != 0 || dto.IdleMinutes != 0 {
		t.Fatalf("totals = %v/%v, want 0/0", dto.TrackedMinutes, dto.IdleMinutes)
	}
	encoded, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal empty day: %v", err)
	}
	for _, field := range []string{"cards", "categories", "failures", "processingRanges"} {
		if strings.Contains(string(encoded), `"`+field+`":null`) {
			t.Fatalf("empty day field %q encoded as null: %s", field, encoded)
		}
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

func TestGetTimelineDayClipsCardAcrossFourAM(t *testing.T) {
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
	from := time.Date(2026, 9, 13, 3, 30, 0, 0, loc)
	to := time.Date(2026, 9, 13, 4, 30, 0, 0, loc)
	if _, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{{
		Start: "3:30 AM", End: "4:30 AM", Category: "Work", Title: "crossing", Summary: "s",
	}}, 1); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		day        string
		start, end int64
	}{
		{"2026-09-12", from.Unix(), from.Add(30 * time.Minute).Unix()},
		{"2026-09-13", from.Add(30 * time.Minute).Unix(), to.Unix()},
	} {
		got, err := backend.GetTimelineDay(tc.day)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Cards) != 1 || got.Cards[0].StartTs != tc.start || got.Cards[0].EndTs != tc.end || got.Cards[0].DurationMinutes != 30 || got.TrackedMinutes != 30 {
			t.Fatalf("%s = %+v, want one 30-minute visible slice", tc.day, got)
		}
	}
}

func TestCrossFourAMCardWriteInvalidatesBothDays(t *testing.T) {
	backend, emitter := writerBackendWithStore(t, t.TempDir())
	store := backend.store()
	ctx := context.Background()
	if err := store.Write(ctx, "seed crossing batch", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at) VALUES (1, 0, 0, 'succeeded', 0, 0)`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	loc := store.Location()
	from := time.Date(2026, 9, 13, 3, 30, 0, 0, loc)
	res, err := store.Cards().ReplaceCardsInRange(ctx, from, from.Add(time.Hour), []domain.CardShell{{
		Start: "3:30 AM", End: "4:30 AM", Category: "Focus Work", Title: "crossing", Summary: "s",
	}}, 1)
	if err != nil || len(res.InsertedIDs) != 1 {
		t.Fatalf("seed card = %+v, %v", res, err)
	}
	if err := backend.UpdateCardTitle(res.InsertedIDs[0], "edited"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for emitter.count(EventTimelineUpdated) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := emitter.count(EventTimelineUpdated); got != 2 {
		t.Fatalf("invalidation events = %d, want both logical days", got)
	}
}

// The bytes below are what the analysis pipeline stores for its happy-path
// card; internal/analysis/pipeline_test.go pins the producer to the same
// shape. appSites crossing this boundary as the model's flat list instead of
// the docs/05 §5.5.2 object made every card lose appSites, distractions and
// activityPoints together, because the decoration parse failed as a unit. The
// per-field decode that replaced it is covered by
// TestCardMetadataKeepsFieldsBesideAnUndecodableOne.
func TestCardMetadataWrittenByAnalysisPipelineParses(t *testing.T) {
	backend, _ := backendWithStore(t)
	seedTimelineDay(t, backend, []domain.CardShell{
		{
			Start: "10:00 AM", End: "10:15 AM", Category: "Coding",
			Title: "Editing code", Summary: "s",
			Metadata: `{"activityPoints":[],"appSites":{"primary":"Code","secondary":null},"distractions":[]}`,
		},
	})

	dto, err := backend.GetTimelineDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetTimelineDay: %v", err)
	}
	if len(dto.Cards) != 1 {
		t.Fatalf("cards = %d, want 1", len(dto.Cards))
	}
	card := dto.Cards[0]
	if card.AppSites == nil || card.AppSites.Primary == nil || *card.AppSites.Primary != "Code" {
		t.Fatalf("appSites = %+v, want primary Code from the pipeline's metadata bytes", card.AppSites)
	}
	if card.AppSites.Secondary != nil {
		t.Fatalf("secondary = %v, want null", card.AppSites.Secondary)
	}
}

// An undecodable decoration is dropped on its own. The pipeline maps what the
// model returns onto the metadata contract, so a drift at that seam shows up
// here as one field the binding layer cannot read — and it must not take its
// siblings with it. The flat distractions list below is the shape stored before
// that mapping existed, and a card carrying it is exactly the card that lost
// appSites, and with it the icon, in the field.
func TestCardMetadataKeepsFieldsBesideAnUndecodableOne(t *testing.T) {
	backend, _ := backendWithStore(t)
	seedTimelineDay(t, backend, []domain.CardShell{
		{
			Start: "10:00 AM", End: "10:30 AM", Category: "Coding",
			Title: "Editing code", Summary: "s",
			Metadata: `{"appSites":{"primary":"code.visualstudio.com","secondary":"github.com"},` +
				`"distractions":["7:17 PM opened a notification panel"],` +
				`"activityPoints":[{"time":"10:05 AM","description":"typed"}]}`,
		},
	})

	dto, err := backend.GetTimelineDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetTimelineDay: %v", err)
	}
	if len(dto.Cards) != 1 {
		t.Fatalf("cards = %d, want 1", len(dto.Cards))
	}
	card := dto.Cards[0]
	if card.AppSites == nil || card.AppSites.Primary == nil || *card.AppSites.Primary != "code.visualstudio.com" {
		t.Fatalf("appSites = %+v, want the primary stored beside the flat distractions list", card.AppSites)
	}
	if len(card.ActivityPoints) != 1 || card.ActivityPoints[0].Description != "typed" {
		t.Fatalf("activityPoints = %+v, want the point stored beside the flat distractions list", card.ActivityPoints)
	}
	// The undecodable field itself is dropped, never half-filled.
	if len(card.Distractions) != 0 {
		t.Fatalf("distractions = %+v, want none: the stored entries are strings", card.Distractions)
	}
}

// The distraction objects the pipeline stores carry a clock range and no id of
// their own; this layer assigns one so the inspector can key its rows
// (docs/05 §5.5.2: "metadata 中缺失时由 Go 生成，保持稳定").
func TestCardDistractionsCarryTheStoredClockRange(t *testing.T) {
	backend, _ := backendWithStore(t)
	seedTimelineDay(t, backend, []domain.CardShell{
		{
			Start: "10:00 AM", End: "10:30 AM", Category: "Coding",
			Title: "Editing code", Summary: "s",
			Metadata: `{"appSites":{"primary":"Code"},` +
				`"distractions":[{"startTime":"10:05 AM","endTime":"10:07 AM","title":"checked a feed","summary":"s"}]}`,
		},
	})

	dto, err := backend.GetTimelineDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetTimelineDay: %v", err)
	}
	if len(dto.Cards) != 1 || len(dto.Cards[0].Distractions) != 1 {
		t.Fatalf("distractions = %+v, want the stored entry", dto.Cards[0].Distractions)
	}
	distraction := dto.Cards[0].Distractions[0]
	if distraction.ID == "" {
		t.Fatal("distraction has no id: the inspector keys its rows by it")
	}
	if distraction.StartTime != "10:05 AM" || distraction.EndTime != "10:07 AM" || distraction.Title != "checked a feed" {
		t.Fatalf("distraction = %+v, want the stored clock range and title", distraction)
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
	// Built-in categories are pipeline-assigned, not manual targets.
	assertAppCode(t, backend.UpdateCardCategory(cardID, "Idle"), apperr.InvalidArgument)
	assertAppCode(t, backend.UpdateCardCategory(cardID, "System"), apperr.InvalidArgument)
	// Empty title is rejected.
	assertAppCode(t, backend.UpdateCardTitle(cardID, "  "), apperr.InvalidArgument)
	// Unknown card id is not_found.
	assertAppCode(t, backend.UpdateCardTitle(9999, "x"), apperr.NotFound)

	if err := backend.UpdateCardCategory(cardID, "Focus Work"); err != nil {
		t.Fatalf("UpdateCardCategory: %v", err)
	}
	if err := backend.UpdateCardTitle(cardID, "renamed"); err != nil {
		t.Fatalf("UpdateCardTitle: %v", err)
	}
	if err := backend.UpdateCardDetailedSummary(cardID, "user-rewritten detail"); err != nil {
		t.Fatalf("UpdateCardDetailedSummary: %v", err)
	}
	// Empty clears the detail; the pane falls back to the short summary.
	if err := backend.UpdateCardDetailedSummary(cardID, ""); err != nil {
		t.Fatalf("UpdateCardDetailedSummary empty: %v", err)
	}
	// Unknown card id is not_found.
	assertAppCode(t, backend.UpdateCardDetailedSummary(9999, "x"), apperr.NotFound)
	if err := backend.UpdateCardSummary(cardID, "user-rewritten short"); err != nil {
		t.Fatalf("UpdateCardSummary: %v", err)
	}
	if err := backend.UpdateCardSummary(cardID, ""); err != nil {
		t.Fatalf("UpdateCardSummary empty: %v", err)
	}
	assertAppCode(t, backend.UpdateCardSummary(9999, "x"), apperr.NotFound)
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
		"UpdateCardCategory":        func() error { return backend.UpdateCardCategory(1, "Focus Work") },
		"UpdateCardTitle":           func() error { return backend.UpdateCardTitle(1, "x") },
		"UpdateCardDetailedSummary": func() error { return backend.UpdateCardDetailedSummary(1, "x") },
		"UpdateCardSummary":         func() error { return backend.UpdateCardSummary(1, "x") },
		"DeleteCard":                func() error { return backend.DeleteCard(1) },
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

func TestClearHistoryData(t *testing.T) {
	dir := t.TempDir()
	backend, emitter := writerBackendWithStore(t, dir)
	seedTimelineDay(t, backend, []domain.CardShell{
		{Start: "10:00 AM", End: "10:30 AM", Category: "Coding", Title: "c1", Summary: "s"},
	})
	store := backend.store()
	if err := backend.ClearHistoryData(); err != nil {
		t.Fatalf("ClearHistoryData: %v", err)
	}

	cards, err := store.Cards().CardsForDay(context.Background(), "2026-09-12")
	if err != nil || len(cards) != 0 {
		t.Fatalf("cards after clear = %d (err %v), want 0", len(cards), err)
	}
	var batches int
	if err := store.Read(context.Background(), "count batches", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM analysis_batches").Scan(&batches)
	}); err != nil || batches != 0 {
		t.Fatalf("batches after clear = %d (err %v), want 0", batches, err)
	}
	// Configuration survives: built-in categories stay seeded. Clear only
	// wipes user data, so the starter set seeded at migration time stays too.
	categories, err := store.Categories().List(context.Background())
	if err != nil || len(categories) != 8 {
		t.Fatalf("categories after clear = %d (err %v), want the built-ins plus the starter set", len(categories), err)
	}
	// Recordings directories are recreated empty, not left missing.
	for _, sub := range []string{"staging", "segments", "timelapses"} {
		info, err := os.Stat(filepath.Join(filepath.Dir(store.Path()), "recordings", sub))
		if err != nil || !info.IsDir() {
			t.Fatalf("recordings/%s after clear: %v", sub, err)
		}
	}
	if emitter.count(EventTimelineUpdated) == 0 {
		t.Fatal("no timeline:updated emitted after clear")
	}

	// A second (read-only) instance on the same directory cannot clear.
	reader := openTestStore(t, dir, false)
	readBackend := newBackend(fixedClock{}, nil, reader, true, true)
	assertAppCode(t, readBackend.ClearHistoryData(), apperr.NotCaptureOwner)
}

// seedFailedBatch inserts a failed batch directly, mirroring what the
// pipeline leaves behind after an llm_error.
func seedFailedBatch(t *testing.T, b *Backend, id int64, at time.Time, attempts int) {
	t.Helper()
	store := b.store()
	err := store.Write(context.Background(), "seed failed batch", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO analysis_batches (id, start_ts, end_ts, status, failure_kind, failure_note, attempts, created_at, updated_at)
			 VALUES (?, ?, ?, 'failed', 'llm_error', 'provider rejected request with HTTP 502', ?, ?, ?)`,
			id, at.Unix(), at.Add(15*time.Minute).Unix(), attempts, at.Unix(), at.Unix())
		return err
	})
	if err != nil {
		t.Fatalf("seed failed batch: %v", err)
	}
}

func TestRetryAndDeleteBatches(t *testing.T) {
	dir := t.TempDir()
	backend, emitter := writerBackendWithStore(t, dir)
	at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	seedFailedBatch(t, backend, 1, at, 3)

	// A succeeded batch cannot be retried or deleted: constraint.
	assertAppCode(t, backend.RetryBatches([]int64{9999}), apperr.NotFound)
	assertAppCode(t, backend.RetryBatches(nil), apperr.InvalidArgument)

	if err := backend.RetryBatches([]int64{1}); err != nil {
		t.Fatalf("RetryBatches: %v", err)
	}
	// The day is invalidated so views re-pull the failure panel.
	waitForTimelineEmit(t, emitter)

	store := backend.store()
	var status string
	var attempts int
	if err := store.Read(context.Background(), "read batch", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT status, attempts FROM analysis_batches WHERE id = 1`).Scan(&status, &attempts)
	}); err != nil {
		t.Fatalf("read batch: %v", err)
	}
	if status != "pending" || attempts != 0 {
		t.Fatalf("batch after retry = %s/%d, want pending/0", status, attempts)
	}

	// Drive it back to failed, then dismiss it from the failure panel.
	if err := store.Analysis().SetBatchStatus(context.Background(), 1, "processing", "", "", at.Add(time.Hour)); err != nil {
		t.Fatalf("processing: %v", err)
	}
	if err := store.Analysis().SetBatchStatus(context.Background(), 1, "failed", "llm_error", "", at.Add(time.Hour)); err != nil {
		t.Fatalf("failed: %v", err)
	}
	if err := backend.DeleteBatches([]int64{1}); err != nil {
		t.Fatalf("DeleteBatches: %v", err)
	}

	// The day view no longer reports the failure.
	dto, err := backend.GetTimelineDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetTimelineDay: %v", err)
	}
	if len(dto.Failures) != 0 {
		t.Fatalf("failures after delete = %d, want 0", len(dto.Failures))
	}
	// A dismissed batch cannot be retried again.
	assertAppCode(t, backend.RetryBatches([]int64{1}), apperr.InvalidArgument)
}

func TestBatchWritesRefuseReadOnlyInstance(t *testing.T) {
	dir := t.TempDir()
	writerBackendWithStore(t, dir) // holds the write lock

	readerStore := openTestStore(t, dir, false)
	backend := newBackend(fixedClock{}, nil, readerStore, false, false)
	backend.setEventEmitter(&recordingEmitter{})

	assertAppCode(t, backend.RetryBatches([]int64{1}), apperr.NotCaptureOwner)
	assertAppCode(t, backend.DeleteBatches([]int64{1}), apperr.NotCaptureOwner)
}

func TestSaveCategoriesReplacesSetAndRewritesRenames(t *testing.T) {
	dir := t.TempDir()
	backend, emitter := writerBackendWithStore(t, dir)

	// A fresh database has the built-ins plus the starter set; one card in a
	// starter category exercises the rename rewrite.
	seedTimelineDay(t, backend, []domain.CardShell{
		{Start: "10:00 AM", End: "10:30 AM", Category: "Focus Work", Title: "c1", Summary: "s"},
	})
	day, err := backend.GetTimelineDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetTimelineDay: %v", err)
	}

	// Rewrite the set: keep "Communication" and "Distraction" as-is, rename
	// "Focus Work" to "Deep Work", drop the rest, add one new row.
	var kept, renamedID string
	for _, c := range day.Categories {
		switch c.Name {
		case "Focus Work":
			renamedID = c.ID
		case "Communication":
			kept = c.ID
		}
	}
	if renamedID == "" || kept == "" {
		t.Fatalf("starter categories missing: %+v", day.Categories)
	}
	next := []CategoryDTO{
		{ID: kept, Name: "Communication", ColorHex: "#FFAE8C"},
		{ID: renamedID, Name: "Deep Work", ColorHex: "#6A7EFF", Details: "renamed"},
		{Name: "New Category", ColorHex: "#123456"},
	}
	if err := backend.SaveCategories(next); err != nil {
		t.Fatalf("SaveCategories: %v", err)
	}

	after, err := backend.GetTimelineDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetTimelineDay after: %v", err)
	}
	// Built-ins merge back; the user set is exactly what was saved.
	want := map[string]bool{"System": true, "Idle": true, "Communication": true, "Deep Work": true, "New Category": true}
	if len(after.Categories) != len(want) {
		t.Fatalf("categories after = %+v, want exactly %v", after.Categories, want)
	}
	for _, c := range after.Categories {
		if !want[c.Name] {
			t.Fatalf("unexpected category %q survived the overwrite", c.Name)
		}
	}

	// The rename rewrote the card's category string in the same transaction.
	if len(after.Cards) != 1 || after.Cards[0].Category != "Deep Work" {
		t.Fatalf("card after rename = %+v, want category Deep Work", after.Cards)
	}

	// Only the day holding the renamed category's cards gets the event.
	waitForTimelineEmit(t, emitter)
	if got := emitter.count(EventTimelineUpdated); got != 1 {
		t.Fatalf("timeline:updated count = %d, want 1", got)
	}

	// Duplicate names are rejected and change nothing.
	assertAppCode(t, backend.SaveCategories([]CategoryDTO{
		{Name: "Dup", ColorHex: "#111111"},
		{Name: "Dup", ColorHex: "#222222"},
	}), apperr.InvalidArgument)
	assertAppCode(t, backend.SaveCategories([]CategoryDTO{
		{Name: "FakeSystem", IsSystem: true},
	}), apperr.InvalidArgument)
}

func TestReprocessDayRequeuesTerminalBatches(t *testing.T) {
	dir := t.TempDir()
	backend, emitter := writerBackendWithStore(t, dir)
	store := backend.store()
	at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)

	// A succeeded batch on the day, a failed batch on the day, and a
	// succeeded batch on the next day (must stay untouched).
	seedSucceededBatch := func(id int64, start time.Time) {
		t.Helper()
		err := store.Write(context.Background(), "seed succeeded batch", func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO analysis_batches (id, start_ts, end_ts, status, attempts, created_at, updated_at)
				 VALUES (?, ?, ?, 'succeeded', 1, ?, ?)`,
				id, start.Unix(), start.Add(15*time.Minute).Unix(), start.Unix(), start.Unix())
			return err
		})
		if err != nil {
			t.Fatalf("seed succeeded batch: %v", err)
		}
	}
	seedSucceededBatch(1, at)
	seedFailedBatch(t, backend, 2, at.Add(time.Hour), 3)
	seedSucceededBatch(3, at.Add(24*time.Hour))

	// Invalid day strings are rejected before anything is touched.
	assertAppCode(t, backend.ReprocessDay("2026-9-12"), apperr.InvalidArgument)
	assertAppCode(t, backend.ReprocessDay(""), apperr.InvalidArgument)

	if err := backend.ReprocessDay("2026-09-12"); err != nil {
		t.Fatalf("ReprocessDay: %v", err)
	}
	waitForTimelineEmit(t, emitter)

	byID := map[int64]string{}
	if err := store.Read(context.Background(), "read batches", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT id, status FROM analysis_batches`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id int64
			var status string
			if err := rows.Scan(&id, &status); err != nil {
				return err
			}
			byID[id] = status
		}
		return rows.Err()
	}); err != nil {
		t.Fatalf("read batches: %v", err)
	}
	for id, want := range map[int64]string{1: "pending", 2: "pending", 3: "succeeded"} {
		if byID[id] != want {
			t.Fatalf("batch %d = %s, want %s", id, byID[id], want)
		}
	}
}
