package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// openWriterAt opens a writer pinned to a fixed zone, so card derivation can
// be tested against DST and fractional offsets without touching the host.
func openWriterAt(t *testing.T, dir, zone string) *Store {
	t.Helper()
	loc, err := time.LoadLocation(zone)
	if err != nil {
		t.Fatalf("load zone %s: %v", zone, err)
	}
	store, err := Open(context.Background(), Options{Dir: dir, Location: loc})
	if err != nil {
		t.Fatalf("Open(%s) writer: %v", dir, err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if store.Mode() != ModeReadWrite {
		t.Fatalf("Mode() = %q, want read_write", store.Mode())
	}
	return store
}

// cardWindow is a helper building a [from, to) window from local wall times.
func window(loc *time.Location, fromH, fromM, toH, toM int) (time.Time, time.Time) {
	return time.Date(2026, 9, 12, fromH, fromM, 0, 0, loc), time.Date(2026, 9, 12, toH, toM, 0, 0, loc)
}

func shell(start, end, category, title string) domain.CardShell {
	return domain.CardShell{
		Start: start, End: end, Category: category,
		Title: title, Summary: "s:" + title,
	}
}

func TestReplaceCardsInRangeInsertsAndDerivesTimestamps(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	loc := store.location()

	from, to := window(loc, 10, 0, 11, 0)
	res, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		shell("10:21 AM", "10:50 AM", "Coding", "review-pr"),
		shell("10:55 AM", "11:05 AM", "Writing", "doc-notes"),
	}, 1)
	if err != nil {
		t.Fatalf("ReplaceCardsInRange: %v", err)
	}
	if len(res.InsertedIDs) != 2 || len(res.SkippedCards) != 0 {
		t.Fatalf("result = %+v, want 2 inserted 0 skipped", res)
	}

	cards, err := store.Cards().CardsForDay(ctx, "2026-09-12")
	if err != nil {
		t.Fatalf("CardsForDay: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("cards = %d, want 2", len(cards))
	}
	first := cards[0]
	if first.Title != "review-pr" {
		t.Fatalf("first card = %q, want review-pr ordered by start", first.Title)
	}
	if want := time.Date(2026, 9, 12, 10, 21, 0, 0, loc); first.StartTs != want.Unix() {
		t.Fatalf("start_ts = %d, want %d", first.StartTs, want.Unix())
	}
	if first.Day != "2026-09-12" || first.BatchID == nil || *first.BatchID != 1 {
		t.Fatalf("card day/batch wrong: %s %+v", first.Day, first.BatchID)
	}
}

// 03 §3.5 rule 1 + 2: near-midnight clock strings resolve onto the nearest of
// three day candidates, and end < start crosses midnight.
func TestReplaceCardsInRangeNearMidnightAndCrossesMidnight(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	loc := store.location()

	// Window centered at 00:30 on the 12th.
	from := time.Date(2026, 9, 12, 0, 0, 0, 0, loc)
	to := time.Date(2026, 9, 12, 1, 0, 0, 0, loc)

	res, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		shell("11:50 PM", "12:10 AM", "Coding", "late-night"),
	}, 1)
	if err != nil {
		t.Fatalf("ReplaceCardsInRange: %v", err)
	}
	if len(res.SkippedCards) != 0 {
		t.Fatalf("skipped = %d, want 0", len(res.SkippedCards))
	}

	cards, err := store.Cards().CardsForDay(ctx, "2026-09-11")
	if err != nil {
		t.Fatalf("CardsForDay: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("cards on logical day 09-11 = %d, want 1 (4 AM boundary)", len(cards))
	}
	c := cards[0]
	if want := time.Date(2026, 9, 11, 23, 50, 0, 0, loc); c.StartTs != want.Unix() {
		t.Fatalf("start_ts = %d, want %d (previous day's 11:50 PM)", c.StartTs, want.Unix())
	}
	if want := time.Date(2026, 9, 12, 0, 10, 0, 0, loc); c.EndTs != want.Unix() {
		t.Fatalf("end_ts = %d, want %d (crossed midnight)", c.EndTs, want.Unix())
	}
	if c.Day != "2026-09-11" {
		t.Fatalf("day = %s, want 2026-09-11 (4 AM boundary from start)", c.Day)
	}
}

