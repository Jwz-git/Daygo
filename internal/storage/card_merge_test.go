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

func mergeFixture(t *testing.T, cards []domain.CardShell) (*Store, []int64) {
	t.Helper()
	s := openWriterAt(t, t.TempDir(), "Asia/Shanghai")
	seedBatch(t, s, 1)
	from, to := window(s.location(), 9, 0, 14, 0)
	result, err := s.Cards().ReplaceCardsInRange(context.Background(), from, to, cards, 1)
	if err != nil || len(result.InsertedIDs) != len(cards) {
		t.Fatalf("seed=%+v err=%v", result, err)
	}
	return s, result.InsertedIDs
}

func TestMergeCardsAtomicAndPreservesSources(t *testing.T) {
	s, ids := mergeFixture(t, []domain.CardShell{
		{Start: "10:00 AM", End: "10:20 AM", Category: "Focus Work", Title: "first", Summary: "first summary", VideoSummaryPath: "anonymous.mp4", Metadata: `{"activityPoints":[{"time":"10:05 AM","description":"first"}],"appSites":{"primary":"Editor"}}`},
		{Start: "10:20 AM", End: "10:40 AM", Category: "Personal", Title: "second", Summary: "second summary", Metadata: `{"activityPoints":[{"time":"10:25 AM","description":"second"}],"distractions":[{"startTime":"10:26 AM","endTime":"10:27 AM","title":"anonymous"}],"appSites":{"primary":"Browser"}}`},
		{Start: "11:00 AM", End: "11:20 AM", Category: "Focus Work", Title: "untouched"},
	})
	ctx := context.Background()
	if err := s.Reviews().SetVerdict(ctx, ids[0], "focus", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.Reviews().SetRating(ctx, ids[1], RatingUp, time.Now()); err != nil {
		t.Fatal(err)
	}
	merged, err := s.Cards().MergeCards(ctx, []int64{ids[1], ids[0]}, domain.CardShell{Title: "Combined", Summary: "Combined summary", DetailedSummary: "Detail", Category: "Focus Work"})
	if err != nil {
		t.Fatal(err)
	}
	if merged.ID == ids[0] || merged.ID == ids[1] || merged.Start != "10:00 AM" || merged.End != "10:40 AM" || merged.DurationMinutes() != 40 || merged.Title != "Combined" || merged.Category != "Focus Work" || merged.DetailedSummary != "Detail" || merged.VideoSummaryPath != "" || merged.BatchID == nil {
		t.Fatalf("merged=%+v", merged)
	}
	var meta struct {
		Points       []json.RawMessage `json:"activityPoints"`
		Distractions []json.RawMessage `json:"distractions"`
		SourceIDs    []int64           `json:"mergedCardIds"`
	}
	if err := json.Unmarshal([]byte(merged.Metadata), &meta); err != nil {
		t.Fatal(err)
	}
	if len(meta.Points) != 2 || len(meta.Distractions) != 1 || !reflect.DeepEqual(meta.SourceIDs, ids[:2]) {
		t.Fatalf("metadata=%s", merged.Metadata)
	}
	for _, id := range ids[:2] {
		if _, err := s.Cards().CardByID(ctx, id); !IsKind(err, KindNotFound) {
			t.Fatalf("source %d live: %v", id, err)
		}
	}
	if _, err := s.Cards().CardByID(ctx, ids[2]); err != nil {
		t.Fatal(err)
	}
	var title, video string
	var edited, deleted int
	if err := s.Read(ctx, "verify source", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, "SELECT title, video_summary_path, is_deleted, is_user_edited FROM timeline_cards WHERE id=?", ids[0]).Scan(&title, &video, &deleted, &edited)
	}); err != nil {
		t.Fatal(err)
	}
	if title != "first" || video != "anonymous.mp4" || deleted != 1 || edited != 1 {
		t.Fatalf("source changed: %s %s %d %d", title, video, deleted, edited)
	}
	if err := s.Read(ctx, "verify original feedback", func(ctx context.Context, tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM card_reviews WHERE card_id=?)+(SELECT count(*) FROM card_ratings WHERE card_id=?)", ids[0], ids[1]).Scan(&count); err != nil {
			return err
		}
		if count != 2 {
			t.Errorf("preserved feedback rows=%d", count)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if verdict, err := s.Reviews().Verdict(ctx, merged.ID); err != nil || verdict != "" {
		t.Fatalf("inherited verdict=%s err=%v", verdict, err)
	}
	if rating, err := s.Reviews().Rating(ctx, merged.ID); err != nil || rating != "" {
		t.Fatalf("inherited rating=%s err=%v", rating, err)
	}
	from, to := window(s.location(), 10, 0, 10, 40)
	if _, err := s.Cards().ReplaceGeneratedCardsInRange(ctx, from, to, []domain.CardShell{shell("10:00 AM", "10:40 AM", "Focus Work", "automatic")}, 1); err != nil {
		t.Fatal(err)
	}
	kept, err := s.Cards().CardByID(ctx, merged.ID)
	if err != nil || kept.Title != "Combined" {
		t.Fatalf("auto rewrite lost merge=%+v err=%v", kept, err)
	}
	if _, err := s.Cards().MergeCards(ctx, ids[:2], domain.CardShell{Title: "Again", Summary: "s", Category: "Focus Work"}); !IsKind(err, KindNotFound) {
		t.Fatalf("replay=%v", err)
	}
}

