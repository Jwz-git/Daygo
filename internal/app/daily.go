package app

import (
	"context"
	"strings"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// JournalDayDTO and DayGoalDTO mirror docs/05 §5.5.2. The nilable journal
// fields mean "empty", not "unknown": a day without an entry returns a zero
// DTO with UpdatedAtTs nil.
type JournalDayDTO struct {
	Day         string  `json:"day"`
	Intentions  *string `json:"intentions"`
	Notes       *string `json:"notes"`
	Goals       *string `json:"goals"`
	Reflections *string `json:"reflections"`
	Summary     *string `json:"summary"` // AI generated, read-only for users
	Status      string  `json:"status"`  // draft | intentions_set | complete
	UpdatedAtTs *int64  `json:"updatedAtTs"`
}

type DayGoalDTO struct {
	Day                     string               `json:"day"`
	FocusTargetMinutes      int                  `json:"focusTargetMinutes"`
	DistractionLimitMinutes int                  `json:"distractionLimitMinutes"`
	IsSkipped               bool                 `json:"isSkipped"`
	FocusCategories         []GoalCategoryRefDTO `json:"focusCategories"`
	DistractionCategories   []GoalCategoryRefDTO `json:"distractionCategories"`
	Exists                  bool                 `json:"exists"`
}

type GoalCategoryRefDTO struct {
	CategoryID string `json:"categoryId"`
	Name       string `json:"name"`
	ColorHex   string `json:"colorHex"`
	SortOrder  int    `json:"sortOrder"`
}

// JournalUpdatedPayload / GoalUpdatedPayload are invalidation-only payloads.
type JournalUpdatedPayload struct {
	Day string `json:"day"`
}

type GoalUpdatedPayload struct {
	Day string `json:"day"`
}

func validJournalStatus(status string) bool {
	switch status {
	case storage.JournalStatusDraft, storage.JournalStatusIntentionsSet, storage.JournalStatusComplete:
		return true
	}
	return false
}

// requireTimelineWrite also guards journal and goal saves: they are write
// methods on the same database, so the read-only instance rule applies
// (docs/05 §5.6.2 rule 7).

// GetJournalDay returns one logical day's journal entry, or a zero DTO when
// none exists yet.
func (b *Backend) GetJournalDay(day string) (JournalDayDTO, error) {
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return JournalDayDTO{}, mapStorageError("get journal day", err)
		}
		return JournalDayDTO{}, apperr.E(apperr.DatabaseError, "journal requires a database", nil)
	}
	if _, _, err := timeutil.DayWindow(day, b.clock.Now().Location()); err != nil {
		return JournalDayDTO{}, apperr.E(apperr.InvalidArgument, "day must use yyyy-MM-dd", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()

	entry, found, err := store.Journal().Get(ctx, day)
	if err != nil {
		return JournalDayDTO{}, mapStorageError("get journal day", err)
	}
	if !found {
		return JournalDayDTO{Day: day}, nil
	}
	updatedAt := entry.UpdatedAt.Unix()
	return JournalDayDTO{
		Day:         entry.Day,
		Intentions:  entry.Intentions,
		Notes:       entry.Notes,
		Goals:       entry.Goals,
		Reflections: entry.Reflections,
		Summary:     entry.Summary,
		Status:      entry.Status,
		UpdatedAtTs: &updatedAt,
	}, nil
}

// SaveJournalDay upserts the user-editable fields. Summary is ignored: it is
// AI-generated and user-read-only, so a client cannot clobber it.
func (b *Backend) SaveJournalDay(entry JournalDayDTO) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	return b.saveJournalDay(ctx, entry)
}

// trimToNil normalizes an optional text field: blank strings become NULL.
func trimToNil(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// GetDayGoal returns one logical day's goal and its category references,
// or Exists=false when none is set.
func (b *Backend) GetDayGoal(day string) (DayGoalDTO, error) {
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return DayGoalDTO{}, mapStorageError("get day goal", err)
		}
		return DayGoalDTO{}, apperr.E(apperr.DatabaseError, "goals require a database", nil)
	}
	if _, _, err := timeutil.DayWindow(day, b.clock.Now().Location()); err != nil {
		return DayGoalDTO{}, apperr.E(apperr.InvalidArgument, "day must use yyyy-MM-dd", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()

	goal, refs, found, err := store.Goals().Get(ctx, day)
	if err != nil {
		return DayGoalDTO{}, mapStorageError("get day goal", err)
	}
	if !found {
		return DayGoalDTO{Day: day, Exists: false}, nil
	}
	dto := DayGoalDTO{
		Day:                     goal.Day,
		FocusTargetMinutes:      goal.FocusTargetMinutes,
		DistractionLimitMinutes: goal.DistractionLimitMinutes,
		IsSkipped:               goal.IsSkipped,
		FocusCategories:         []GoalCategoryRefDTO{},
		DistractionCategories:   []GoalCategoryRefDTO{},
		Exists:                  true,
	}
	for _, ref := range refs {
		dtoRef := GoalCategoryRefDTO{
			CategoryID: ref.CategoryID,
			Name:       ref.Name,
			ColorHex:   ref.ColorHex,
			SortOrder:  ref.SortOrder,
		}
		switch ref.Role {
		case storage.GoalRoleFocus:
			dto.FocusCategories = append(dto.FocusCategories, dtoRef)
		case storage.GoalRoleDistraction:
			dto.DistractionCategories = append(dto.DistractionCategories, dtoRef)
		}
	}
	return dto, nil
}

// SaveDayGoal upserts one day's goal.
func (b *Backend) SaveDayGoal(goal DayGoalDTO) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	return b.saveDayGoal(ctx, goal)
}
