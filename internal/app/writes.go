package app

import (
	"context"
	"strings"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// The shared write paths. Bindings and the chat tool executor both call these,
// never storage directly: same-origin means one implementation of every
// validation, guard, and event emission (docs/05 §5.12 "写入路径"). Each
// function performs the same steps the binding performed inline before the
// extraction — this file is a move, not a redesign.

// updateCardCategory moves one card to an existing category name. Unknown
// names are rejected — categories are never created implicitly (docs/05 §5.5.1).
func (b *Backend) updateCardCategory(ctx context.Context, cardID int64, category string) error {
	store := b.store()
	if _, found, err := store.Categories().ByName(ctx, strings.TrimSpace(category)); err != nil {
		return mapStorageError("update card category", err)
	} else if !found {
		return apperr.E(apperr.InvalidArgument, "unknown category: "+category, nil)
	}
	if err := store.Cards().UpdateCardCategory(ctx, cardID, category); err != nil {
		return mapStorageError("update card category", err)
	}
	b.invalidateCardDay(cardID)
	return nil
}

// updateCardTitle renames one card.
func (b *Backend) updateCardTitle(ctx context.Context, cardID int64, title string) error {
	if strings.TrimSpace(title) == "" {
		return apperr.E(apperr.InvalidArgument, "title is required", nil)
	}
	store := b.store()
	if err := store.Cards().UpdateCardTitle(ctx, cardID, title); err != nil {
		return mapStorageError("update card title", err)
	}
	b.invalidateCardDay(cardID)
	return nil
}

// deleteCard soft-deletes one card. The returned timelapse path (cleanup is a
// data-maintenance concern) is intentionally dropped here.
func (b *Backend) deleteCard(ctx context.Context, cardID int64) error {
	store := b.store()
	day, err := b.cardDay(ctx, cardID)
	if err != nil {
		return err
	}
	if _, err := store.Cards().SoftDeleteCard(ctx, cardID); err != nil {
		return mapStorageError("delete card", err)
	}
	b.emitTimelineInvalidation(day)
	return nil
}

// saveJournalDay upserts the user-editable fields. Summary is ignored: it is
// AI-generated and user-read-only, so a client cannot clobber it.
func (b *Backend) saveJournalDay(ctx context.Context, entry JournalDayDTO) error {
	day := strings.TrimSpace(entry.Day)
	if _, _, err := timeutil.DayWindow(day, b.clock.Now().Location()); err != nil {
		return apperr.E(apperr.InvalidArgument, "day must use yyyy-MM-dd", err)
	}
	status := strings.TrimSpace(entry.Status)
	if status == "" {
		status = storage.JournalStatusDraft
	}
	if !validJournalStatus(status) {
		return apperr.E(apperr.InvalidArgument, "unknown journal status: "+status, nil)
	}

	store := b.store()
	if err := store.Journal().Upsert(ctx, storage.JournalEntry{
		Day:         day,
		Intentions:  trimToNil(entry.Intentions),
		Notes:       trimToNil(entry.Notes),
		Goals:       trimToNil(entry.Goals),
		Reflections: trimToNil(entry.Reflections),
		Status:      status,
	}); err != nil {
		return mapStorageError("save journal day", err)
	}
	b.emitter.Emit(EventJournalUpdated, JournalUpdatedPayload{Day: day})
	return nil
}

// saveDayGoal upserts one day's goal. Category ids are validated against the
// categories table first so an unknown id is a clear invalid_argument rather
// than an FK backstop error.
func (b *Backend) saveDayGoal(ctx context.Context, goal DayGoalDTO) error {
	day := strings.TrimSpace(goal.Day)
	if _, _, err := timeutil.DayWindow(day, b.clock.Now().Location()); err != nil {
		return apperr.E(apperr.InvalidArgument, "day must use yyyy-MM-dd", err)
	}
	if goal.FocusTargetMinutes < 0 || goal.DistractionLimitMinutes < 0 {
		return apperr.E(apperr.InvalidArgument, "goal minutes must not be negative", nil)
	}

	store := b.store()
	categories, err := store.Categories().List(ctx)
	if err != nil {
		return mapStorageError("save day goal", err)
	}
	byID := make(map[string]struct{}, len(categories))
	for _, c := range categories {
		byID[c.ID] = struct{}{}
	}
	refs := make([]storage.GoalCategoryRef, 0, len(goal.FocusCategories)+len(goal.DistractionCategories))
	for _, dtoRef := range goal.FocusCategories {
		if _, ok := byID[dtoRef.CategoryID]; !ok {
			return apperr.E(apperr.InvalidArgument, "unknown category id: "+dtoRef.CategoryID, nil)
		}
		refs = append(refs, storage.GoalCategoryRef{
			CategoryID: dtoRef.CategoryID, Role: storage.GoalRoleFocus,
		})
	}
	for _, dtoRef := range goal.DistractionCategories {
		if _, ok := byID[dtoRef.CategoryID]; !ok {
			return apperr.E(apperr.InvalidArgument, "unknown category id: "+dtoRef.CategoryID, nil)
		}
		refs = append(refs, storage.GoalCategoryRef{
			CategoryID: dtoRef.CategoryID, Role: storage.GoalRoleDistraction,
		})
	}

	if err := store.Goals().Save(ctx, storage.DayGoal{
		Day:                     day,
		FocusTargetMinutes:      goal.FocusTargetMinutes,
		DistractionLimitMinutes: goal.DistractionLimitMinutes,
		IsSkipped:               goal.IsSkipped,
	}, refs); err != nil {
		return mapStorageError("save day goal", err)
	}
	b.emitter.Emit(EventGoalUpdated, GoalUpdatedPayload{Day: day})
	return nil
}

// saveCategories replaces the whole user category set. Callers derive the new
// set from the current one (add one row, change one row, drop one row) and are
// responsible for the semantic checks — name collisions, built-in protection —
// on top of the same CategoryRepo.Save transaction that rewrites cards on a
// rename. It is shared rather than binding-exposed: the SaveCategories
// binding waits for a category-management UI slice (docs/05 §5.2.1).
func (b *Backend) saveCategories(ctx context.Context, cats []domain.Category) error {
	store := b.store()
	if err := store.Categories().Save(ctx, cats); err != nil {
		return mapStorageError("save categories", err)
	}
	return nil
}
