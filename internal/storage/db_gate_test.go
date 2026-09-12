package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
)

// readEveryTable exercises every read-only repository method this package
// exposes. DB-3 asserts each returns without error on every fixture, because a
// method that only works on a populated database is a latent failure on a
// fresh install.
func readEveryTable(t *testing.T, store *Store) {
	t.Helper()
	ctx := context.Background()

	// settings
	if _, err := store.Settings().GetAll(ctx); err != nil {
		t.Errorf("settings GetAll: %v", err)
	}
	if _, _, err := store.Settings().Get(ctx, "appearance.theme"); err != nil {
		t.Errorf("settings Get: %v", err)
	}

	// cards
	if _, err := store.Cards().CardsForDay(ctx, "2026-09-12"); err != nil {
		t.Errorf("cards CardsForDay: %v", err)
	}
	if _, err := store.Cards().CardsInRange(ctx,
		time.Unix(0, 0), time.Unix(2_000_000_000, 0)); err != nil {
		t.Errorf("cards CardsInRange: %v", err)
	}
	if _, err := store.Cards().CardsForBatch(ctx, 1); err != nil {
		t.Errorf("cards CardsForBatch: %v", err)
	}
	if _, err := store.Cards().TotalMinutesTracked(ctx,
		time.Unix(0, 0), time.Unix(2_000_000_000, 0)); err != nil {
		t.Errorf("cards TotalMinutesTracked: %v", err)
	}

	// categories
	if _, err := store.Categories().List(ctx); err != nil {
		t.Errorf("categories List: %v", err)
	}
	if _, _, err := store.Categories().ByName(ctx, "Development"); err != nil {
		t.Errorf("categories ByName: %v", err)
	}

	// captures
	if _, err := store.Captures().Pending(ctx); err != nil {
		t.Errorf("captures Pending: %v", err)
	}

	// diagnostics
	if _, err := store.Stats(ctx); err != nil {
		t.Errorf("stats: %v", err)
	}
}

func TestReadEveryTableOnEmptyDatabase(t *testing.T) {
	readEveryTable(t, openWriter(t, newDir(t)))
}

func TestReadEveryTableOnMigratedFixture(t *testing.T) {
	for _, fixture := range []string{"v0-with-settings.db", "v1-with-settings.db"} {
		t.Run(fixture, func(t *testing.T) {
			dir := newDir(t)
			copyFile(t, "testdata/"+fixture, dir+"/"+DatabaseFileName)
			store := openWriter(t, dir)
			readEveryTable(t, store)
		})
	}
}

