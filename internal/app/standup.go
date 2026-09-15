package app

import (
	"context"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/insight"
	"github.com/Jwz-git/Daygo/internal/storage"
	daytime "github.com/Jwz-git/Daygo/internal/timeutil"
)

// RecapUpdatedPayload is an invalidation-only payload (docs/05 §5.5.3): the
// standupDay identifies which recap to re-pull; no recap content rides the
// event.
type RecapUpdatedPayload struct {
	StandupDay string `json:"standupDay"`
}

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
	GeneratedAtTs   *int64   `json:"generatedAtTs"`
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
	b.emitter.Emit(EventRecapUpdated, RecapUpdatedPayload{StandupDay: recap.StandupDay})
	return nil
}

// standupTimeout bounds one synchronous recap generation: a single LLM call
// plus the card query and the upsert. Two minutes covers slow providers
// without leaving the UI waiting indefinitely.
const standupTimeout = 2 * time.Minute

// GenerateDailyRecap regenerates the standup for one calendar day from the
// day's activity cards and stores the result. The write guard matches the
// other recap writes: a read-only second instance must not overwrite the
// owner's content.
func (b *Backend) GenerateDailyRecap(standupDay string) (DailyRecapDTO, error) {
	if err := b.requireTimelineWrite(); err != nil {
		return DailyRecapDTO{}, err
	}
	loc := b.clock.Now().Location()
	if _, _, err := daytime.DayWindow(standupDay, loc); err != nil {
		return DailyRecapDTO{}, apperr.E(apperr.InvalidArgument, "standupDay must use yyyy-MM-dd", err)
	}

	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return DailyRecapDTO{}, mapStorageError("generate daily recap", err)
		}
		return DailyRecapDTO{}, apperr.E(apperr.DatabaseError, "recap requires a database", nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), standupTimeout)
	defer cancel()

	// The standup covers the calendar day [00:00, next 00:00) local, per
	// docs/05 §5.3.2 — not the 4 AM logical-day window the cards are filed
	// under. CardsInRange's overlap rule pulls in cross-midnight activities.
	dayStart, err := daytime.ParseDay(standupDay, loc)
	if err != nil {
		return DailyRecapDTO{}, apperr.E(apperr.InvalidArgument, "standupDay must use yyyy-MM-dd", err)
	}
	cards, err := store.Cards().CardsInRange(ctx, dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		return DailyRecapDTO{}, mapStorageError("generate daily recap", err)
	}

	// System and Idle cards are machine semantics, not user activity; the
	// prompt must not ask the model to summarise them.
	categories, err := store.Categories().List(ctx)
	if err != nil {
		return DailyRecapDTO{}, mapStorageError("generate daily recap", err)
	}
	excluded := make(map[string]bool, len(categories))
	for _, c := range categories {
		if c.IsSystem || c.IsIdle {
			excluded[c.Name] = true
		}
	}
	activity := make([]domain.TimelineCard, 0, len(cards))
	for _, card := range cards {
		if !excluded[card.Category] {
			activity = append(activity, card)
		}
	}

	chain, err := analysisChainSource{backend: b}.AnalysisChain(ctx)
	if err != nil {
		return DailyRecapDTO{}, mapStorageError("generate daily recap", err)
	}
	request := ai.Request{
		Purpose:         ai.PurposeStandup,
		Parts:           []ai.Part{ai.TextPart(insight.StandupPrompt(standupDay, activity, analysisLanguage(b)(ctx)))},
		Output:          &insight.StandupOutput,
		MaxOutputTokens: 2048,
	}
	result, err := chain.Generate(ctx, request)
	if err != nil {
		return DailyRecapDTO{}, mapRecapGenerationError(err)
	}
	envelope, err := insight.ParseStandupOutput(result.Text)
	if err != nil {
		return DailyRecapDTO{}, mapRecapGenerationError(err)
	}

	now := b.clock.Now()
	generatedAt := now.Unix()
	entry := storage.DailyStandupEntry{
		StandupDay:      standupDay,
		HighlightsTitle: envelope.HighlightsTitle,
		Highlights:      envelope.Highlights,
		TasksTitle:      envelope.TasksTitle,
		Tasks:           envelope.Tasks,
		BlockersTitle:   envelope.BlockersTitle,
		BlockersBody:    envelope.BlockersBody,
		GeneratedAt:     now,
	}
	if err := store.Standup().Upsert(ctx, entry); err != nil {
		return DailyRecapDTO{}, mapStorageError("generate daily recap", err)
	}
	b.emitter.Emit(EventRecapUpdated, RecapUpdatedPayload{StandupDay: standupDay})
	return DailyRecapDTO{
		StandupDay:      standupDay,
		HighlightsTitle: envelope.HighlightsTitle,
		Highlights:      envelope.Highlights,
		TasksTitle:      envelope.TasksTitle,
		Tasks:           envelope.Tasks,
		BlockersTitle:   envelope.BlockersTitle,
		BlockersBody:    envelope.BlockersBody,
		GeneratedAtTs:   &generatedAt,
	}, nil
}

// mapRecapGenerationError turns an ai-layer failure into a binding error. The
// ai layer's messages are fixed sanitized strings; provider bodies never
// reach them, so passing the message through is safe.
func mapRecapGenerationError(err error) error {
	if err == ai.ErrNoProvider {
		return apperr.E(apperr.ProviderNotConfigured, "generate daily recap: no provider is configured", err)
	}
	return apperr.E(apperr.ProviderFailed, "generate daily recap: "+err.Error(), err)
}
