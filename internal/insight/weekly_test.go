package insight

import (
	"math"
	"reflect"
	"testing"

	"github.com/Jwz-git/Daygo/internal/storage"
)

func TestAggregateWeeklySpansMatchesCategoryMinutes(t *testing.T) {
	spans := []storage.CardSpan{
		{Day: "2026-11-16", StartTs: 3, EndTs: 31, Category: "Work", ColorHex: "#123456"},
		{Day: "2026-11-17", StartTs: 31, EndTs: 62, Category: "Work", ColorHex: "#123456"},
		{Day: "2026-11-17", StartTs: 62, EndTs: 63, Category: "Idle", IsIdle: true},
		{Day: "2026-11-17", StartTs: 63, EndTs: 130, Category: "System"},
	}
	want := AggregateWeekly([]storage.CategoryMinutes{
		{Name: "Idle", IsIdle: true, Minutes: 1.0 / 60},
		{Name: "System", Minutes: 67.0 / 60},
		{Name: "Work", ColorHex: "#123456", Minutes: 59.0 / 60},
	})
	if got := AggregateWeeklySpans(spans); !reflect.DeepEqual(got, want) {
		t.Fatalf("weekly spans = %+v, want %+v", got, want)
	}
}

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
		{Name: " Distraction ", Minutes: 50},
		{Name: "Coding", Minutes: 300},
		{Name: "Writing", Minutes: 100},
	})
	if got.TrackedMinutes != 570 {
		t.Fatalf("tracked = %v, want 570 (System excluded; Idle and Distraction included)", got.TrackedMinutes)
	}
	if got.FocusMinutes != 400 {
		t.Fatalf("focus = %v, want 400 (System, Idle and Distraction excluded)", got.FocusMinutes)
	}
	wantOrder := []string{"Coding", "Idle", "Writing", " Distraction "}
	if len(got.Categories) != len(wantOrder) {
		t.Fatalf("categories = %+v, want %d rows", got.Categories, len(wantOrder))
	}
	for i, name := range wantOrder {
		if got.Categories[i].Name != name {
			t.Fatalf("category[%d] = %q, want %q (minutes DESC)", i, got.Categories[i].Name, name)
		}
	}
	// Shares are against tracked: Coding 300/520, Idle 120/520, Writing 100/520.
	wantShares := []float64{300.0 / 570, 120.0 / 570, 100.0 / 570, 50.0 / 570}
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