// 03 §3.5: unparseable clock strings must surface in SkippedCards, never
// disappear, and the resolvable siblings still commit.
func TestReplaceCardsInRangeSkipsUnparseableButCommitsRest(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	loc := store.location()

	before := SkippedCards()
	from, to := window(loc, 10, 0, 11, 0)
	res, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		shell("10:21 AM", "10:50 AM", "Coding", "good-card"),
		shell("sometime", "later", "Coding", "bad-card"),
	}, 1)
	if err != nil {
		t.Fatalf("ReplaceCardsInRange: %v", err)
	}
	if len(res.InsertedIDs) != 1 {
		t.Fatalf("inserted = %d, want 1", len(res.InsertedIDs))
	}
	if len(res.SkippedCards) != 1 || res.SkippedCards[0].Title != "bad-card" {
		t.Fatalf("skipped = %+v, want bad-card", res.SkippedCards)
	}
	if got := SkippedCards() - before; got != 1 {
		t.Fatalf("SkippedCards counter delta = %d, want 1 (must feed diagnostics)", got)
	}

	cards, err := store.Cards().CardsForDay(ctx, "2026-09-12")
	if err != nil {
		t.Fatalf("CardsForDay: %v", err)
	}
	if len(cards) != 1 || cards[0].Title != "good-card" {
		t.Fatalf("cards = %+v, want only good-card", cards)
	}
}

// A second rewrite of the same range replaces the first batch's cards but
// keeps System cards from other batches (docs/03 §3.5 overlap predicate).
func TestReplaceCardsInRangeKeepsOtherBatchesSystemCards(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	loc := store.location()

	from, to := window(loc, 10, 0, 11, 0)
	if _, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		shell("10:00 AM", "10:30 AM", "Coding", "old-activity"),
	}, 1); err != nil {
		t.Fatalf("first replace: %v", err)
	}
	// A System failure marker written by batch 2 inside the same range.
	if err := seedSystemCard(t, store, 2, "10:30 AM", "10:40 AM"); err != nil {
		t.Fatalf("seedSystemCard: %v", err)
	}

	// Batch 1 rewrites its range.
	res, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		shell("10:05 AM", "10:25 AM", "Coding", "new-activity"),
	}, 1)
	if err != nil {
		t.Fatalf("second replace: %v", err)
	}
	if len(res.InsertedIDs) != 1 {
		t.Fatalf("inserted = %d, want 1", len(res.InsertedIDs))
	}

	cards, err := store.Cards().CardsForDay(ctx, "2026-09-12")
	if err != nil {
		t.Fatalf("CardsForDay: %v", err)
	}
	var titles []string
	for _, c := range cards {
		titles = append(titles, c.Title+"/"+c.Category)
	}
	// new-activity (batch 1), the System marker (batch 2); old-activity soft-deleted.
	if len(cards) != 2 {
		t.Fatalf("cards = %v, want new-activity + system marker", titles)
	}
	hasSystem, hasNew := false, false
	for _, c := range cards {
		if c.Category == "System" && c.Title == "failure-marker" {
			hasSystem = true
		}
		if c.Title == "new-activity" {
			hasNew = true
		}
	}
	if !hasSystem || !hasNew {
		t.Fatalf("cards = %v, want system marker and new activity", titles)
	}
}

