package app

import (
	"context"
	"encoding/json"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/chat"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// chatToolExecutor is the chat agent's only effect outlet (docs/05 §5.12).
// Reads go through the same Get* assembly the bindings serve; writes go
// through the shared write paths in writes.go, so validation, the read-only
// instance guard, and event emission are identical for both entry points.
type chatToolExecutor struct {
	backend *Backend
}

// Execute runs one tool call and always returns a complete result envelope —
// including on panic, because the model's output is data and the tool face
// must be closed no matter what it asks for.
func (e chatToolExecutor) Execute(ctx context.Context, call chat.ToolCall) chat.ToolResult {
	data, err := e.dispatch(ctx, call)
	if err != nil {
		return chat.ToolResult{Data: toolErrorEnvelope(err), Err: true}
	}
	return chat.ToolResult{Data: data}
}

func (e chatToolExecutor) dispatch(ctx context.Context, call chat.ToolCall) (json.RawMessage, error) {
	backend := e.backend
	if backend == nil || backend.store() == nil {
		return nil, toolError(apperr.DatabaseError, "chat 工具需要数据库。")
	}
	switch call.Tool {
	case chat.ToolTimeline:
		dto, err := backend.GetTimelineDay(stringField(call.Arguments, "day"))
		if err != nil {
			return nil, err
		}
		return json.Marshal(dto)
	case chat.ToolCard:
		return e.cardResult(ctx, intField(call.Arguments, "cardId"))
	case chat.ToolDaily:
		return e.dailyResult(stringField(call.Arguments, "day"))
	case chat.ToolWeekly:
		dto, err := backend.GetWeeklyDashboard(stringField(call.Arguments, "weekStart"))
		if err != nil {
			return nil, err
		}
		return json.Marshal(dto)
	case chat.ToolCategories:
		return e.categoriesResult(ctx)
	case chat.ToolCategoryAdd:
		return e.categoryAdd(ctx, call.Arguments)
	case chat.ToolCategoryUpdate:
		return e.categoryUpdate(ctx, call.Arguments)
	case chat.ToolCategoryRemove:
		return e.categoryRemove(ctx, call.Arguments)
	case chat.ToolCardUpdate:
		return e.cardUpdate(ctx, call.Arguments)
	case chat.ToolCardDelete:
		if err := backend.requireTimelineWrite(); err != nil {
			return nil, err
		}
		if err := backend.deleteCard(ctx, intField(call.Arguments, "cardId")); err != nil {
			return nil, err
		}
		return toolOKEnvelope()
	case chat.ToolGoalSet:
		return e.goalSet(ctx, call.Arguments)
	default:
		return nil, toolError(apperr.InvalidArgument, "未知工具 "+call.Tool+"。")
	}
}

// --- Read tools ---

func (e chatToolExecutor) cardResult(ctx context.Context, cardID int64) (json.RawMessage, error) {
	store := e.backend.store()
	card, err := store.Cards().CardByID(ctx, cardID)
	if err != nil {
		return nil, mapStorageError("chat card", err)
	}
	categories, err := store.Categories().List(ctx)
	if err != nil {
		return nil, mapStorageError("chat card", err)
	}
	dto := sharedCardDTO(card, categoryFlagsFrom(categories))
	return json.Marshal(dto)
}

func (e chatToolExecutor) dailyResult(day string) (json.RawMessage, error) {
	if _, _, err := timeutil.DayWindow(day, e.backend.clock.Now().Location()); err != nil {
		return nil, toolError(apperr.InvalidArgument, "day 必须是 yyyy-MM-dd 格式的合法日期。")
	}
	journal, err := e.backend.GetJournalDay(day)
	if err != nil {
		return nil, err
	}
	goal, err := e.backend.GetDayGoal(day)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"journal": journal, "goal": goal})
}