func TestMergeCardsRejectsWithoutPartialWrites(t *testing.T) {
	for _, name := range []string{"gap", "unselected overlap", "too long", "duplicate", "missing", "built-in source", "built-in target", "unknown target", "empty title", "invalid metadata", "one card", "too many", "rollback"} {
		t.Run(name, func(t *testing.T) {
			cards := []domain.CardShell{shell("10:00 AM", "10:20 AM", "Focus Work", "one"), shell("10:20 AM", "10:40 AM", "Personal", "two")}
			if name == "gap" {
				cards[1].Start = "10:21 AM"
			}
			if name == "unselected overlap" {
				cards = append(cards, shell("10:10 AM", "10:30 AM", "Focus Work", "other"))
			}
			if name == "too long" {
				cards[0].Start = "9:00 AM"
				cards[1].End = "1:01 PM"
			}
			if name == "built-in source" {
				cards[0].Category = "System"
			}
			if name == "invalid metadata" {
				cards[0].Metadata = "broken"
			}
			s, ids := mergeFixture(t, cards)
			input := ids[:2]
			content := domain.CardShell{Title: "Merged", Summary: "s", Category: "Focus Work"}
			switch name {
			case "duplicate":
				input = []int64{ids[0], ids[0]}
			case "missing":
				input = []int64{ids[0], 9999}
			case "built-in target":
				content.Category = "Idle"
			case "unknown target":
				content.Category = "invented"
			case "empty title":
				content.Title = " "
			case "one card":
				input = ids[:1]
			case "too many":
				input = make([]int64, 33)
			case "rollback":
				if err := s.Write(context.Background(), "inject failure", func(ctx context.Context, tx *sql.Tx) error {
					_, err := tx.ExecContext(ctx, `CREATE TRIGGER reject_merge BEFORE INSERT ON timeline_cards WHEN NEW.is_user_edited=1 BEGIN SELECT RAISE(ABORT, 'anonymous fixture failure'); END`)
					return err
				}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.Cards().MergeCards(context.Background(), input, content); err == nil {
				t.Fatal("invalid merge succeeded")
			}
			for i, id := range ids {
				got, err := s.Cards().CardByID(context.Background(), id)
				if err != nil || got.Title != cards[i].Title {
					t.Fatalf("partial merge: card=%+v err=%v", got, err)
				}
			}
		})
	}
}

func TestMergeCardsCrossFourAMUsesPersistedTimestamps(t *testing.T) {
	s := openWriterAt(t, t.TempDir(), "Asia/Kathmandu")
	seedBatch(t, s, 1)
	ctx := context.Background()
	from := time.Date(2026, 9, 12, 3, 30, 0, 0, s.location())
	to := from.Add(time.Hour)
	res, err := s.Cards().ReplaceCardsInRange(ctx, from, to, []domain.CardShell{shell("3:30 AM", "4:00 AM", "Focus Work", "first"), shell("4:00 AM", "4:30 AM", "Focus Work", "second")}, 1)
	if err != nil {
		t.Fatal(err)
	}
	card, err := s.Cards().MergeCards(ctx, res.InsertedIDs, domain.CardShell{Title: "Merged", Summary: "s", Category: "Focus Work"})
	if err != nil || card.Day != "2026-09-11" || card.StartTs != from.Unix() || card.EndTs != to.Unix() {
		t.Fatalf("card=%+v err=%v", card, err)
	}
	for _, day := range []string{"2026-09-11", "2026-09-12"} {
		cards, err := s.Cards().CardsForDay(ctx, day)
		if err != nil || len(cards) != 1 || cards[0].ID != card.ID {
			t.Fatalf("day=%s cards=%+v err=%v", day, cards, err)
		}
	}
}

func TestMergeCardsConcurrentReplayCommitsOnlyOnce(t *testing.T) {
	s, ids := mergeFixture(t, []domain.CardShell{shell("10:00 AM", "10:20 AM", "Focus Work", "one"), shell("10:20 AM", "10:40 AM", "Focus Work", "two")})
	start := make(chan struct{})
	done := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := s.Cards().MergeCards(context.Background(), ids, domain.CardShell{Title: "Merged", Summary: "s", Category: "Focus Work"})
			done <- err
		}()
	}
	close(start)
	succeeded := 0
	for range 2 {
		if err := <-done; err == nil {
			succeeded++
		} else if !IsKind(err, KindNotFound) {
			t.Fatal(err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful commits=%d", succeeded)
	}
}

func TestMergeCardsPreservesAmbiguousDSTInstants(t *testing.T) {
	s := openWriterAt(t, t.TempDir(), "America/New_York")
	ctx := context.Background()
	from, err := time.Parse(time.RFC3339, "2026-11-01T01:30:00-04:00")
	if err != nil {
		t.Fatal(err)
	}
	// Both endpoints say 01:30, one hour apart. Re-parsing clocks would lose
	// that interval; the merge must use the already-disambiguated instants.
	if err := s.Write(ctx, "seed ambiguous DST cards", func(ctx context.Context, tx *sql.Tx) error {
		for i, clocks := range [][2]string{{"1:30 AM", "1:00 AM"}, {"1:00 AM", "1:30 AM"}} {
			_, err := tx.ExecContext(ctx, `INSERT INTO timeline_cards(day,start,end,start_ts,end_ts,category,title,summary,created_at,updated_at)
			VALUES('2026-10-31',?,?,?,?, 'Focus Work','anonymous','s',1,1)`, clocks[0], clocks[1], from.Add(time.Duration(i)*30*time.Minute).Unix(), from.Add(time.Duration(i+1)*30*time.Minute).Unix())
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	merged, err := s.Cards().MergeCards(ctx, []int64{1, 2}, domain.CardShell{Title: "Merged", Summary: "s", Category: "Focus Work"})
	if err != nil || merged.StartTs != from.Unix() || merged.EndTs != from.Add(time.Hour).Unix() || merged.Day != "2026-10-31" || merged.DurationMinutes() != 60 {
		t.Fatalf("merged=%+v err=%v", merged, err)
	}
}