func TestReplaceCardsInRangeCollectsDeletedVideoPaths(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	loc := store.location()

	from, to := window(loc, 10, 0, 11, 0)
	res, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{{
		Start: "10:00 AM", End: "10:30 AM", Category: "Coding",
		Title: "with-video", Summary: "s", VideoSummaryPath: "timelapses/2026-09-12/a.mp4",
	}}, 1)
	if err != nil {
		t.Fatalf("first replace: %v", err)
	}
	if len(res.DeletedVideoPaths) != 0 {
		t.Fatalf("first replace video paths = %v, want none", res.DeletedVideoPaths)
	}

	res, err = store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		shell("10:00 AM", "10:30 AM", "Coding", "replacement"),
	}, 1)
	if err != nil {
		t.Fatalf("second replace: %v", err)
	}
	if len(res.DeletedVideoPaths) != 1 || res.DeletedVideoPaths[0] != "timelapses/2026-09-12/a.mp4" {
		t.Fatalf("video paths = %v, want the replaced card's timelapse", res.DeletedVideoPaths)
	}
}

func TestCardUpdateCategoryTitleAndSoftDelete(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	loc := store.location()

	from, to := window(loc, 10, 0, 11, 0)
	res, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		shell("10:00 AM", "10:30 AM", "Coding", "card-one"),
	}, 1)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	id := res.InsertedIDs[0]

	if err := store.Cards().UpdateCardCategory(ctx, id, "Writing"); err != nil {
		t.Fatalf("UpdateCardCategory: %v", err)
	}
	if err := store.Cards().UpdateCardTitle(ctx, id, "renamed"); err != nil {
		t.Fatalf("UpdateCardTitle: %v", err)
	}
	card, err := store.Cards().CardByID(ctx, id)
	if err != nil {
		t.Fatalf("CardByID: %v", err)
	}
	if card.Category != "Writing" || card.Title != "renamed" {
		t.Fatalf("card = %s/%s, want Writing/renamed", card.Category, card.Title)
	}

	if _, err := store.Cards().SoftDeleteCard(ctx, id); err != nil {
		t.Fatalf("SoftDeleteCard: %v", err)
	}
	if _, err := store.Cards().CardByID(ctx, id); !IsKind(err, KindNotFound) {
		t.Fatalf("CardByID after delete = %v, want not_found", err)
	}
	// Updating a deleted card is not_found, not a silent success.
	if err := store.Cards().UpdateCardTitle(ctx, id, "zombie"); !IsKind(err, KindNotFound) {
		t.Fatalf("UpdateCardTitle after delete = %v, want not_found", err)
	}
}

func TestCardsInRangeAndTotalMinutesExcludesSystem(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	loc := store.location()

	from, to := window(loc, 10, 0, 11, 0)
	if _, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		shell("10:00 AM", "10:30 AM", "Coding", "thirty"),
		shell("10:30 AM", "11:00 AM", "Writing", "thirty-more"),
	}, 1); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := seedSystemCard(t, store, 2, "10:00 AM", "11:00 AM"); err != nil {
		t.Fatalf("seedSystemCard: %v", err)
	}

	rangeFrom := time.Date(2026, 9, 12, 4, 0, 0, 0, loc)
	rangeTo := time.Date(2026, 9, 13, 4, 0, 0, 0, loc)
	cards, err := store.Cards().CardsInRange(ctx, rangeFrom, rangeTo)
	if err != nil {
		t.Fatalf("CardsInRange: %v", err)
	}
	if len(cards) != 3 {
		t.Fatalf("cards in range = %d, want 3 (System included in queries)", len(cards))
	}

	minutes, err := store.Cards().TotalMinutesTracked(ctx, rangeFrom, rangeTo)
	if err != nil {
		t.Fatalf("TotalMinutesTracked: %v", err)
	}
	if minutes != 60 {
		t.Fatalf("tracked minutes = %v, want 60 (System excluded)", minutes)
	}
}