func (e chatToolExecutor) categoriesResult(ctx context.Context) (json.RawMessage, error) {
	categories, err := e.backend.store().Categories().List(ctx)
	if err != nil {
		return nil, mapStorageError("chat categories", err)
	}
	out := make([]map[string]any, 0, len(categories))
	for _, c := range categories {
		out = append(out, map[string]any{
			"id": c.ID, "name": c.Name, "colorHex": c.ColorHex, "details": c.Details,
			"sortOrder": c.SortOrder, "isSystem": c.IsSystem, "isIdle": c.IsIdle,
		})
	}
	return json.Marshal(out)
}

// --- Write tools (all gated by requireTimelineWrite, same as bindings) ---

func (e chatToolExecutor) cardUpdate(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	if err := e.backend.requireTimelineWrite(); err != nil {
		return nil, err
	}
	cardID := intField(args, "cardId")
	category, hasCategory := optionalString(args, "category")
	title, hasTitle := optionalString(args, "title")
	if !hasCategory && !hasTitle {
		return nil, toolError(apperr.InvalidArgument, "card_update 需要 category 或 title 至少一项。")
	}
	if hasCategory {
		if err := e.backend.updateCardCategory(ctx, cardID, category); err != nil {
			return nil, err
		}
	}
	if hasTitle {
		if err := e.backend.updateCardTitle(ctx, cardID, title); err != nil {
			return nil, err
		}
	}
	return toolOKEnvelope()
}

func (e chatToolExecutor) goalSet(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	if err := e.backend.requireTimelineWrite(); err != nil {
		return nil, err
	}
	goal := DayGoalDTO{
		Day:                     stringField(args, "day"),
		FocusTargetMinutes:      int(intField(args, "focusTargetMinutes")),
		DistractionLimitMinutes: int(intField(args, "distractionLimitMinutes")),
		IsSkipped:               boolField(args, "isSkipped"),
	}
	if ids, ok := stringSlice(args, "focusCategoryIds"); ok {
		goal.FocusCategories = refDTOs(ids)
	}
	if ids, ok := stringSlice(args, "distractionCategoryIds"); ok {
		goal.DistractionCategories = refDTOs(ids)
	}
	// Unspecified minutes default to zero, matching a fresh goal row; the
	// shared save path validates day shape, id existence, and non-negatives.
	if err := e.backend.saveDayGoal(ctx, goal); err != nil {
		return nil, err
	}
	return toolOKEnvelope()
}

func refDTOs(ids []string) []GoalCategoryRefDTO {
	out := make([]GoalCategoryRefDTO, len(ids))
	for i, id := range ids {
		out[i] = GoalCategoryRefDTO{CategoryID: id}
	}
	return out
}

// categoryAdd / categoryUpdate / categoryRemove map their single-row intent
// onto the whole-set CategoryRepo.Save transaction, deriving the new set from
// the current one inside this call. The built-ins (is_system) are protected
// here; Save itself re-seeds them and rewrites cards on a rename.

func (e chatToolExecutor) categoryAdd(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	if err := e.backend.requireTimelineWrite(); err != nil {
		return nil, err
	}
	name := stringField(args, "name")
	categories, err := e.backend.store().Categories().List(ctx)
	if err != nil {
		return nil, mapStorageError("chat category add", err)
	}
	for _, c := range categories {
		if c.Name == name {
			return nil, toolError(apperr.InvalidArgument, "分类名已存在："+name)
		}
	}
	// Only the non-built-in rows go into the new set: Save merges the
	// built-ins back itself, and a hand-copied built-in would collide with
	// the merged row as a duplicate name.
	next := make([]domain.Category, 0, len(categories)+1)
	maxOrder := 0
	for _, c := range categories {
		if c.IsSystem {
			continue
		}
		next = append(next, domain.Category{
			ID: c.ID, Name: c.Name, ColorHex: c.ColorHex, Details: c.Details,
			SortOrder: c.SortOrder, IsIdle: c.IsIdle,
		})
		if c.SortOrder > maxOrder {
			maxOrder = c.SortOrder
		}
	}
	added := domain.Category{
		Name:     name,
		ColorHex: "#8E8E93",
		IsIdle:   boolField(args, "isIdle"),
	}
	if v, ok := optionalString(args, "colorHex"); ok {
		added.ColorHex = v
	}
	if v, ok := optionalString(args, "details"); ok {
		added.Details = v
	}
	if v, ok := optionalInt(args, "sortOrder"); ok {
		added.SortOrder = v
	} else {
		added.SortOrder = maxOrder + 1
	}
	next = append(next, added)
	if err := e.backend.saveCategories(ctx, next); err != nil {
		return nil, err
	}
	return toolOKEnvelope()
}

