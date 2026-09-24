package analysis

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
)

// mergeableSingleCardPredecessor is the deterministic gate behind the short
// single-card recovery merge (docs/04 §4.3.1). Its boundaries are the contract,
// so each one gets a fixture rather than trusting the end-to-end pipeline.
func TestMergeableSingleCardPredecessor(t *testing.T) {
	loc := time.Local
	at := func(h, m int) time.Time { return time.Date(2026, 9, 12, h, m, 0, 0, loc) }
	card := func(start, end time.Time, category string) domain.TimelineCard {
		return domain.TimelineCard{StartTs: start.Unix(), EndTs: end.Unix(), Category: category}
	}

	tests := []struct {
		name     string
		cStart   time.Time
		cEnd     time.Time
		existing []domain.TimelineCard
		wantOK   bool
	}{
		{
			name:   "adjacent cross-category short card merges",
			cStart: at(10, 15), cEnd: at(10, 20), // 5min
			existing: []domain.TimelineCard{card(at(10, 0), at(10, 14), "Coding")},
			wantOK:   true,
		},
		{
			name:   "card just under the thirteen-minute ceiling merges",
			cStart: at(10, 15), cEnd: at(10, 27), // 12min
			existing: []domain.TimelineCard{card(at(10, 12), at(10, 14), "Coding")},
			wantOK:   true,
		},
		{
			name:   "card at the thirteen-minute ceiling does not merge",
			cStart: at(10, 15), cEnd: at(10, 28), // exactly 13min
			existing: []domain.TimelineCard{card(at(10, 12), at(10, 14), "Coding")},
			wantOK:   false,
		},
		{
			name:   "no predecessor",
			cStart: at(10, 15), cEnd: at(10, 20),
			existing: nil,
			wantOK:   false,
		},
		{
			name:   "idle predecessor never absorbs real activity",
			cStart: at(10, 15), cEnd: at(10, 20),
			existing: []domain.TimelineCard{card(at(10, 0), at(10, 14), "Idle")},
			wantOK:   false,
		},
		{
			name:   "system predecessor never absorbs real activity",
			cStart: at(10, 15), cEnd: at(10, 20),
			existing: []domain.TimelineCard{card(at(10, 0), at(10, 14), "System")},
			wantOK:   false,
		},
		{
			name:   "gap over four minutes stays alone (the isolated fresh card)",
			cStart: at(10, 15), cEnd: at(10, 20),
			existing: []domain.TimelineCard{card(at(9, 55), at(10, 10), "Coding")}, // 5min gap
			wantOK:   false,
		},
		{
			name:   "merged span over sixty minutes does not merge",
			cStart: at(10, 15), cEnd: at(10, 20),
			existing: []domain.TimelineCard{card(at(9, 19), at(10, 14), "Coding")}, // 61min span
			wantOK:   false,
		},
		{
			name:   "a later card is not a predecessor",
			cStart: at(10, 15), cEnd: at(10, 20),
			existing: []domain.TimelineCard{
				card(at(10, 0), at(10, 14), "Coding"),  // the real predecessor
				card(at(10, 25), at(10, 40), "Coding"), // ends after C: ignored
			},
			wantOK: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pred, ok := mergeableSingleCardPredecessor(tc.cStart, tc.cEnd, tc.existing)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (pred=%+v)", ok, tc.wantOK, pred)
			}
		})
	}
}

// mergeableSmallPredecessor is the mirror gate (docs/04 §4.3.1): a newly
// generated card of any length absorbs a short committed predecessor left beside
// the rewrite by the ownership gate. It shares adjacentMergeablePredecessor's
// bounds with mergeableSingleCardPredecessor but gates on the *predecessor's*
// duration, not the output's, so each boundary gets its own fixture.
func TestMergeableSmallPredecessor(t *testing.T) {
	loc := time.Local
	at := func(h, m int) time.Time { return time.Date(2026, 9, 12, h, m, 0, 0, loc) }
	card := func(start, end time.Time, category string) domain.TimelineCard {
		return domain.TimelineCard{StartTs: start.Unix(), EndTs: end.Unix(), Category: category}
	}

	tests := []struct {
		name     string
		cStart   time.Time
		cEnd     time.Time
		existing []domain.TimelineCard
		wantOK   bool
	}{
		{
			name:   "long new card absorbs a short cross-category predecessor",
			cStart: at(10, 15), cEnd: at(10, 45), // 30min output, length is not the gate
			existing: []domain.TimelineCard{card(at(10, 9), at(10, 14), "Communication")}, // 5min
			wantOK:   true,
		},
		{
			name:   "predecessor just under the thirteen-minute ceiling merges",
			cStart: at(10, 15), cEnd: at(10, 45),
			existing: []domain.TimelineCard{card(at(10, 2), at(10, 14), "Communication")}, // 12min
			wantOK:   true,
		},
		{
			name:   "predecessor at the thirteen-minute ceiling does not merge",
			cStart: at(10, 15), cEnd: at(10, 45),
			existing: []domain.TimelineCard{card(at(10, 1), at(10, 14), "Communication")}, // exactly 13min
			wantOK:   false,
		},
		{
			name:   "no predecessor",
			cStart: at(10, 15), cEnd: at(10, 45),
			existing: nil,
			wantOK:   false,
		},
		{
			name:   "idle predecessor is never absorbed",
			cStart: at(10, 15), cEnd: at(10, 45),
			existing: []domain.TimelineCard{card(at(10, 9), at(10, 14), "Idle")},
			wantOK:   false,
		},
		{
			name:   "system predecessor is never absorbed",
			cStart: at(10, 15), cEnd: at(10, 45),
			existing: []domain.TimelineCard{card(at(10, 9), at(10, 14), "System")},
			wantOK:   false,
		},
		{
			name:   "gap over four minutes leaves the fragment alone",
			cStart: at(10, 15), cEnd: at(10, 45),
			existing: []domain.TimelineCard{card(at(10, 5), at(10, 10), "Communication")}, // 5min gap
			wantOK:   false,
		},
		{
			name:   "merged span over sixty minutes does not merge",
			cStart: at(9, 8), cEnd: at(10, 10), // combined 70min from the predecessor start
			existing: []domain.TimelineCard{card(at(9, 0), at(9, 5), "Communication")},
			wantOK:   false,
		},
		{
			name:   "a later card is not a predecessor",
			cStart: at(10, 15), cEnd: at(10, 45),
			existing: []domain.TimelineCard{
				card(at(10, 9), at(10, 14), "Communication"), // the real predecessor
				card(at(10, 50), at(11, 0), "Coding"),        // ends after C: ignored
			},
			wantOK: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pred, ok := mergeableSmallPredecessor(tc.cStart, tc.cEnd, tc.existing)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (pred=%+v)", ok, tc.wantOK, pred)
			}
		})
	}
}

