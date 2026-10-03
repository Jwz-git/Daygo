package app

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/insight"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// Plan blocks (docs/modules/plan.md): the user's time-blocked intentions for a
// logical day. Times cross the wire as 24-hour "HH:mm" plus the logical day;
// Go alone resolves them to instants, so no client derives the 4 AM boundary.

const (
	planTitleMaxRunes = 200
	planNotesMaxRunes = 4000
)

// PlanBlockDTO is one block with its derived coverage up to now.
type PlanBlockDTO struct {
	ID                 int64   `json:"id"`
	Day                string  `json:"day"`
	Start              string  `json:"start"`
	End                string  `json:"end"`
	StartTs            int64   `json:"startTs"`
	EndTs              int64   `json:"endTs"`
	Title              string  `json:"title"`
	Notes              *string `json:"notes"`
	CategoryID         string  `json:"categoryId"`
	CategoryName       string  `json:"categoryName"`
	ColorHex           string  `json:"colorHex"`
	Status             string  `json:"status"` // planned | done | skipped
	CompletedAtTs      *int64  `json:"completedAtTs"`
	Remind             bool    `json:"remind"`
	MatchedMinutes     float64 `json:"matchedMinutes"`
	DistractionMinutes float64 `json:"distractionMinutes"`
}

// PlanDayDTO is one logical day's plan, ordered by start time.
type PlanDayDTO struct {
	Day    string         `json:"day"`
	Blocks []PlanBlockDTO `json:"blocks"`
}

// PlanBlockInputDTO adds (ID 0) or fully replaces (ID > 0) a block's editable
// fields. The completion mark is set only through SetPlanBlockStatus.
type PlanBlockInputDTO struct {
	ID         int64   `json:"id"`
	Day        string  `json:"day"`
	Start      string  `json:"start"`
	End        string  `json:"end"`
	Title      string  `json:"title"`
	Notes      *string `json:"notes"`
	CategoryID string  `json:"categoryId"`
	Remind     bool    `json:"remind"`
}

// PlanUpdatedPayload is invalidation-only.
type PlanUpdatedPayload struct {
	Day string `json:"day"`
}

// GetPlanDay returns one logical day's plan with each block's coverage.
func (b *Backend) GetPlanDay(day string) (PlanDayDTO, error) {
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return PlanDayDTO{}, mapStorageError("get plan day", err)
		}
		return PlanDayDTO{}, apperr.E(apperr.DatabaseError, "plans require a database", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	return b.planDay(ctx, store, day)
}

func (b *Backend) planDay(ctx context.Context, store *storage.Store, day string) (PlanDayDTO, error) {
	loc := b.clock.Now().Location()
	start, end, err := timeutil.DayWindow(day, loc)
	if err != nil {
		return PlanDayDTO{}, apperr.E(apperr.InvalidArgument, "day must use yyyy-MM-dd", err)
	}
	reviews, err := loadPlanReviews(ctx, store, day, start, end, b.clock.Now(), loc)
	if err != nil {
		return PlanDayDTO{}, mapStorageError("get plan day", err)
	}
	dto := PlanDayDTO{Day: day, Blocks: make([]PlanBlockDTO, 0, len(reviews))}
	for _, review := range reviews {
		dto.Blocks = append(dto.Blocks, planBlockDTO(review, end, loc))
	}
	return dto, nil
}

// loadPlanReviews reads a day's blocks, the card spans of that day and the day
// goal's distraction categories, then measures each block.
func loadPlanReviews(ctx context.Context, store *storage.Store, day string, start, end, now time.Time,
	loc *time.Location) ([]insight.PlanReview, error) {
	blocks, err := store.Plans().ForDay(ctx, day)
	if err != nil || len(blocks) == 0 {
		return []insight.PlanReview{}, err
	}
	spans, err := store.Cards().CardSpansInRange(ctx, start, end)
	if err != nil {
		return nil, err
	}
	distraction, err := goalDistractionNames(ctx, store, day)
	if err != nil {
		return nil, err
	}
	return insight.PlanReviews(blocks, spans, distraction, now.Unix(), loc), nil
}

func goalDistractionNames(ctx context.Context, store *storage.Store, day string) ([]string, error) {
	_, refs, found, err := store.Goals().Get(ctx, day)
	if err != nil || !found {
		return nil, err
	}
	var names []string
	for _, ref := range refs {
		if ref.Role == storage.GoalRoleDistraction {
			names = append(names, ref.Name)
		}
	}
	return names, nil
}

func planBlockDTO(review insight.PlanReview, dayEnd time.Time, loc *time.Location) PlanBlockDTO {
	block := review.Block
	dto := PlanBlockDTO{
		ID:                 block.ID,
		Day:                block.Day,
		Start:              planClock(block.StartTs, dayEnd, loc),
		End:                planClock(block.EndTs, dayEnd, loc),
		StartTs:            block.StartTs,
		EndTs:              block.EndTs,
		Title:              block.Title,
		Notes:              block.Notes,
		CategoryID:         block.CategoryID,
		CategoryName:       block.CategoryName,
		ColorHex:           block.CategoryColor,
		Status:             block.Status,
		Remind:             block.Remind,
		MatchedMinutes:     review.Coverage.MatchedMinutes,
		DistractionMinutes: review.Coverage.DistractionMinutes,
	}
	if block.CompletedAt != nil {
		at := block.CompletedAt.Unix()
		dto.CompletedAtTs = &at
	}
	return dto
}

