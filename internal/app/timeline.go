package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/analysis"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/recorder"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

const timelineTimeout = 10 * time.Second

// cardRegenerationTimeout bounds the synchronous single-card rewrite, which
// spends one model call on top of its storage reads. It is far longer than the
// storage timeout because the model round trip is the bulk of it; the ai layer
// still applies its own per-request timeout inside this one.
const cardRegenerationTimeout = 5 * time.Minute

// timelineEventMergeWindow is the docs/05 §5.5.3 rule: timeline:updated is an
// invalidation event, so multiple writes to the same day within this window
// collapse into one emit. Lossy by design — views re-pull, they never consume
// the payload as data.
const timelineEventMergeWindow = 200 * time.Millisecond

// parseCardMetadata decodes the subset of timeline_cards.metadata the UI
// renders (appSites, distractions, activityPoints) out of the opaque JSON
// column. Parsing is tolerant: a card whose metadata is absent or shaped
// differently renders without those decorations rather than failing the whole
// day view.
//
// Every decoration decodes on its own. Metadata is model output the pipeline
// stores as it arrives, so one field can drift away from the shape this layer
// reads while its siblings stay intact; decoding them together made a single
// mismatched field take the whole card with it. A live card whose metadata
// carried a flat `distractions` list (the model's shape) failed the shared
// decode and lost appSites with it, which emptied the icon slot on every
// timeline view. A field that does not decode is dropped, never substituted.
func parseCardMetadata(raw string) (appSites *AppSitesDTO, distractions []DistractionDTO, activityPoints []ActivityPointDTO) {
	if raw == "" {
		return nil, nil, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return nil, nil, nil
	}

	var sites *AppSitesDTO
	if decodeMetadataField(fields["appSites"], &sites) {
		appSites = sites
	}
	var distractionList []DistractionDTO
	if decodeMetadataField(fields["distractions"], &distractionList) {
		// Metadata carries no id of its own — the pipeline stores what the model
		// reported — so one is assigned here, in list order. docs/05 §5.5.2 asks
		// for exactly that ("metadata 中缺失时由 Go 生成，保持稳定"): the
		// inspector keys its rows by this id, and a position in a stored list is
		// stable for as long as the list is.
		for i := range distractionList {
			if distractionList[i].ID == "" {
				distractionList[i].ID = fmt.Sprintf("d%d", i+1)
			}
		}
		distractions = distractionList
	}
	var points []ActivityPointDTO
	if decodeMetadataField(fields["activityPoints"], &points) {
		activityPoints = points
	}
	return appSites, distractions, activityPoints
}

// decodeMetadataField reports whether one metadata field decoded into target.
// Absent fields and fields shaped differently both report false, which leaves
// the caller's own nil in place instead of a half-filled value.
func decodeMetadataField(raw json.RawMessage, target any) bool {
	if len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, target) == nil
}

// categoryByID indexes categories by name for per-card isIdle resolution.
type categoryFlags struct {
	isIdle map[string]bool
}

func categoryFlagsFrom(list []domain.Category) categoryFlags {
	flags := categoryFlags{isIdle: make(map[string]bool, len(list))}
	for _, c := range list {
		flags.isIdle[c.Name] = c.IsIdle
	}
	return flags
}

// retryableFailure reports whether a failed batch is worth retrying, for the
// day view's failure panel. Mirrors the batch:failed event's classification
// (retryableFailureKind in analysis_wiring.go).
func retryableFailure(kind string, attempts int) bool {
	switch kind {
	case "auth", "invalid_request", "no_provider":
		return false
	}
	return attempts < storage.MaxBatchAttempts
}