// DST: a rewrite whose anchor falls in the extra hour of a fall-back day must
// still derive both candidates from local wall time, not UTC arithmetic.
func TestReplaceCardsInRangeAcrossDSTFallBack(t *testing.T) {
	store := openWriterAt(t, newDir(t), "America/New_York")
	ctx := context.Background()
	loc := store.location()

	// 2026-11-01: 01:00-02:00 local happens twice. Window anchored in the gap.
	from := time.Date(2026, 11, 1, 0, 30, 0, 0, loc)
	to := time.Date(2026, 11, 1, 3, 30, 0, 0, loc)

	res, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		shell("12:45 AM", "1:30 AM", "Coding", "dst-card"),
	}, 1)
	if err != nil {
		t.Fatalf("ReplaceCardsInRange: %v", err)
	}
	if len(res.SkippedCards) != 0 {
		t.Fatalf("skipped = %d, want 0 across DST", len(res.SkippedCards))
	}
	cards, err := store.Cards().CardsForDay(ctx, "2026-10-31")
	if err != nil {
		t.Fatalf("CardsForDay: %v", err)
	}
	if len(cards) != 1 || cards[0].Title != "dst-card" {
		t.Fatalf("cards = %+v, want dst-card on logical day 10-31", cards)
	}
	// end > start: no accidental day-crossing from DST shift.
	if cards[0].EndTs <= cards[0].StartTs {
		t.Fatalf("end_ts <= start_ts across fall-back: %d <= %d", cards[0].EndTs, cards[0].StartTs)
	}
}

// seedSystemCard inserts a failure-marker-shaped System card for batchID with
// raw SQL. The pipeline writes these through ReplaceCardsInRange, but going
// through it here would soft-delete the window's other cards, which is that
// method's behavior under test, not this seed's job.
func seedSystemCard(t *testing.T, store *Store, batchID int64, start, end string) error {
	t.Helper()
	loc := store.location()
	from := mustResolveClock(t, start, loc)
	to := mustResolveClock(t, end, loc)
	at := store.now().Unix()
	return store.Write(context.Background(), "seed system card", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO timeline_cards
				(batch_id, day, start, end, start_ts, end_ts, category, title, summary, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 'System', 'failure-marker', 'seeded', ?, ?)`,
			batchID, timeutil.LogicalDay(from, loc), start, end, from.Unix(), to.Unix(), at, at)
		return err
	})
}

func mustResolveClock(t *testing.T, clock string, loc *time.Location) time.Time {
	t.Helper()
	at, err := timeutil.ResolveClock(clock, time.Date(2026, 9, 12, 12, 0, 0, 0, loc), loc)
	if err != nil {
		t.Fatalf("resolve %q: %v", clock, err)
	}
	return at
}

// Concurrent rewrites of overlapping ranges must not interleave into a
// half-written window. A worker that loses the race sees SQLITE_BUSY (mapped
// to KindBusy, retryable as conflict at the binding layer); that is the
// designed behavior, not a failure. The invariant is the committed state:
// whoever wins, exactly one rewrite's cards are visible.
func TestReplaceCardsInRangeConcurrentOverlapStaysConsistent(t *testing.T) {
	store := openWriterAt(t, newDir(t), "Asia/Shanghai")
	ctx := context.Background()
	loc := store.location()
	from, to := window(loc, 10, 0, 11, 0)

	const workers = 4
	done := make(chan error, workers)
	committed := 0
	for w := range workers {
		go func(w int) {
			shells := []domain.CardShell{
				shell("10:00 AM", "10:30 AM", "Coding", "activity"),
				shell("10:30 AM", "11:00 AM", "Writing", "activity"),
			}
			_, err := store.Cards().ReplaceCardsInRange(ctx, from, to, shells, int64(w+1))
			done <- err
		}(w)
	}
	for w := range workers {
		err := <-done
		switch {
		case err == nil:
			committed++
		case IsKind(err, KindBusy):
			// Lost the write race; retryable by contract.
		default:
			t.Fatalf("worker %d: %v", w, err)
		}
	}
	if committed == 0 {
		t.Fatal("all workers reported busy; none of the rewrites committed")
	}

	cards, err := store.Cards().CardsForDay(ctx, "2026-09-12")
	if err != nil {
		t.Fatalf("CardsForDay: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("cards = %d, want exactly 2 after %d overlapping rewrites", len(cards), workers)
	}
}
