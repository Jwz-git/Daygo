package timeutil

import "time"

// FormatClock renders an instant as the LLM clock-string format "h:mm AM/PM"
// (docs/03 §3.5) — the same shape ResolveClock accepts. The analysis pipeline
// uses it for locally generated cards (Idle, skipped) whose start/end must
// round-trip through the same clock-string derivation as LLM output.
func FormatClock(t time.Time, loc *time.Location) string {
	if loc == nil {
		loc = t.Location()
	}
	return t.In(loc).Format("3:04 PM")
}
