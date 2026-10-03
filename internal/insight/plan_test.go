package insight

import (
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/storage"
)

/*
 * Fixture day 2026-09-12 in Asia/Shanghai. One plan block 10:00–11:00 for
 * "Coding". Cards (expected values worked out by hand):
 *
 *   09:30–10:30  Coding                    → 30 min inside the block
 *   10:30–10:45  Distraction               → 15 min distraction
 *   10:45–11:30  Coding, embedded distraction 10:50–10:55
 *                                           → 15 min inside, 5 min distraction
 *   10:40–10:44  Distraction-in-metadata on a Distraction card (10:40–10:44,
 *                 already inside the 10:30–10:45 card) → counted once
 */
func planFixture(t *testing.T) (*time.Location, func(h, m int) int64, []storage.CardSpan) {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	at := func(h, m int) int64 { return time.Date(2026, 9, 12, h, m, 0, 0, loc).Unix() }
	spans := []storage.CardSpan{
		{Day: "2026-09-12", StartTs: at(9, 30), EndTs: at(10, 30), Category: "Coding"},
		{Day: "2026-09-12", StartTs: at(10, 30), EndTs: at(10, 45), Category: "Distraction",
			Metadata: `{"distractions":[{"startTime":"10:40 AM","endTime":"10:44 AM"}]}`},
		{Day: "2026-09-12", StartTs: at(10, 45), EndTs: at(11, 30), Category: "Coding",
			Metadata: `{"distractions":[{"startTime":"10:50 AM","endTime":"10:55 AM"}]}`},
	}
	return loc, at, spans
}

func TestPlanBlockCoverageAfterTheBlock(t *testing.T) {
	loc, at, spans := planFixture(t)
	got := PlanBlockCoverage(at(10, 0), at(11, 0), at(12, 0), "Coding", spans, IsDistractionCategory, loc)
	if got.MatchedMinutes != 45 || got.DistractionMinutes != 20 {
		t.Fatalf("coverage = %+v, want 45 matched (30 + 15) and 20 distraction (15 + 5, overlap counted once)", got)
	}
}

func TestPlanBlockCoverageStopsAtNow(t *testing.T) {
	loc, at, spans := planFixture(t)
	got := PlanBlockCoverage(at(10, 0), at(11, 0), at(10, 40), "Coding", spans, IsDistractionCategory, loc)
	if got.MatchedMinutes != 30 || got.DistractionMinutes != 10 {
		t.Fatalf("coverage at 10:40 = %+v, want 30 matched and 10 distraction so far", got)
	}
	before := PlanBlockCoverage(at(10, 0), at(11, 0), at(9, 0), "Coding", spans, IsDistractionCategory, loc)
	if before.MatchedMinutes != 0 || before.DistractionMinutes != 0 {
		t.Fatalf("coverage before the block = %+v, want zero", before)
	}
}

func TestPlanBlockCoverageWithoutCategoryAndWithCustomDistraction(t *testing.T) {
	loc, at, spans := planFixture(t)
	none := PlanBlockCoverage(at(10, 0), at(11, 0), at(12, 0), "", spans, IsDistractionCategory, loc)
	if none.MatchedMinutes != 0 || none.DistractionMinutes != 20 {
		t.Fatalf("uncategorized block = %+v, want 0 matched and the same 20 distraction", none)
	}
	// A user who marks Coding itself as a distraction category (the day goal's
	// distraction set) sees all card time in the block as distraction.
	custom := func(name string) bool { return IsDistractionCategory(name) || name == "Coding" }
	got := PlanBlockCoverage(at(10, 0), at(11, 0), at(12, 0), "Coding", spans, custom, loc)
	if got.DistractionMinutes != 60 {
		t.Fatalf("custom distraction = %+v, want the whole hour", got)
	}
}

func TestDistractionMinutesOverADay(t *testing.T) {
	loc, at, spans := planFixture(t)
	if got := DistractionMinutes(at(4, 0), at(23, 0), spans, IsDistractionCategory, loc); got != 20 {
		t.Fatalf("day distraction = %v, want 20", got)
	}
	if !IsDistractionCategory(" distractions ") || !IsDistractionCategory("Distraction") || IsDistractionCategory("Coding") {
		t.Fatal("IsDistractionCategory must follow the built-in name rule")
	}
}

func TestPlanReviewsUseTheGoalDistractionSet(t *testing.T) {
	loc, at, spans := planFixture(t)
	blocks := []storage.PlanBlock{
		{ID: 1, StartTs: at(10, 0), EndTs: at(11, 0), CategoryName: "Coding"},
		{ID: 2, StartTs: at(11, 0), EndTs: at(12, 0)},
	}
	reviews := PlanReviews(blocks, spans, nil, at(12, 0), loc)
	if len(reviews) != 2 || reviews[0].Coverage.MatchedMinutes != 45 || reviews[0].Coverage.DistractionMinutes != 20 {
		t.Fatalf("reviews = %+v, want the first block at 45 matched / 20 distraction", reviews)
	}
	if reviews[1].Block.ID != 2 || reviews[1].Coverage != (PlanCoverage{}) {
		t.Fatalf("second block = %+v, want no coverage (11:00–11:30 is Coding without distraction, no category)", reviews[1])
	}
	withGoal := PlanReviews(blocks, spans, []string{"Coding"}, at(12, 0), loc)
	if withGoal[1].Coverage.DistractionMinutes != 30 {
		t.Fatalf("goal distraction set = %+v, want 11:00–11:30 Coding counted as distraction", withGoal[1])
	}
}
