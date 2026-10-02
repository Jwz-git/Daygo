package app

import (
	"context"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

func TestGetTokenUsageContract(t *testing.T) {
	b, _ := backendWithStore(t)
	loc := b.store().Location()
	start, end, err := timeutil.DayWindow("2026-09-07", loc)
	if err != nil {
		t.Fatal(err)
	}
	input, output := int64(100), int64(20)
	for _, at := range []time.Time{start.Add(-time.Second), start, start.Add(time.Hour), end} {
		err = b.store().LlmCalls().Insert(context.Background(), storage.LlmCall{
			Purpose: "chat", AttemptNo: 1, ProviderID: "fixture", Protocol: "openai", RequestedModel: "fixture",
			StartedAt: at, FinishedAt: at, Outcome: "succeeded", InputTokens: &input, OutputTokens: &output,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	dto, err := b.GetTokenUsage("day", "2026-09-07")
	if err != nil {
		t.Fatal(err)
	}
	if dto.TimeZone != timeutil.ZoneName(loc) {
		t.Fatalf("timeZone = %q, want Intl-compatible %q", dto.TimeZone, timeutil.ZoneName(loc))
	}
	if len(dto.Buckets) != 24 || dto.Buckets[0].InputTokens != 100 || dto.Buckets[1].InputTokens != 100 || dto.Buckets[23].Calls != 0 {
		t.Fatalf("dto=%+v", dto)
	}
	week, err := b.GetTokenUsage("week", "2026-09-07")
	if err != nil || len(week.Buckets) != 7 || week.Buckets[0].Calls != 2 || week.Buckets[1].Calls != 1 {
		t.Fatalf("week=%+v err=%v", week, err)
	}
	for _, args := range [][2]string{{"month", "2026-09-07"}, {"week", "2026-09-08"}, {"day", "bad"}} {
		if _, err := b.GetTokenUsage(args[0], args[1]); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestTokenUsageDSTAndFractionalZones(t *testing.T) {
	for _, fixture := range []struct {
		zone, day string
		hours     int
	}{
		{"America/New_York", "2026-03-07", 23},
		{"America/New_York", "2026-10-31", 25},
		{"Asia/Kathmandu", "2026-09-07", 24},
		{"Australia/Lord_Howe", "2026-10-03", 24},
	} {
		t.Run(fixture.zone+"/"+fixture.day, func(t *testing.T) {
			loc, err := time.LoadLocation(fixture.zone)
			if err != nil {
				t.Fatal(err)
			}
			s, err := storage.Open(context.Background(), storage.Options{Dir: t.TempDir(), Location: loc})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			b := newBackend(fixedClock{}, nil, s, false, false)
			dto, err := b.GetTokenUsage("day", fixture.day)
			if err != nil {
				t.Fatal(err)
			}
			if len(dto.Buckets) != fixture.hours {
				t.Fatalf("buckets=%d want %d", len(dto.Buckets), fixture.hours)
			}
			start, end, err := timeutil.DayWindow(fixture.day, loc)
			if err != nil {
				t.Fatal(err)
			}
			if dto.Buckets[0].StartTs != start.Unix() || dto.Buckets[len(dto.Buckets)-1].EndTs != end.Unix() {
				t.Fatal("window mismatch")
			}
			for i, bucket := range dto.Buckets {
				if bucket.EndTs <= bucket.StartTs || bucket.EndTs-bucket.StartTs > 3600 {
					t.Fatalf("invalid bucket %+v", bucket)
				}
				if i > 0 && dto.Buckets[i-1].EndTs != bucket.StartTs {
					t.Fatal("gap or overlap")
				}
			}
			weekStart, err := timeutil.WeekStart(fixture.day, loc)
			if err != nil {
				t.Fatal(err)
			}
			week, err := b.GetTokenUsage("week", weekStart)
			if err != nil {
				t.Fatal(err)
			}
			if len(week.Buckets) != 7 {
				t.Fatal("week must have seven days")
			}
			for i, bucket := range week.Buckets {
				if time.Unix(bucket.StartTs, 0).In(loc).Hour() != 4 || time.Unix(bucket.EndTs, 0).In(loc).Hour() != 4 {
					t.Fatal("week boundary shifted")
				}
				if i > 0 && week.Buckets[i-1].EndTs != bucket.StartTs {
					t.Fatal("week gap or overlap")
				}
			}
		})
	}
}
