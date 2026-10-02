package app

import (
	"context"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

type TokenUsageBucketDTO struct {
	StartTs      int64 `json:"startTs"`
	EndTs        int64 `json:"endTs"`
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	Calls        int64 `json:"calls"`
	UnknownCalls int64 `json:"unknownCalls"`
}

type TokenUsageDTO struct {
	Period   string                `json:"period"`
	TimeZone string                `json:"timeZone"`
	Buckets  []TokenUsageBucketDTO `json:"buckets"`
}

// GetTokenUsage returns all recorded attempts by invocation start time.
// day uses logical-day hourly buckets; week uses seven logical-day buckets.
func (b *Backend) GetTokenUsage(period, day string) (TokenUsageDTO, error) {
	s := b.store()
	if s == nil {
		return TokenUsageDTO{}, apperr.E(apperr.DatabaseError, "token usage requires a database", nil)
	}
	loc := s.Location()
	var start, end time.Time
	var err error
	switch period {
	case "day":
		start, end, err = timeutil.DayWindow(day, loc)
	case "week":
		start, end, err = timeutil.WeekWindow(day, loc)
	default:
		return TokenUsageDTO{}, apperr.E(apperr.InvalidArgument, "period must be day or week", nil)
	}
	if err != nil {
		return TokenUsageDTO{}, apperr.E(apperr.InvalidArgument, "invalid token usage date", err)
	}
	dto := TokenUsageDTO{Period: period, TimeZone: loc.String(), Buckets: make([]TokenUsageBucketDTO, 0)}
	for at := start; at.Before(end); {
		next := at.Add(time.Hour)
		if period == "week" {
			_, next, err = timeutil.DayWindow(timeutil.LogicalDay(at, loc), loc)
			if err != nil {
				return TokenUsageDTO{}, err
			}
		}
		if next.After(end) {
			next = end
		}
		dto.Buckets = append(dto.Buckets, TokenUsageBucketDTO{StartTs: at.Unix(), EndTs: next.Unix()})
		at = next
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	samples, err := s.LlmCalls().TokenUsage(ctx, start, end)
	if err != nil {
		return TokenUsageDTO{}, mapStorageError("get token usage", err)
	}
	index := 0
	for _, sample := range samples {
		for index < len(dto.Buckets)-1 && sample.StartedAt >= dto.Buckets[index].EndTs {
			index++
		}
		bucket := &dto.Buckets[index]
		bucket.InputTokens += sample.InputTokens
		bucket.OutputTokens += sample.OutputTokens
		bucket.Calls += sample.Calls
		bucket.UnknownCalls += sample.UnknownCalls
	}
	return dto, nil
}
