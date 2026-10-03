package app

import (
	"context"
	"encoding/json"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// Plan tools for in-app chat and the agent socket (docs/05 §5.9.2 / §5.12).
// Arguments arrive schema-validated; the shared write paths in plan.go do the
// semantic checks, so a tool and a binding can never disagree.

func (e chatToolExecutor) planResult(ctx context.Context, day string) (json.RawMessage, error) {
	dto, err := e.backend.planDay(ctx, e.backend.store(), day)
	if err != nil {
		return nil, err
	}
	return json.Marshal(dto)
}

func (e chatToolExecutor) planAdd(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	if err := e.backend.requireTimelineWrite(); err != nil {
		return nil, err
	}
	remind, set := optionalBool(args, "remind")
	if !set {
		remind = true
	}
	input := PlanBlockInputDTO{
		Day:        stringField(args, "day"),
		Start:      stringField(args, "start"),
		End:        stringField(args, "end"),
		Title:      stringField(args, "title"),
		CategoryID: stringField(args, "categoryId"),
		Remind:     remind,
	}
	if notes, ok := optionalString(args, "notes"); ok {
		input.Notes = &notes
	}
	id, err := e.backend.savePlanBlock(ctx, input)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"ok": true, "blockId": id})
}

// planUpdate merges the given fields onto the stored block and saves the
// whole block through the same path as the binding.
func (e chatToolExecutor) planUpdate(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	if err := e.backend.requireTimelineWrite(); err != nil {
		return nil, err
	}
	id := intField(args, "blockId")
	block, err := e.backend.store().Plans().Get(ctx, id)
	if err != nil {
		return nil, mapStorageError("update plan block", err)
	}
	input := planInputOf(block, e.backend)
	changed := false
	for field, target := range map[string]*string{
		"day": &input.Day, "start": &input.Start, "end": &input.End, "title": &input.Title, "categoryId": &input.CategoryID,
	} {
		if value, ok := optionalString(args, field); ok {
			*target = value
			changed = true
		}
	}
	if notes, ok := optionalString(args, "notes"); ok {
		input.Notes = &notes
		changed = true
	}
	if remind, ok := optionalBool(args, "remind"); ok {
		input.Remind = remind
		changed = true
	}
	if !changed {
		return nil, toolError(apperr.InvalidArgument, "plan_update needs at least one field to change.")
	}
	if _, err := e.backend.savePlanBlock(ctx, input); err != nil {
		return nil, err
	}
	return toolOKEnvelope()
}

func (e chatToolExecutor) planComplete(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	if err := e.backend.requireTimelineWrite(); err != nil {
		return nil, err
	}
	status, ok := optionalString(args, "status")
	if !ok {
		status = storage.PlanStatusDone
	}
	if err := e.backend.setPlanBlockStatus(ctx, intField(args, "blockId"), status); err != nil {
		return nil, err
	}
	return toolOKEnvelope()
}

func (e chatToolExecutor) planDelete(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	if err := e.backend.requireTimelineWrite(); err != nil {
		return nil, err
	}
	if err := e.backend.deletePlanBlock(ctx, intField(args, "blockId")); err != nil {
		return nil, err
	}
	return toolOKEnvelope()
}

// planInputOf turns a stored block back into the editable input form, so a
// partial update can be merged and re-validated as a whole.
func planInputOf(block storage.PlanBlock, b *Backend) PlanBlockInputDTO {
	loc := b.clock.Now().Location()
	_, dayEnd, _ := timeutil.DayWindow(block.Day, loc)
	return PlanBlockInputDTO{
		ID:         block.ID,
		Day:        block.Day,
		Start:      planClock(block.StartTs, dayEnd, loc),
		End:        planClock(block.EndTs, dayEnd, loc),
		Title:      block.Title,
		Notes:      block.Notes,
		CategoryID: block.CategoryID,
		Remind:     block.Remind,
	}
}
