package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
)

func TestGeneratedRewriteSubtractsHumanAndDeletedIntervals(t *testing.T) {
	s := openWriter(t, t.TempDir())
	ctx := context.Background()
	at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	// Anonymous batches are required by the card FK.
	seedBatch(t, s, 1)
	old, err := s.Cards().ReplaceCardsInRange(ctx, at, at.Add(50*time.Minute), []domain.CardShell{
		{Start: "10:10 AM", End: "10:20 AM", Category: "Work", Title: "human", Summary: "s"},
		{Start: "10:30 AM", End: "10:40 AM", Category: "Work", Title: "deleted", Summary: "s"},
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Cards().UpdateCardTitle(ctx, old.InsertedIDs[0], "Human title"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Cards().SoftDeleteCard(ctx, old.InsertedIDs[1]); err != nil {
		t.Fatal(err)
	}
	generated := domain.CardShell{Start: "10:00 AM", End: "10:50 AM", Category: "Work", Title: "AI", Summary: "s", VideoSummaryPath: "anonymous.mp4",
		Metadata: `{"activityPoints":[{"time":"10:05 AM"},{"time":"10:15 AM"},{"time":"10:25 AM"},{"time":"10:35 AM"},{"time":"10:45 AM"}],"distractions":[{"startTime":"10:05 AM","endTime":"10:25 AM","title":"anonymous"}]}`}
	result, err := s.Cards().ReplaceGeneratedCardsInRange(ctx, at, at.Add(50*time.Minute), []domain.CardShell{generated}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.InsertedIDs) != 3 || len(result.DeletedVideoPaths) != 0 {
		t.Fatalf("rewrite=%+v", result)
	}
	kept, err := s.Cards().CardByID(ctx, old.InsertedIDs[0])
	if err != nil || kept.Title != "Human title" {
		t.Fatalf("kept=%+v err=%v", kept, err)
	}
	for i, want := range [][2]int{{0, 10}, {20, 30}, {40, 50}} {
		card, err := s.Cards().CardByID(ctx, result.InsertedIDs[i])
		if err != nil || card.StartTs != at.Add(time.Duration(want[0])*time.Minute).Unix() || card.EndTs != at.Add(time.Duration(want[1])*time.Minute).Unix() || card.VideoSummaryPath != "" {
			t.Fatalf("fragment=%+v err=%v", card, err)
		}
		var meta struct {
			Points []struct {
				Time string `json:"time"`
			} `json:"activityPoints"`
			Distractions []struct {
				Start string `json:"startTime"`
				End   string `json:"endTime"`
			} `json:"distractions"`
		}
		if err := json.Unmarshal([]byte(card.Metadata), &meta); err != nil {
			t.Fatal(err)
		}
		if len(meta.Points) != 1 {
			t.Fatalf("fragment points=%+v", meta.Points)
		}
		if i == 0 && (len(meta.Distractions) != 1 || meta.Distractions[0].Start != "10:05 AM" || meta.Distractions[0].End != "10:10 AM") {
			t.Fatalf("head distractions=%+v", meta.Distractions)
		}
		if i == 1 && (len(meta.Distractions) != 1 || meta.Distractions[0].Start != "10:20 AM" || meta.Distractions[0].End != "10:25 AM") {
			t.Fatalf("middle distractions=%+v", meta.Distractions)
		}
		if i == 2 && len(meta.Distractions) != 0 {
			t.Fatalf("tail distractions=%+v", meta.Distractions)
		}
	}
}

func TestGeneratedRewriteAfterClearingFeedback(t *testing.T) {
	for _, kind := range []string{"review", "rating"} {
		t.Run(kind, func(t *testing.T) {
			s := openWriter(t, t.TempDir())
			ctx := context.Background()
			at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
			seedBatch(t, s, 1)
			shells := []domain.CardShell{{Start: "10:00 AM", End: "10:15 AM", Category: "Work", Title: "original", Summary: "s"}}
			old, err := s.Cards().ReplaceGeneratedCardsInRange(ctx, at, at.Add(15*time.Minute), shells, 1)
			if err != nil {
				t.Fatal(err)
			}
			id := old.InsertedIDs[0]
			if kind == "review" {
				err = s.Reviews().SetVerdict(ctx, id, VerdictFocus, at)
			} else {
				err = s.Reviews().SetRating(ctx, id, RatingUp, at)
			}
			if err != nil {
				t.Fatal(err)
			}
			result, err := s.Cards().ReplaceGeneratedCardsInRange(ctx, at, at.Add(15*time.Minute), shells, 1)
			if err != nil || len(result.InsertedIDs) != 0 {
				t.Fatalf("protected=%+v err=%v", result, err)
			}
			if kind == "review" {
				err = s.Reviews().ClearVerdict(ctx, id)
			} else {
				err = s.Reviews().ClearRating(ctx, id)
			}
			if err != nil {
				t.Fatal(err)
			}
			result, err = s.Cards().ReplaceGeneratedCardsInRange(ctx, at, at.Add(15*time.Minute), shells, 1)
			if err != nil || len(result.InsertedIDs) != 1 {
				t.Fatalf("unprotected=%+v err=%v", result, err)
			}
		})
	}
}

func TestGeneratedRewriteRecomputesLogicalDaysAfterClipping(t *testing.T) {
	s := openWriter(t, t.TempDir())
	ctx := context.Background()
	at := time.Date(2026, 9, 12, 3, 40, 0, 0, time.Local)
	seedBatch(t, s, 1)
	old, err := s.Cards().ReplaceCardsInRange(ctx, at, at.Add(50*time.Minute), []domain.CardShell{{Start: "3:50 AM", End: "4:10 AM", Category: "Work", Title: "old", Summary: "s"}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Cards().UpdateCardTitle(ctx, old.InsertedIDs[0], "Human"); err != nil {
		t.Fatal(err)
	}
	result, err := s.Cards().ReplaceGeneratedCardsInRange(ctx, at, at.Add(50*time.Minute), []domain.CardShell{{Start: "3:40 AM", End: "4:30 AM", Category: "Work", Title: "new", Summary: "s"}}, 1)
	if err != nil || len(result.InsertedIDs) != 2 {
		t.Fatalf("rewrite=%+v err=%v", result, err)
	}
	for i, day := range []string{"2026-09-11", "2026-09-12"} {
		card, err := s.Cards().CardByID(ctx, result.InsertedIDs[i])
		if err != nil || card.Day != day {
			t.Fatalf("fragment=%+v err=%v", card, err)
		}
	}
}