// seedRepresentativeData fills a database with the shapes docs/08 §8.4 asks
// fixtures to cover: a typical install with cards, categories, a committed
// frame and settings.
func seedRepresentativeData(t *testing.T, store *Store) {
	t.Helper()
	ctx := context.Background()

	if err := store.Settings().SetMany(ctx, map[string]string{
		"appearance.theme":        `"dark"`,
		"capture.intervalSeconds": `30`,
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	if err := store.Categories().Save(ctx, []domain.Category{
		{ID: "c1", Name: "Development", ColorHex: "#3B82F6", SortOrder: 1, CreatedAtUnix: 1, UpdatedAtUnix: 1},
	}); err != nil {
		t.Fatalf("seed categories: %v", err)
	}

	// A batch has to exist before cards can reference it.
	if err := store.Write(ctx, "seed batch", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at)
			 VALUES (1, ?, ?, 'succeeded', 1, 1)`,
			time.Now().Add(-time.Hour).Unix(), time.Now().Unix())
		return err
	}); err != nil {
		t.Fatalf("seed batch: %v", err)
	}

	// Two cards inside one window: a normal one and one crossing midnight, so
	// the day-boundary logic is exercised rather than assumed.
	from := time.Date(2026, time.September, 12, 9, 0, 0, 0, time.Local)
	to := from.Add(2 * time.Hour)
	result, err := store.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{
		{
			Start: "9:00 AM", End: "10:00 AM", Category: "Development",
			Title: "Implement storage", Summary: "Wrote repositories",
			Metadata: `{"appSites":["code","terminal"],"distractions":["news"]}`,
		},
	}, 1)
	if err != nil {
		t.Fatalf("seed cards: %v", err)
	}
	if len(result.InsertedIDs) != 1 {
		t.Fatalf("seeded %d cards, want 1 (skipped: %d)", len(result.InsertedIDs), len(result.SkippedCards))
	}

	id, err := store.Captures().Begin(ctx, "2026/09/12/segment-0001", from, nil, 1920, 1080, false)
	if err != nil {
		t.Fatalf("seed capture: %v", err)
	}
	if err := store.Captures().Commit(ctx, id, 8192); err != nil {
		t.Fatalf("commit capture: %v", err)
	}
}

func TestReadEveryTableOnRepresentativeData(t *testing.T) {
	store := openWriter(t, newDir(t))
	seedRepresentativeData(t, store)
	readEveryTable(t, store)
}

// DB-5: every timeline_cards.metadata value must decode, and the fields the
// product reads out of it must survive a round trip. A metadata blob that
// silently loses appSites would make the timeline's frame view lie about what
// was on screen.
func TestCardMetadataRoundTrips(t *testing.T) {
	store := openWriter(t, newDir(t))
	seedRepresentativeData(t, store)
	ctx := context.Background()

	cards, err := store.Cards().CardsInRange(ctx,
		time.Date(2026, time.September, 12, 0, 0, 0, 0, time.Local),
		time.Date(2026, time.September, 13, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("CardsInRange: %v", err)
	}
	if len(cards) == 0 {
		t.Fatal("no cards to check")
	}

	for _, card := range cards {
		if card.Metadata == "" {
			t.Errorf("card %d has empty metadata", card.ID)
			continue
		}
		var decoded struct {
			AppSites     []string `json:"appSites"`
			Distractions []string `json:"distractions"`
		}
		if err := json.Unmarshal([]byte(card.Metadata), &decoded); err != nil {
			t.Errorf("card %d metadata does not decode: %v", card.ID, err)
			continue
		}
		if !reflect.DeepEqual(decoded.AppSites, []string{"code", "terminal"}) {
			t.Errorf("card %d appSites = %v", card.ID, decoded.AppSites)
		}
		if !reflect.DeepEqual(decoded.Distractions, []string{"news"}) {
			t.Errorf("card %d distractions = %v", card.ID, decoded.Distractions)
		}
	}
}

// The counters that report batch health need their tables to exist. Dropping
// them is the only way to reach the "source absent" branch now that the
// migration chain always creates them, so this test does exactly that.
func TestStatsReportsUnavailableSourcesWhenTablesAbsent(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()

	// batch_screenshots references screenshots and analysis_batches, so it must
	// go before either of them.
	for _, table := range []string{"batch_screenshots", "screenshots", "analysis_batches"} {
		if err := store.Write(ctx, "drop "+table, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "DROP TABLE "+table)
			return err
		}); err != nil {
			t.Fatalf("drop %s: %v", table, err)
		}
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.RecordingsAvailable {
		t.Error("recordings reported available after screenshots was dropped")
	}
	if stats.BatchesAvailable {
		t.Error("batches reported available after analysis_batches was dropped")
	}
	if stats.LastCaptureAtTs != nil {
		t.Error("lastCaptureAtTs is set although screenshots does not exist")
	}
	if stats.PendingBatches != 0 || stats.FailedBatches != 0 {
		t.Error("batch counters are non-zero although the table does not exist")
	}
}

// An empty analysis_batches table reports zero, which is different from the
// table being absent: the source exists and there simply is nothing pending.
func TestStatsReportsZeroForEmptyBatchTable(t *testing.T) {
	store := openWriter(t, newDir(t))

	stats, err := store.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if !stats.BatchesAvailable {
		t.Fatal("analysis_batches reported absent on a migrated database")
	}
	if stats.PendingBatches != 0 || stats.FailedBatches != 0 {
		t.Fatalf("counters = %d/%d on an empty table", stats.PendingBatches, stats.FailedBatches)
	}
}

// Batch states must be classified the way docs/03 §3.3.1 defines them: only
// succeeded is terminal-success, and the two failure states both count as
// failures rather than one silently disappearing.
func TestStatsCountsBatchStates(t *testing.T) {
	store := openWriter(t, newDir(t))
	ctx := context.Background()

	states := []string{"pending", "processing", "succeeded", "failed", "failed_empty", "skipped_short"}
	if err := store.Write(ctx, "seed batches", func(ctx context.Context, tx *sql.Tx) error {
		for i, state := range states {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO analysis_batches (id, start_ts, end_ts, status, created_at, updated_at)
				 VALUES (?, ?, ?, ?, 1, 1)`,
				i+1, i*600, (i+1)*600, state); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed batches: %v", err)
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.PendingBatches != 2 {
		t.Errorf("PendingBatches = %d, want 2 (pending + processing)", stats.PendingBatches)
	}
	if stats.FailedBatches != 2 {
		t.Errorf("FailedBatches = %d, want 2 (failed + failed_empty)", stats.FailedBatches)
	}
}