// planClock renders an instant as the "HH:mm" the inputs accept; the day's own
// end renders as "04:00", which ResolveDayRange reads back as the end.
func planClock(ts int64, dayEnd time.Time, loc *time.Location) string {
	if ts == dayEnd.Unix() {
		return dayEnd.In(loc).Format("15:04")
	}
	return time.Unix(ts, 0).In(loc).Format("15:04")
}

// SavePlanBlock adds a block (ID 0) or replaces one's editable fields and
// returns its id.
func (b *Backend) SavePlanBlock(input PlanBlockInputDTO) (int64, error) {
	if err := b.requireTimelineWrite(); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	return b.savePlanBlock(ctx, input)
}

// SetPlanBlockStatus records a block's completion mark.
func (b *Backend) SetPlanBlockStatus(id int64, status string) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	return b.setPlanBlockStatus(ctx, id, status)
}

// DeletePlanBlock removes a block.
func (b *Backend) DeletePlanBlock(id int64) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	return b.deletePlanBlock(ctx, id)
}

// The shared write paths below serve the bindings, in-app chat and the agent
// socket alike (docs/05 §5.9.2), so validation and events never diverge.

func (b *Backend) savePlanBlock(ctx context.Context, input PlanBlockInputDTO) (int64, error) {
	store := b.store()
	block, err := b.validatePlanInput(ctx, store, input)
	if err != nil {
		return 0, err
	}
	previousDay := ""
	if input.ID > 0 {
		existing, err := store.Plans().Get(ctx, input.ID)
		if err != nil {
			return 0, mapStorageError("save plan block", err)
		}
		previousDay = existing.Day
		block.ID = input.ID
		if err := store.Plans().Update(ctx, block); err != nil {
			return 0, mapStorageError("save plan block", err)
		}
	} else {
		id, err := store.Plans().Insert(ctx, block)
		if err != nil {
			return 0, mapStorageError("save plan block", err)
		}
		block.ID = id
	}
	b.planChanged(block.Day)
	if previousDay != "" && previousDay != block.Day {
		b.planChanged(previousDay)
	}
	return block.ID, nil
}

func (b *Backend) validatePlanInput(ctx context.Context, store *storage.Store, input PlanBlockInputDTO) (storage.PlanBlock, error) {
	loc := b.clock.Now().Location()
	day := strings.TrimSpace(input.Day)
	start, end, err := timeutil.ResolveDayRange(day, strings.TrimSpace(input.Start), strings.TrimSpace(input.End), loc)
	if err != nil {
		return storage.PlanBlock{}, apperr.E(apperr.InvalidArgument, "plan times must be HH:mm inside the day, end after start", err)
	}
	title := strings.TrimSpace(input.Title)
	if title == "" || utf8.RuneCountInString(title) > planTitleMaxRunes {
		return storage.PlanBlock{}, apperr.E(apperr.InvalidArgument, "plan title must be 1 to 200 characters", nil)
	}
	notes := trimToNil(input.Notes)
	if notes != nil && utf8.RuneCountInString(*notes) > planNotesMaxRunes {
		return storage.PlanBlock{}, apperr.E(apperr.InvalidArgument, "plan notes must be at most 4000 characters", nil)
	}
	categoryID := strings.TrimSpace(input.CategoryID)
	if categoryID != "" {
		categories, err := store.Categories().List(ctx)
		if err != nil {
			return storage.PlanBlock{}, mapStorageError("save plan block", err)
		}
		known := false
		for _, category := range categories {
			if category.ID == categoryID {
				known = true
				break
			}
		}
		if !known {
			return storage.PlanBlock{}, apperr.E(apperr.InvalidArgument, "unknown category id: "+categoryID, nil)
		}
	}
	return storage.PlanBlock{
		Day:        day,
		StartTs:    start.Unix(),
		EndTs:      end.Unix(),
		Title:      title,
		Notes:      notes,
		CategoryID: categoryID,
		Remind:     input.Remind,
	}, nil
}

func (b *Backend) setPlanBlockStatus(ctx context.Context, id int64, status string) error {
	status = strings.TrimSpace(status)
	if !storage.ValidPlanStatus(status) {
		return apperr.E(apperr.InvalidArgument, "plan status must be planned, done or skipped", nil)
	}
	store := b.store()
	block, err := store.Plans().Get(ctx, id)
	if err != nil {
		return mapStorageError("set plan status", err)
	}
	if err := store.Plans().SetStatus(ctx, id, status); err != nil {
		return mapStorageError("set plan status", err)
	}
	b.planChanged(block.Day)
	return nil
}

func (b *Backend) deletePlanBlock(ctx context.Context, id int64) error {
	store := b.store()
	block, err := store.Plans().Get(ctx, id)
	if err != nil {
		return mapStorageError("delete plan block", err)
	}
	if err := store.Plans().Delete(ctx, id); err != nil {
		return mapStorageError("delete plan block", err)
	}
	b.planChanged(block.Day)
	return nil
}

// planChanged announces a day's plan change and nudges the plan reminder so a
// new or moved block's start notification is scheduled without waiting a tick.
func (b *Backend) planChanged(day string) {
	b.emitter.Emit(EventPlanUpdated, PlanUpdatedPayload{Day: day})
	b.nudgePlanReminder()
}
