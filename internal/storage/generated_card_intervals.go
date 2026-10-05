package storage

import (
	"encoding/json"
	"time"

	"github.com/Jwz-git/Daygo/internal/timeutil"
)

type cardInterval struct{ start, end time.Time }

func subtractCardIntervals(span cardInterval, protected []cardInterval) []cardInterval {
	pieces := []cardInterval{span}
	for _, keep := range protected {
		var next []cardInterval
		for _, piece := range pieces {
			if !keep.end.After(piece.start) || !keep.start.Before(piece.end) {
				next = append(next, piece)
				continue
			}
			if keep.start.After(piece.start) {
				next = append(next, cardInterval{piece.start, keep.start})
			}
			if keep.end.Before(piece.end) {
				next = append(next, cardInterval{keep.end, piece.end})
			}
		}
		pieces = next
	}
	return pieces
}

// Keep the opaque metadata shape while limiting its two time-bearing arrays to
// a generated fragment. Invalid metadata cannot be clipped safely: roll back.
func clipGeneratedMetadata(raw string, span cardInterval, anchor time.Time, loc *time.Location) (string, error) {
	if raw == "" {
		return "", nil
	}
	var meta map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return "", wrap("clip generated card metadata", err)
	}
	for _, key := range []string{"activityPoints", "distractions"} {
		data, exists := meta[key]
		if !exists {
			continue
		}
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			return "", wrap("clip generated card metadata array", err)
		}
		kept := make([]map[string]json.RawMessage, 0, len(items))
		for _, item := range items {
			clock := func(key string) (time.Time, error) {
				var value string
				if err := json.Unmarshal(item[key], &value); err != nil {
					return time.Time{}, err
				}
				return timeutil.ResolveClock(value, anchor, loc)
			}
			if key == "activityPoints" {
				at, err := clock("time")
				if err != nil {
					return "", newError(KindConstraint, "clip generated activity time: invalid clock")
				}
				if !at.Before(span.start) && at.Before(span.end) {
					kept = append(kept, item)
				}
				continue
			}
			start, err := clock("startTime")
			if err != nil {
				return "", newError(KindConstraint, "clip generated distraction start: invalid clock")
			}
			end, err := clock("endTime")
			if err != nil {
				return "", newError(KindConstraint, "clip generated distraction end: invalid clock")
			}
			if start.Before(span.start) {
				start = span.start
			}
			if end.After(span.end) {
				end = span.end
			}
			if !end.After(start) {
				continue
			}
			item["startTime"], _ = json.Marshal(timeutil.FormatClock(start, loc))
			item["endTime"], _ = json.Marshal(timeutil.FormatClock(end, loc))
			kept = append(kept, item)
		}
		meta[key], _ = json.Marshal(kept)
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return "", wrap("encode clipped generated metadata", err)
	}
	return string(data), nil
}