// mergeFailures groups failed batches whose windows are adjacent within the
// 60-second tolerance (docs/05 §5.5.2 TimelineFailureDTO), so a burst of small
// batch failures renders as one panel entry with all their ids. A group carries
// the retryable flag of its first batch; merged entries are adjacent in time
// and produced by the same failure event, so the flags agree in practice.
func mergeFailures(batches []failedBatchView) []TimelineFailureDTO {
	if len(batches) == 0 {
		// The wire contract declares failures as an array. A nil slice encodes
		// as null and violates the generated TimelineFailureDTO[] type.
		return []TimelineFailureDTO{}
	}
	groups := make([]TimelineFailureDTO, 0, len(batches))
	current := TimelineFailureDTO{
		BatchIDs:  []int64{batches[0].ID},
		StartTs:   batches[0].StartTs,
		EndTs:     batches[0].EndTs,
		Kind:      batches[0].FailureKind,
		Message:   batches[0].FailureNote,
		Retryable: retryableFailure(batches[0].FailureKind, batches[0].Attempts),
	}
	for _, b := range batches[1:] {
		if b.StartTs-current.EndTs <= 60 {
			current.BatchIDs = append(current.BatchIDs, b.ID)
			if b.EndTs > current.EndTs {
				current.EndTs = b.EndTs
			}
			continue
		}
		groups = append(groups, current)
		current = TimelineFailureDTO{
			BatchIDs:  []int64{b.ID},
			StartTs:   b.StartTs,
			EndTs:     b.EndTs,
			Kind:      b.FailureKind,
			Message:   b.FailureNote,
			Retryable: retryableFailure(b.FailureKind, b.Attempts),
		}
	}
	return append(groups, current)
}

// failedBatchView is storage.FailedBatch viewed as timeline input.
type failedBatchView struct {
	ID          int64
	StartTs     int64
	EndTs       int64
	Status      string
	FailureKind string
	FailureNote string
	Attempts    int
}

// emitTimelineInvalidation schedules a merged timeline:updated emit for day.
// The timer map is small (one entry per day touched within 200 ms) and
// self-cleaning; dropping events on shutdown is acceptable because they are
// invalidation-only.
func (b *Backend) emitTimelineInvalidation(day string) {
	b.timelineEventsMu.Lock()
	defer b.timelineEventsMu.Unlock()
	if b.timelineEvents == nil {
		b.timelineEvents = make(map[string]*time.Timer)
	}
	if _, pending := b.timelineEvents[day]; pending {
		return
	}
	b.timelineEvents[day] = time.AfterFunc(timelineEventMergeWindow, func() {
		b.timelineEventsMu.Lock()
		delete(b.timelineEvents, day)
		b.timelineEventsMu.Unlock()
		b.emitter.Emit(EventTimelineUpdated, TimelineUpdatedPayload{Day: day})
	})
}

// TimelineUpdatedPayload is the invalidation-only payload (docs/05 §5.5.3).
type TimelineUpdatedPayload struct {
	Day string `json:"day"`
}

