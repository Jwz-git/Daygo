package insight

import (
	"math"
	"testing"

	"github.com/Jwz-git/Daygo/internal/storage"
)

func TestAggregateWeeklyEmpty(t *testing.T) {
	got := AggregateWeekly(nil)
	if got.TrackedMinutes != 0 || got.FocusMinutes != 0 || len(got.Categories) != 0 {
		t.Fatalf("AggregateWeekly(nil) = %+v, want zero totals and empty categories", got)
	}
	// An all-System week is also an empty dashboard: share stays 0.
	got = AggregateWeekly([]storage.CategoryMinutes{{Name: "System", Minutes: 600}})
	if got.TrackedMinutes != 0 || len(got.Categories) != 0 {
		t.Fatalf("all-System week = %+v, want zero tracked and no categories", got)
	}
}

func TestAggregateWeeklyExclusions(t *testing.T) {
	got := AggregateWeekly([]storage.CategoryMinutes{
		{Name: "System", Minutes: 600},
		{Name: "Idle", IsIdle: true, Minutes: 120},
		{Name: "Coding", Minutes: 300},
		{Name: "Writing", Minutes: 100},
	})
	if got.TrackedMinutes != 520 {
		t.Fatalf("tracked = %v, want 520 (System excluded, Idle included)", got.TrackedMinutes)
	}
	if got.FocusMinutes != 400 {
		t.Fatalf("focus = %v, want 400 (System and Idle excluded)", got.FocusMinutes)
	}
	wantOrder := []string{"Coding", "Idle", "Writing"}
	if len(got.Categories) != len(wantOrder) {
		t.Fatalf("categories = %+v, want %d rows", got.Categories, len(wantOrder))
	}
	for i, name := range wantOrder {
		if got.Categories[i].Name != name {
			t.Fatalf("category[%d] = %q, want %q (minutes DESC)", i, got.Categories[i].Name, name)
		}
	}
	// Shares are against tracked: Coding 300/520, Idle 120/520, Writing 100/520.
	wantShares := []float64{300.0 / 520, 120.0 / 520, 100.0 / 520}
	for i, want := range wantShares {
		if math.Abs(got.Categories[i].Share-want) > 1e-9 {
			t.Fatalf("share[%d] = %v, want %v", i, got.Categories[i].Share, want)
		}
	}
}

func TestAggregateWeeklyTieBreakByName(t *testing.T) {
	got := AggregateWeekly([]storage.CategoryMinutes{
		{Name: "Writing", Minutes: 100},
		{Name: "Coding", Minutes: 100},
	})
	if got.Categories[0].Name != "Coding" || got.Categories[1].Name != "Writing" {
		t.Fatalf("tie order = %q, %q; want Coding, Writing (name tie-break)", got.Categories[0].Name, got.Categories[1].Name)
	}
}