func (e chatToolExecutor) categoryUpdate(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	if err := e.backend.requireTimelineWrite(); err != nil {
		return nil, err
	}
	categoryID := stringField(args, "categoryId")
	categories, err := e.backend.store().Categories().List(ctx)
	if err != nil {
		return nil, mapStorageError("chat category update", err)
	}
	found := false
	for _, c := range categories {
		if c.ID != categoryID {
			continue
		}
		if c.IsSystem {
			return nil, toolError(apperr.InvalidArgument, "内置分类不可修改。")
		}
		found = true
	}
	if !found {
		return nil, toolError(apperr.NotFound, "分类不存在。")
	}
	next := make([]domain.Category, 0, len(categories))
	for _, c := range categories {
		if c.IsSystem {
			continue
		}
		if c.ID != categoryID {
			next = append(next, domain.Category{
				ID: c.ID, Name: c.Name, ColorHex: c.ColorHex, Details: c.Details,
				SortOrder: c.SortOrder, IsIdle: c.IsIdle,
			})
			continue
		}
		updated := domain.Category{
			ID:        c.ID,
			Name:      c.Name,
			ColorHex:  c.ColorHex,
			Details:   c.Details,
			SortOrder: c.SortOrder,
			IsIdle:    c.IsIdle,
		}
		if v, ok := optionalString(args, "name"); ok {
			for _, other := range categories {
				if other.ID != categoryID && other.Name == v {
					return nil, toolError(apperr.InvalidArgument, "分类名已存在："+v)
				}
			}
			updated.Name = v
		}
		if v, ok := optionalString(args, "colorHex"); ok {
			updated.ColorHex = v
		}
		if v, ok := optionalString(args, "details"); ok {
			updated.Details = v
		}
		if v, ok := optionalInt(args, "sortOrder"); ok {
			updated.SortOrder = v
		}
		if v, ok := optionalBool(args, "isIdle"); ok {
			updated.IsIdle = v
		}
		next = append(next, updated)
	}
	if err := e.backend.saveCategories(ctx, next); err != nil {
		return nil, err
	}
	return toolOKEnvelope()
}

func (e chatToolExecutor) categoryRemove(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	if err := e.backend.requireTimelineWrite(); err != nil {
		return nil, err
	}
	categoryID := stringField(args, "categoryId")
	categories, err := e.backend.store().Categories().List(ctx)
	if err != nil {
		return nil, mapStorageError("chat category remove", err)
	}
	found := false
	next := make([]domain.Category, 0, len(categories))
	for _, c := range categories {
		if c.IsSystem {
			continue
		}
		if c.ID != categoryID {
			next = append(next, domain.Category{
				ID: c.ID, Name: c.Name, ColorHex: c.ColorHex, Details: c.Details,
				SortOrder: c.SortOrder, IsIdle: c.IsIdle,
			})
			continue
		}
		if c.IsSystem {
			return nil, toolError(apperr.InvalidArgument, "内置分类不可删除。")
		}
		found = true
	}
	if !found {
		return nil, toolError(apperr.NotFound, "分类不存在。")
	}
	if err := e.backend.saveCategories(ctx, next); err != nil {
		return nil, err
	}
	return toolOKEnvelope()
}

// --- Envelope helpers ---

func toolOKEnvelope() (json.RawMessage, error) {
	return json.Marshal(map[string]any{"ok": true})
}

// toolError carries an apperr through to the model; the message is the same
// sanitized text a binding caller would see.
func toolError(code apperr.Code, message string) error {
	return apperr.E(code, message, nil)
}

