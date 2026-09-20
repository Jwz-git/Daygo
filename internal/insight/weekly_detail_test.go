package insight

import (
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/storage"
)

func detailFixture(loc *time.Location) []storage.CardSpan {
	at := func(day, hour, minute int) int64 {
		return time.Date(2026, 9, day, hour, minute, 0, 0, loc).Unix()
	}
	return []storage.CardSpan{
		// Monday: 2h Coding (focus) + 1h Idle.
		{Day: "2026-09-07", StartTs: at(7, 9, 0), EndTs: at(7, 11, 0), Category: "Coding", ColorHex: "#111111"},
		{Day: "2026-09-07", StartTs: at(7, 13, 0), EndTs: at(7, 14, 0), Category: "Idle", IsIdle: true, ColorHex: "#222222"},
		// Monday: a non-idle Distraction card is tracked but not focused.
		{Day: "2026-09-07", StartTs: at(7, 15, 0), EndTs: at(7, 15, 30), Category: "Distraction", ColorHex: "#444444"},
		// Tuesday: 1.5h Writing crossing the 11:00 hour boundary.
		{Day: "2026-09-08", StartTs: at(8, 10, 30), EndTs: at(8, 12, 0), Category: "Writing", ColorHex: "#333333"},
		// System excluded everywhere.
		{Day: "2026-09-08", StartTs: at(8, 15, 0), EndTs: at(8, 16, 0), Category: "System", ColorHex: "#8E8E93"},
		// A span outside the week is ignored.
		{Day: "2026-08-31", StartTs: at(1, 9, 0), EndTs: at(1, 10, 0), Category: "Coding", ColorHex: "#111111"},
	}
}

func TestBuildWeeklyDetailTotals(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	detail := BuildWeeklyDetail(detailFixture(loc), "2026-09-07", loc)

	if len(detail.Days) != 7 {
		t.Fatalf("days = %d, want 7", len(detail.Days))
	}
	if detail.Days[0].Day != "2026-09-07" || detail.Days[1].Day != "2026-09-08" {
		t.Fatalf("day order = %q, %q, want Mon then Tue", detail.Days[0].Day, detail.Days[1].Day)
	}

	monday := detail.Days[0]
	if monday.TrackedMinutes != 210 || monday.FocusMinutes != 120 {
		t.Fatalf("monday tracked/focus = %v/%v, want 210/120", monday.TrackedMinutes, monday.FocusMinutes)
	}
	if len(monday.Segments) != 3 {
		t.Fatalf("monday segments = %+v, want 3 (System absent)", monday.Segments)
	}
	if len(monday.Categories) != 3 || monday.Categories[0].Name != "Coding" {
		t.Fatalf("monday categories = %+v, want Coding first (minutes DESC)", monday.Categories)
	}
	if monday.Categories[0].ColorHex != "#111111" {
		t.Fatalf("coding color = %q, want the seeded color", monday.Categories[0].ColorHex)
	}

	tuesday := detail.Days[1]
	if tuesday.TrackedMinutes != 90 || tuesday.FocusMinutes != 90 {
		t.Fatalf("tuesday tracked/focus = %v/%v, want 90/90", tuesday.TrackedMinutes, tuesday.FocusMinutes)
	}
	if tuesday.Segments[0].StartTs >= tuesday.Segments[0].EndTs {
		t.Fatal("segment timestamps inverted")
	}
}

func TestBuildWeeklyDetailInsights(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	detail := BuildWeeklyDetail(detailFixture(loc), "2026-09-07", loc)
	ins := detail.Insights

	if ins.ActiveDays != 2 {
		t.Fatalf("activeDays = %d, want 2", ins.ActiveDays)
	}
	if ins.LongestFocusMinutes != 120 || ins.LongestFocusDay != "2026-09-07" {
		t.Fatalf("longest focus = %v on %q, want 120 on Monday", ins.LongestFocusMinutes, ins.LongestFocusDay)
	}
	// Tuesday's 10:30–12:00 Writing splits across 10, 11 (60m peak) and 12 is
	// outside; Monday contributes 9 (60m) and 10 (60m). Hour 10 has 60+30=90.
	if ins.PeakHour != 10 || ins.PeakHourMinutes != 90 {
		t.Fatalf("peak hour = %d with %v min, want 10 with 90", ins.PeakHour, ins.PeakHourMinutes)
	}
	if ins.MostActiveDay != "2026-09-07" || ins.MostActiveDayMinutes != 210 {
		t.Fatalf("most active = %q with %v, want Monday 180", ins.MostActiveDay, ins.MostActiveDayMinutes)
	}
	if ins.AvgDailyFocusMinutes != 105 {
		t.Fatalf("avg daily focus = %v, want (120+90)/2", ins.AvgDailyFocusMinutes)
	}
}

func TestBuildWeeklyDetailEmptyWeek(t *testing.T) {
	loc := time.UTC
	detail := BuildWeeklyDetail(nil, "2026-09-07", loc)
	if len(detail.Days) != 7 {
		t.Fatalf("days = %d, want 7 even when empty", len(detail.Days))
	}
	for _, day := range detail.Days {
		if day.TrackedMinutes != 0 || day.FocusMinutes != 0 || len(day.Segments) != 0 {
			t.Fatalf("empty day = %+v, want zeroed", day)
		}
	}
	if detail.Insights.PeakHour != -1 || detail.Insights.ActiveDays != 0 {
		t.Fatalf("insights = %+v, want no-data sentinels", detail.Insights)
	}
}

func TestBuildWeeklyDetailRejectsBadWeekStart(t *testing.T) {
	detail := BuildWeeklyDetail(nil, "not-a-monday", time.UTC)
	if len(detail.Days) != 0 || detail.Insights.PeakHour != -1 {
		t.Fatalf("detail = %+v, want empty with no-data sentinels", detail)
	}
}

func TestAddSpanHoursHalfHourZone(t *testing.T) {
	// A half-hour offset zone: time.Date normalizes to local wall-clock hours,
	// so a 10:15–11:00 span belongs entirely to local hour 10 (45 minutes).
	loc := time.FixedZone("IST", 5*3600+1800)
	var hours [24]float64
	start := time.Date(2026, 9, 7, 10, 15, 0, 0, loc).Unix()
	end := time.Date(2026, 9, 7, 11, 0, 0, 0, loc).Unix()
	addSpanHours(start, end, loc, &hours)
	if hours[10] != 45 || hours[11] != 0 {
		t.Fatalf("hours[10]=%v hours[11]=%v, want 45/0", hours[10], hours[11])
	}
}