// GetTimelineDay returns one logical day's cards, categories, totals, and
// failure groups in a single call (docs/05 §5.3.4).
func (b *Backend) GetTimelineDay(day string) (TimelineDayDTO, error) {
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return TimelineDayDTO{}, mapStorageError("get timeline day", err)
		}
		return TimelineDayDTO{}, apperr.E(apperr.DatabaseError, "timeline requires a database", nil)
	}
	loc := store.Location()
	start, end, err := timeutil.DayWindow(day, loc)
	if err != nil {
		return TimelineDayDTO{}, apperr.E(apperr.InvalidArgument, "day must use yyyy-MM-dd", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()

	cards, err := store.Cards().CardsForDay(ctx, day)
	if err != nil {
		return TimelineDayDTO{}, mapStorageError("get timeline day", err)
	}
	categories, err := store.Categories().List(ctx)
	if err != nil {
		return TimelineDayDTO{}, mapStorageError("get timeline day", err)
	}
	failedBatches, err := store.Cards().FailedBatchesInRange(ctx, start, end)
	if err != nil {
		return TimelineDayDTO{}, mapStorageError("get timeline day", err)
	}
	processingBatches, err := store.Analysis().ProcessingBatchesInRange(ctx, start, end)
	if err != nil {
		return TimelineDayDTO{}, mapStorageError("get timeline day", err)
	}

	flags := categoryFlagsFrom(categories)
	dto := TimelineDayDTO{
		Day:              day,
		DayStartTs:       start.Unix(),
		DayEndTs:         end.Unix(),
		Cards:            make([]TimelineCardDTO, 0, len(cards)),
		Categories:       make([]CategoryDTO, 0, len(categories)),
		GeneratedAtTs:    b.clock.Now().Unix(),
		ProcessingRanges: make([]RangeDTO, 0, len(processingBatches)),
	}
	for _, batch := range processingBatches {
		dto.ProcessingRanges = append(dto.ProcessingRanges, RangeDTO{
			StartTs:  batch.Start.Unix(),
			EndTs:    batch.End.Unix(),
			BatchIDs: []int64{batch.ID},
		})
	}
	for _, card := range cards {
		if card.EndTs <= card.StartTs || card.EndTs-card.StartTs > 4*3600 {
			continue
		}
		visible := sharedCardDTO(card, flags)
		visible.StartTs = max(card.StartTs, start.Unix())
		visible.EndTs = min(card.EndTs, end.Unix())
		visible.DurationMinutes = float64(visible.EndTs-visible.StartTs) / 60
		dto.Cards = append(dto.Cards, visible)
		if card.Category == "System" {
			continue
		}
		minutes := visible.DurationMinutes
		if flags.isIdle[card.Category] {
			dto.IdleMinutes += minutes
		} else {
			dto.TrackedMinutes += minutes
		}
	}
	for _, c := range categories {
		dto.Categories = append(dto.Categories, CategoryDTO{
			ID:          c.ID,
			Name:        c.Name,
			ColorHex:    c.ColorHex,
			Details:     c.Details,
			SortOrder:   c.SortOrder,
			IsSystem:    c.IsSystem,
			IsIdle:      c.IsIdle,
			CreatedAtTs: c.CreatedAtUnix,
			UpdatedAtTs: c.UpdatedAtUnix,
		})
	}
	views := make([]failedBatchView, 0, len(failedBatches))
	for _, fb := range failedBatches {
		views = append(views, failedBatchView(fb))
	}
	dto.Failures = mergeFailures(views)
	return dto, nil
}

func cardDurationMinutes(card domain.TimelineCard) float64 {
	if card.EndTs <= card.StartTs {
		return 0
	}
	return float64(card.EndTs-card.StartTs) / 60.0
}

// sharedCardDTO assembles one card's DTO. Both the day view and the chat card
// read tool go through it, so a card renders identically wherever it appears.
// videoSummaryUrl stays null and otherVideoSummaryUrls empty: real URLs are
// the media slice's concern, not a path-to-URL guess.
func sharedCardDTO(card domain.TimelineCard, flags categoryFlags) TimelineCardDTO {
	appSites, distractions, activityPoints := parseCardMetadata(card.Metadata)
	// The wire contract declares these as arrays; nil slices encode as null
	// and crash consumers doing .length on them (same rule mergeFailures
	// already follows).
	if distractions == nil {
		distractions = []DistractionDTO{}
	}
	if activityPoints == nil {
		activityPoints = []ActivityPointDTO{}
	}
	return TimelineCardDTO{
		ID:                    card.ID,
		BatchID:               card.BatchID,
		Day:                   card.Day,
		Start:                 card.Start,
		End:                   card.End,
		StartTs:               card.StartTs,
		EndTs:                 card.EndTs,
		Category:              card.Category,
		Subcategory:           card.Subcategory,
		Title:                 card.Title,
		Summary:               card.Summary,
		DetailedSummary:       card.DetailedSummary,
		OtherVideoSummaryURLs: []string{},
		AppSites:              appSites,
		Distractions:          distractions,
		ActivityPoints:        activityPoints,
		IsIdle:                flags.isIdle[card.Category],
		DurationMinutes:       cardDurationMinutes(card),
	}
}

// requireTimelineWrite rejects write methods on instances without the write
// lock (docs/05 §5.6.2 rule 7).
func (b *Backend) requireTimelineWrite() error {
	canWrite, _ := b.instanceOwnership()
	if !canWrite {
		return apperr.E(apperr.NotCaptureOwner, "this instance is not the capture owner", nil)
	}
	return nil
}

// cardDay returns the persisted ownership day of one card.
func (b *Backend) cardDay(ctx context.Context, cardID int64) (string, error) {
	store := b.store()
	card, err := store.Cards().CardByID(ctx, cardID)
	if err != nil {
		return "", mapStorageError("load card", err)
	}
	return card.Day, nil
}

// cardVisibleDays includes the continuation day when one persisted card spans
// 4 AM. A write to that card must refresh both timeline projections.
func (b *Backend) cardVisibleDays(ctx context.Context, cardID int64) ([]string, error) {
	store := b.store()
	card, err := store.Cards().CardByID(ctx, cardID)
	if err != nil {
		return nil, mapStorageError("load card", err)
	}
	days := []string{card.Day}
	if card.EndTs > card.StartTs {
		last := timeutil.LogicalDay(time.Unix(card.EndTs-1, 0), store.Location())
		if last != card.Day {
			days = append(days, last)
		}
	}
	return days, nil
}

// UpdateCardCategory moves one card to an existing category name. Unknown
// names are rejected — categories are never created implicitly (docs/05 §5.5.1).
func (b *Backend) UpdateCardCategory(cardID int64, category string) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	return b.updateCardCategory(ctx, cardID, category)
}