// buildMergedShell must give the merged card the majority-time activity's
// identity (docs/04 §4.3.4) while keeping the outer span [pred.Start, short.End]
// and unioning both cards' metadata so no evidence is dropped.
func TestBuildMergedShellMajorityCategory(t *testing.T) {
	loc := time.Local
	at := func(h, m int) time.Time { return time.Date(2026, 9, 12, h, m, 0, 0, loc) }
	pred := domain.TimelineCard{
		StartTs: at(10, 0).Unix(), EndTs: at(10, 14).Unix(),
		Start: "10:00 AM", End: "10:14 AM", Category: "Coding", Title: "coding",
		Summary:  "wrote code",
		Metadata: `{"appSites":{"primary":"Code"},"distractions":[],"activityPoints":[{"time":"10:05 AM","description":"a"}]}`,
	}
	short := domain.CardShell{
		Start: "10:15 AM", End: "10:20 AM", Category: "Communication", Title: "chat",
		Summary:  "replied",
		Metadata: `{"appSites":null,"distractions":[],"activityPoints":[{"time":"10:16 AM","description":"b"}]}`,
	}

	// Predecessor (14min) outlasts the short card (5min): its identity wins, the
	// span runs 10:00 AM–10:20 AM, and both activity points survive in order.
	merged := buildMergedShell(pred, short, 5*time.Minute)
	if merged.Category != "Coding" || merged.Title != "coding" {
		t.Fatalf("merged identity = %s/%s, want Coding/coding", merged.Category, merged.Title)
	}
	if merged.Start != "10:00 AM" || merged.End != "10:20 AM" {
		t.Fatalf("merged span = %s–%s, want 10:00 AM–10:20 AM", merged.Start, merged.End)
	}
	var meta struct {
		AppSites       *appSitesMetadata   `json:"appSites"`
		ActivityPoints []cardActivityPoint `json:"activityPoints"`
	}
	if err := json.Unmarshal([]byte(merged.Metadata), &meta); err != nil {
		t.Fatalf("decode merged metadata: %v", err)
	}
	if meta.AppSites == nil || meta.AppSites.Primary == nil || *meta.AppSites.Primary != "Code" {
		t.Fatalf("merged appSites = %+v, want the predecessor's Code", meta.AppSites)
	}
	if len(meta.ActivityPoints) != 2 || meta.ActivityPoints[0].Description != "a" || meta.ActivityPoints[1].Description != "b" {
		t.Fatalf("merged points = %+v, want both cards' points in order", meta.ActivityPoints)
	}

	// Short card outlasts a tiny predecessor: its identity wins, and it inherits
	// the predecessor's appSites because it named none of its own.
	tiny := pred
	tiny.EndTs = at(10, 3).Unix() // 3min predecessor
	merged = buildMergedShell(tiny, short, 8*time.Minute)
	if merged.Category != "Communication" || merged.Title != "chat" {
		t.Fatalf("merged identity = %s/%s, want Communication/chat", merged.Category, merged.Title)
	}
	if err := json.Unmarshal([]byte(merged.Metadata), &meta); err != nil {
		t.Fatalf("decode merged metadata: %v", err)
	}
	if meta.AppSites == nil || meta.AppSites.Primary == nil || *meta.AppSites.Primary != "Code" {
		t.Fatalf("merged appSites = %+v, want inherited Code when the majority named none", meta.AppSites)
	}
}