// toolErrorEnvelope converts any error into the result envelope the model
// reads. apperr codes pass through; everything else is internal_error with a
// generic message so raw storage errors never reach the prompt.
func toolErrorEnvelope(err error) json.RawMessage {
	if err == nil {
		out, _ := json.Marshal(map[string]any{"ok": true})
		return out
	}
	var ae *apperr.Error
	if ok := asApperr(err, &ae); ok {
		out, _ := json.Marshal(map[string]any{
			"ok": false,
			"error": map[string]string{
				"code":    string(ae.Code),
				"message": ae.Message,
			},
		})
		return out
	}
	out, _ := json.Marshal(map[string]any{
		"ok": false,
		"error": map[string]string{
			"code":    "internal_error",
			"message": "工具执行失败。",
		},
	})
	return out
}

// asApperr unwraps an apperr.Error through any wrapping.
func asApperr(err error, target **apperr.Error) bool {
	if ae, ok := err.(*apperr.Error); ok {
		*target = ae
		return true
	}
	return false
}

// --- Argument field accessors over the schema-validated arguments ---

func stringField(args json.RawMessage, field string) string {
	v, _ := optionalString(args, field)
	return v
}

func optionalString(args json.RawMessage, field string) (string, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(args, &m); err != nil {
		return "", false
	}
	raw, ok := m[field]
	if !ok {
		return "", false
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", false
	}
	return v, true
}

func intField(args json.RawMessage, field string) int64 {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(args, &m); err != nil {
		return 0
	}
	raw, ok := m[field]
	if !ok {
		return 0
	}
	var v int64
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0
	}
	return v
}

func optionalInt(args json.RawMessage, field string) (int, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(args, &m); err != nil {
		return 0, false
	}
	raw, ok := m[field]
	if !ok {
		return 0, false
	}
	var v int
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, false
	}
	return v, true
}

func boolField(args json.RawMessage, field string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(args, &m); err != nil {
		return false
	}
	raw, ok := m[field]
	if !ok {
		return false
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return false
	}
	return v
}

func optionalBool(args json.RawMessage, field string) (bool, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(args, &m); err != nil {
		return false, false
	}
	raw, ok := m[field]
	if !ok {
		return false, false
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return false, false
	}
	return v, true
}

func stringSlice(args json.RawMessage, field string) ([]string, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(args, &m); err != nil {
		return nil, false
	}
	raw, ok := m[field]
	if !ok {
		return nil, false
	}
	var v []string
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, false
	}
	return v, true
}

// attemptSink adapts storage.LlmCallRepo to chat.AttemptSink, mapping an
// ai.Attempt's sanitized metadata onto one llm_calls row (purpose=chat).
type attemptSink struct {
	repo *storage.LlmCallRepo
}

func (s attemptSink) RecordAttempt(ctx context.Context, attempt ai.Attempt) {
	actualModel := (*string)(nil)
	if attempt.ActualModel != "" {
		m := attempt.ActualModel
		actualModel = &m
	}
	errKind := (*string)(nil)
	if attempt.ErrorKind != "" {
		k := string(attempt.ErrorKind)
		errKind = &k
	}
	httpStatus := (*int)(nil)
	if attempt.HTTPStatus != 0 {
		status := attempt.HTTPStatus
		httpStatus = &status
	}
	_ = s.repo.Insert(ctx, storage.LlmCall{
		BatchID:          attempt.BatchID,
		Purpose:          string(attempt.Purpose),
		AttemptNo:        attempt.AttemptNo,
		ProviderID:       attempt.ProviderID,
		Protocol:         string(attempt.Protocol),
		RequestedModel:   attempt.RequestedModel,
		ActualModel:      actualModel,
		StartedAt:        attempt.StartedAt,
		FinishedAt:       attempt.FinishedAt,
		Outcome:          attempt.Outcome,
		ErrorKind:        errKind,
		HTTPStatus:       httpStatus,
		InputTokens:      attempt.InputTokens,
		OutputTokens:     attempt.OutputTokens,
		CacheReadTokens:  attempt.CacheReadTokens,
		CacheWriteTokens: attempt.CacheWriteTokens,
	})
}