// UpdateCardTitle renames one card.
func (b *Backend) UpdateCardTitle(cardID int64, title string) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	return b.updateCardTitle(ctx, cardID, title)
}

// UpdateCardSummary rewrites one card's short summary. Empty clears it.
func (b *Backend) UpdateCardSummary(cardID int64, text string) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	return b.updateCardSummary(ctx, cardID, text)
}

// UpdateCardDetailedSummary rewrites one card's long-form summary. Empty
// clears it.
func (b *Backend) UpdateCardDetailedSummary(cardID int64, text string) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	return b.updateCardDetailedSummary(ctx, cardID, text)
}

// DeleteCard soft-deletes one card.
func (b *Backend) DeleteCard(cardID int64) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	return b.deleteCard(ctx, cardID)
}

// SaveCategories replaces the whole user category set (docs/05 §5.2.1). The
// input carries only user categories; the built-ins are merged back inside
// the storage transaction. A rename rewrites timeline_cards in the same
// transaction, so the days whose cards were renamed must refresh.
func (b *Backend) SaveCategories(categories []CategoryDTO) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()

	store := b.store()
	before, err := store.Categories().List(ctx)
	if err != nil {
		return mapStorageError("save categories", err)
	}
	domainRows, err := categoryDTOsToDomain(categories)
	if err != nil {
		return err
	}
	if err := b.saveCategories(ctx, domainRows); err != nil {
		return err
	}

	// A renamed category rewrites card rows, so every day holding the
	// renamed category's cards needs a timeline:updated event. The rewrite
	// has already happened, so query by the new names.
	days, err := store.Cards().CardDaysByCategory(ctx, renamedCategoryNames(before, categories))
	if err != nil {
		days = nil
	}
	for _, day := range days {
		b.emitTimelineInvalidation(day)
	}
	return nil
}

// renamedCategoryNames returns the new names of categories whose name changed
// between the stored set and the incoming DTO rows (matched by id; a row
// without an id is new and renames nothing). The card rows already carry the
// new names when this runs.
func renamedCategoryNames(before []domain.Category, next []CategoryDTO) []string {
	nextByID := make(map[string]CategoryDTO, len(next))
	for _, r := range next {
		if r.ID != "" && !r.IsSystem {
			nextByID[r.ID] = r
		}
	}
	var renamed []string
	for _, c := range before {
		if next, ok := nextByID[c.ID]; ok && next.Name != c.Name {
			renamed = append(renamed, next.Name)
		}
	}
	return renamed
}

// categoryDTOsToDomain maps binding-layer rows to the storage shape. A row
// claiming is_system is rejected, not silently dropped: is_system is assigned
// by the database, never the caller (docs/03 §3.5), and a caller that thinks
// it can write built-ins has a stale model of the contract.
func categoryDTOsToDomain(rows []CategoryDTO) ([]domain.Category, error) {
	out := make([]domain.Category, 0, len(rows))
	for _, r := range rows {
		if r.IsSystem {
			return nil, apperr.E(apperr.InvalidArgument,
				fmt.Sprintf("category %q claims isSystem; built-ins are managed by the database", r.Name), nil)
		}
		out = append(out, domain.Category{
			ID: r.ID, Name: r.Name, ColorHex: r.ColorHex, Details: r.Details,
			SortOrder: r.SortOrder, IsIdle: r.IsIdle,
		})
	}
	return out, nil
}

// RetryBatches requeues failed batches by explicit user action: status back
// to pending with the failure info cleared and the attempt counter reset
// (docs/05 §5.2.1). It returns immediately; the scheduler picks the batches
// up on its next tick and progress arrives as batch:progress /
// timeline:updated events.
func (b *Backend) RetryBatches(batchIDs []int64) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return mapStorageError("retry batches", err)
		}
		return apperr.E(apperr.DatabaseError, "retry batches requires a database", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	batches, err := store.Analysis().RetryBatches(ctx, batchIDs, b.clock.Now())
	if err != nil {
		return mapStorageError("retry batches", err)
	}
	for _, batch := range batches {
		b.emitTimelineInvalidation(timeutil.LogicalDay(batch.Start, b.clock.Now().Location()))
	}
	return nil
}

