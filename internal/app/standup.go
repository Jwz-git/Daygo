package app

import (
	"context"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/storage"
	daytime "github.com/Jwz-git/Daygo/internal/timeutil"
)

// DailyRecapDTO mirrors docs/05 §5.5.2. The standup day is the calendar day,
// not the logical day (4am boundary), per §5.3.2.
type DailyRecapDTO struct {
	StandupDay      string   `json:"standupDay"`
	HighlightsTitle string   `json:"highlightsTitle"`
	Highlights      []string `json:"highlights"`
	TasksTitle      string   `json:"tasksTitle"`
	Tasks           []string `json:"tasks"`
	BlockersTitle   string   `json:"blockersTitle"`
	BlockersBody    string   `json:"blockersBody"`
	GeneratedAtTs  *int64   `json:"generatedAtTs"`
}

// GetDailyRecap returns the AI-generated standup for one calendar day, or
// an empty recap when none has been generated yet.
func (b *Backend) GetDailyRecap(standupDay string) (DailyRecapDTO, error) {
	// Validate standup day format (calendar day yyyy-MM-dd).
	if _, _, err := daytime.DayWindow(standupDay, b.clock.Now().Location()); err != nil {
		return DailyRecapDTO{}, apperr.E(apperr.InvalidArgument, "standupDay must use yyyy-MM-dd", err)
	}

	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return DailyRecapDTO{}, mapStorageError("get daily recap", err)
		}
		return DailyRecapDTO{}, apperr.E(apperr.DatabaseError, "recap requires a database", nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()

	entry, found, err := store.Standup().Get(ctx, standupDay)
	if err != nil {
		return DailyRecapDTO{}, mapStorageError("get daily recap", err)
	}
	if !found {
		// Return empty recap for days without generated content.
		return DailyRecapDTO{
			StandupDay:      standupDay,
			HighlightsTitle: "Today",
			Highlights:      []string{},
			TasksTitle:      "Next",
			Tasks:           []string{},
			BlockersTitle:   "Blockers",
			BlockersBody:    "",
			GeneratedAtTs:   nil,
		}, nil
	}

	generatedAt := entry.GeneratedAt.Unix()
	return DailyRecapDTO{
		StandupDay:      entry.StandupDay,
		HighlightsTitle: entry.HighlightsTitle,
		Highlights:      entry.Highlights,
		TasksTitle:      entry.TasksTitle,
		Tasks:           entry.Tasks,
		BlockersTitle:   entry.BlockersTitle,
		BlockersBody:    entry.BlockersBody,
		GeneratedAtTs:   &generatedAt,
	}, nil
}

// SaveDailyRecap stores the AI-generated standup for one calendar day.
// This is typically called by the analysis pipeline after LLM generation.
func (b *Backend) SaveDailyRecap(recap DailyRecapDTO) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	if _, _, err := daytime.DayWindow(recap.StandupDay, b.clock.Now().Location()); err != nil {
		return apperr.E(apperr.InvalidArgument, "standupDay must use yyyy-MM-dd", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()

	generatedAt := time.Now()
	if recap.GeneratedAtTs != nil {
		generatedAt = time.Unix(*recap.GeneratedAtTs, 0)
	}

	entry := storage.DailyStandupEntry{
		StandupDay:      recap.StandupDay,
		HighlightsTitle: recap.HighlightsTitle,
		Highlights:      recap.Highlights,
		TasksTitle:      recap.TasksTitle,
		Tasks:           recap.Tasks,
		BlockersTitle:   recap.BlockersTitle,
		BlockersBody:    recap.BlockersBody,
		GeneratedAt:     generatedAt,
	}
	if err := b.storage.Standup().Upsert(ctx, entry); err != nil {
		return mapStorageError("save daily recap", err)
	}
	return nil
}