// StopRetries stops the named failed batches from auto-requeueing after their
// cooldown. It is the inverse of RetryBatches: rather than resetting the
// attempt counter it caps it, so RequeueFailed leaves the batch in its failed
// terminal state and the failure panel stops promising an automatic retry. The
// batch stays visible and a later RetryBatches still overrides this, so the
// user can resume analyzing it. No batch is deleted and no LLM call is made.
func (b *Backend) StopRetries(batchIDs []int64) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return mapStorageError("stop retries", err)
		}
		return apperr.E(apperr.DatabaseError, "stop retries requires a database", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	batches, err := store.Analysis().StopRetries(ctx, batchIDs, b.clock.Now())
	if err != nil {
		return mapStorageError("stop retries", err)
	}
	for _, batch := range batches {
		b.emitTimelineInvalidation(timeutil.LogicalDay(batch.Start, b.clock.Now().Location()))
	}
	return nil
}

// ReprocessDay requeues one logical day's terminal batches (succeeded,
// failed, failed_empty) for re-analysis — the explicit user path to rebuild
// cards with an updated prompt. Dismissed and skipped-short batches stay as
// they are. It returns immediately; the scheduler picks the pending batches
// up on its next tick and progress arrives as batch:progress /
// timeline:updated events.
func (b *Backend) ReprocessDay(day string) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return mapStorageError("reprocess day", err)
		}
		return apperr.E(apperr.DatabaseError, "reprocess day requires a database", nil)
	}
	loc := store.Location()
	start, end, err := timeutil.DayWindow(day, loc)
	if err != nil {
		return apperr.E(apperr.InvalidArgument, "day must use yyyy-MM-dd", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	requeued, err := store.Analysis().ReprocessDay(ctx, start, end, b.clock.Now())
	if err != nil {
		return mapStorageError("reprocess day", err)
	}
	if len(requeued) > 0 {
		b.emitTimelineInvalidation(timeutil.LogicalDay(start, loc))
	}
	return nil
}

// ReprocessCard regenerates one card: the card's own window is re-analyzed
// from the evidence already stored for it and rewritten in place, leaving the
// cards on either side — including the siblings the same batch produced —
// untouched (analysis.Service.RegenerateCard). It is not the batch requeue it
// replaced: a batch writes every card of its rewrite span with one batch id, so
// requeueing it regenerated the card's siblings too, and the sliding window's
// merge rule could extend the rewrite back over the preceding card.
//
// The call is synchronous — the user asked for this one card, and its result or
// its failure is the answer. Progress reaches the UI through the same
// timeline:updated invalidation every other card write uses.
func (b *Backend) ReprocessCard(cardID int64) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	if cardID <= 0 {
		return apperr.E(apperr.InvalidArgument, "invalid card id", nil)
	}
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return mapStorageError("reprocess card", err)
		}
		return apperr.E(apperr.DatabaseError, "reprocess card requires a database", nil)
	}
	pipeline := b.analysisService()
	ctx, cancel := context.WithTimeout(context.Background(), cardRegenerationTimeout)
	defer cancel()
	card, err := store.Cards().CardByID(ctx, cardID)
	if err != nil {
		return mapStorageError("reprocess card", err)
	}
	// A card without batch provenance (a System fallback) is unregenerable
	// whatever else is available, so it is answered before the pipeline check.
	if card.BatchID == nil {
		return apperr.E(apperr.InvalidArgument, "card has no batch to regenerate", nil)
	}
	if pipeline == nil {
		// No pipeline started: a read-only instance or a failed startup. There
		// is no path that could rewrite the card.
		return apperr.E(apperr.Internal, "reprocess card requires the analysis pipeline", nil)
	}
	if err := pipeline.RegenerateCard(ctx, card); err != nil {
		return mapCardRegenerationError(err)
	}
	return nil
}

// DeleteBatches dismisses failed batches from the timeline's failure panel
// (soft delete: the rows and frame membership stay, so the frames never get
// re-analyzed). Configuration and cards are untouched.
func (b *Backend) DeleteBatches(batchIDs []int64) error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return mapStorageError("delete batches", err)
		}
		return apperr.E(apperr.DatabaseError, "delete batches requires a database", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	batches, err := store.Analysis().DeleteBatches(ctx, batchIDs, b.clock.Now())
	if err != nil {
		return mapStorageError("delete batches", err)
	}
	for _, batch := range batches {
		b.emitTimelineInvalidation(timeutil.LogicalDay(batch.Start, b.clock.Now().Location()))
	}
	return nil
}

// mapCardRegenerationError turns a failed single-card rewrite into a binding
// error. The analysis layer's own messages are fixed sanitized strings and
// never carry provider bodies or paths, so they pass through; the sentinel
// errors pick the code, never a match on message text (docs/05 §5.6.2).
func mapCardRegenerationError(err error) error {
	switch {
	case errors.Is(err, analysis.ErrNoSourceBatch):
		return apperr.E(apperr.InvalidArgument, "card has no batch to regenerate", err)
	case errors.Is(err, analysis.ErrNoCardEvidence):
		return apperr.E(apperr.InvalidArgument, "reprocess card: "+err.Error(), err)
	case errors.Is(err, analysis.ErrWindowInAnalysis):
		return apperr.E(apperr.Conflict, "reprocess card: "+err.Error(), err)
	case errors.Is(err, analysis.ErrCardsInvalid):
		return apperr.E(apperr.ProviderFailed, "reprocess card: "+err.Error(), err)
	case errors.Is(err, ai.ErrNoProvider):
		return apperr.E(apperr.ProviderNotConfigured, "reprocess card: no provider is configured", err)
	}
	var aiErr *ai.Error
	if errors.As(err, &aiErr) {
		return apperr.E(apperr.ProviderFailed, "reprocess card: "+err.Error(), err)
	}
	if _, ok := storage.KindOf(err); ok {
		return mapStorageError("reprocess card", err)
	}
	return apperr.E(apperr.Internal, "reprocess card: "+err.Error(), err)
}

// ClearHistoryData is the test-only one-click reset: it wipes recorded and
// analyzed history (frames, batches, observations, cards, journal, goals,
// chat) plus the recordings files, keeping configuration — settings,
// providers, categories — untouched. Refused while the recorder is running:
// an active capture writes into the very directories being removed.
func (b *Backend) ClearHistoryData() error {
	if err := b.requireTimelineWrite(); err != nil {
		return err
	}
	if state := b.recorderState(); state != recorder.StateIdle {
		return apperr.E(apperr.Conflict, "stop recording before clearing history data", nil)
	}
	store := b.store()
	if store == nil {
		if err := b.storageFailure(); err != nil {
			return mapStorageError("clear history data", err)
		}
		return apperr.E(apperr.DatabaseError, "history data requires a database", nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	if _, err := store.ClearHistoryData(ctx); err != nil {
		return mapStorageError("clear history data", err)
	}

	// Files after the rows: file deletion cannot roll back with the database,
	// so a failure here leaves orphan files for the next clear, not rows
	// pointing at missing files.
	recordings := filepath.Join(filepath.Dir(store.Path()), "recordings")
	for _, dir := range []string{"staging", "segments", "timelapses"} {
		if err := os.RemoveAll(filepath.Join(recordings, dir)); err != nil {
			return apperr.E(apperr.Internal, "remove recordings "+dir+": "+err.Error(), nil)
		}
		if err := os.MkdirAll(filepath.Join(recordings, dir), 0o700); err != nil {
			return apperr.E(apperr.Internal, "recreate recordings "+dir+": "+err.Error(), nil)
		}
	}

	// Empty day means "every day": listeners re-pull whatever they show.
	b.emitter.Emit(EventTimelineUpdated, TimelineUpdatedPayload{Day: ""})
	b.emitter.Emit(EventJournalUpdated, JournalUpdatedPayload{Day: ""})
	b.emitter.Emit(EventGoalUpdated, GoalUpdatedPayload{Day: ""})
	return nil
}

// invalidateCardDay resolves a card's visible days after a successful write and
// schedules the merged invalidation emit. The write already succeeded, so a
// failure to reload the card (concurrent delete) still emits nothing wrong:
// the day is simply unknown, and no emit is the acceptable degraded case.
func (b *Backend) invalidateCardDay(cardID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	if days, err := b.cardVisibleDays(ctx, cardID); err == nil {
		for _, day := range days {
			b.emitTimelineInvalidation(day)
		}
	}
}
