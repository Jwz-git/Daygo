package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
)

/*
 * Single-card regeneration: the timeline's per-card "regenerate" rewrites the
 * card's own window from the evidence already stored for it. The fixtures
 * below pin the boundaries the batch pipeline cannot give it — one card out of
 * a batch's several, neighbours untouched, and a window that never moves.
 */

// regenerateHarness runs the pipeline once over 10:00-10:15 and returns the
// batch id plus the card it produced. A second card, the sibling a long
// activity was split into, is seeded with the same batch id — the shape the
// user sees as "regenerating either card regenerates both".
func regenerateHarness(t *testing.T) (*harness, int64, domain.TimelineCard, domain.TimelineCard) {
	t.Helper()
	h, batchID, target := pipelineCard(t)

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.seedCardBefore(t, batchID, base.Add(15*time.Minute), base.Add(20*time.Minute), "Coding", "sibling", "")
	after := h.cardsFor(t, "2026-09-12")
	if len(after) != 2 {
		t.Fatalf("cards = %+v, want the batch card plus its sibling", after)
	}
	var sibling domain.TimelineCard
	for _, c := range after {
		if c.ID != target.ID {
			sibling = c
		}
	}
	if sibling.BatchID == nil || *sibling.BatchID != batchID {
		t.Fatalf("sibling batch = %v, want the shared batch %d", sibling.BatchID, batchID)
	}
	return h, batchID, target, sibling
}

// pipelineCard is the bare setup: one succeeded batch over 10:00-10:15 with its
// stored observations and its one card.
func pipelineCard(t *testing.T) (*harness, int64, domain.TimelineCard) {
	t.Helper()
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"Working in an editor","apps":["Code"]}]}`,
		string(ai.PurposeCards): `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"editor",` +
			`"title":"Editing code","summary":"Working in an editor.","detailed_summary":"","appSites":["code.visualstudio.com"],` +
			`"distractions":[],"activityPoints":[]}]}`,
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	h.service.tick(context.Background())

	batchID := h.onlyBatch(t, base)
	cards := h.cardsFor(t, "2026-09-12")
	if len(cards) != 1 {
		t.Fatalf("cards = %+v, want the batch's one card", cards)
	}
	return h, batchID, cards[0]
}

func setCardsResponse(t *testing.T, h *harness, response string) {
	t.Helper()
	h.provider.mu.Lock()
	h.provider.responses[string(ai.PurposeCards)] = response
	h.provider.mu.Unlock()
}

func cardsPromptText(t *testing.T, h *harness) string {
	t.Helper()
	h.provider.mu.Lock()
	defer h.provider.mu.Unlock()
	for i := len(h.provider.calls) - 1; i >= 0; i-- {
		call := h.provider.calls[i]
		if string(call.Purpose) != string(ai.PurposeCards) {
			continue
		}
		for _, part := range call.Parts {
			if part.Kind() == ai.PartText {
				return part.Text()
			}
		}
	}
	t.Fatalf("no card prompt was sent")
	return ""
}

// The user's case: one long activity split into two cards that share a batch.
// Regenerating one must rewrite that card only — the sibling keeps its id, its
// span and its title.
func TestRegenerateCardRewritesOnlyItsOwnWindow(t *testing.T) {
	h, batchID, target, sibling := regenerateHarness(t)
	setCardsResponse(t, h, `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"editor",`+
		`"title":"Second take on the editor","summary":"S2","detailed_summary":"","appSites":["code.visualstudio.com"],`+
		`"distractions":[],"activityPoints":[]}]}`)

	if err := h.service.RegenerateCard(context.Background(), target); err != nil {
		t.Fatalf("RegenerateCard: %v", err)
	}

	cards := h.cardsFor(t, "2026-09-12")
	if len(cards) != 2 {
		t.Fatalf("cards = %+v, want two after the rewrite", cards)
	}
	var regenerated, untouched domain.TimelineCard
	for _, c := range cards {
		switch c.Title {
		case "Second take on the editor":
			regenerated = c
		case "sibling":
			untouched = c
		default:
			t.Fatalf("unexpected card %+v", c)
		}
	}
	if regenerated.ID == 0 {
		t.Fatalf("cards = %+v, want the regenerated card", cards)
	}
	if regenerated.Start != "10:00 AM" || regenerated.End != "10:15 AM" {
		t.Fatalf("regenerated span = %s-%s, want the unchanged window", regenerated.Start, regenerated.End)
	}
	if regenerated.BatchID == nil || *regenerated.BatchID != batchID {
		t.Fatalf("regenerated batch = %v, want its source batch %d kept", regenerated.BatchID, batchID)
	}
	if untouched.ID != sibling.ID || untouched.Title != "sibling" ||
		untouched.StartTs != sibling.StartTs || untouched.EndTs != sibling.EndTs {
		t.Fatalf("sibling = %+v, want the untouched row %+v", untouched, sibling)
	}
	// The rewrite replaced the card rather than editing it in place, and the
	// day notification fires so the timeline reloads.
	if regenerated.ID == target.ID {
		t.Fatalf("regenerated card kept id %d, want a new row", regenerated.ID)
	}
	if len(h.days) == 0 || h.days[len(h.days)-1] != "2026-09-12" {
		t.Fatalf("days = %v, want the day notification", h.days)
	}

	// The prompt is the scoped one: it fixes the window and never asks for a
	// merge into the preceding card.
	prompt := cardsPromptText(t, h)
	for _, want := range []string{"<scoped_rewrite>", "Rewrite exactly the current window and nothing outside it",
		"the first card starts at the window start and the last card ends at the window end"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("scoped prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "merging means one card whose start is the preceding card's start") ||
		strings.Contains(prompt, "continuing a previous card starts where that card starts") {
		t.Fatalf("scoped prompt carries the batch merge rule:\n%s", prompt)
	}
}

// A model that nudges the end a minute past the window (inside the validator's
// rounding slack) must not drag the rewrite over the neighbour: the stored card
// ends exactly at the window end, and the sibling survives. Without the pin the
// storage ownership constraint would reject the whole transaction.
func TestRegenerateCardPinsTheWindowWhenTheModelOverruns(t *testing.T) {
	h, _, target, sibling := regenerateHarness(t)
	setCardsResponse(t, h, `{"cards":[{"start":"10:00 AM","end":"10:16 AM","category":"Coding","subcategory":"",`+
		`"title":"Overrun take","summary":"S","detailed_summary":"","appSites":["code.visualstudio.com"],`+
		`"distractions":[],"activityPoints":[]}]}`)

	if err := h.service.RegenerateCard(context.Background(), target); err != nil {
		t.Fatalf("RegenerateCard: %v", err)
	}

	cards := h.cardsFor(t, "2026-09-12")
	if len(cards) != 2 {
		t.Fatalf("cards = %+v, want two", cards)
	}
	for _, c := range cards {
		switch c.Title {
		case "Overrun take":
			if c.End != "10:15 AM" || c.EndTs != target.EndTs {
				t.Fatalf("regenerated end = %s (%d), want the window end %s (%d)",
					c.End, c.EndTs, "10:15 AM", target.EndTs)
			}
		case "sibling":
			if c.ID != sibling.ID || c.EndTs != sibling.EndTs {
				t.Fatalf("sibling = %+v, want the untouched row %+v", c, sibling)
			}
		default:
			t.Fatalf("unexpected card %+v", c)
		}
	}
}

// A model that insists on covering time outside the window fails the
// regeneration after the correction attempts and leaves the stored card alone:
// the fixture's provider answers every attempt with the same overreaching JSON.
func TestRegenerateCardRejectsAPersistentOverrunWithoutWriting(t *testing.T) {
	h, _, target, sibling := regenerateHarness(t)
	setCardsResponse(t, h, `{"cards":[{"start":"9:30 AM","end":"10:30 AM","category":"Coding","subcategory":"",`+
		`"title":"Greedy take","summary":"S","detailed_summary":"","appSites":[],`+
		`"distractions":[],"activityPoints":[]}]}`)

	err := h.service.RegenerateCard(context.Background(), target)
	if err == nil {
		t.Fatalf("RegenerateCard accepted a card reaching outside the window")
	}
	if !strings.Contains(err.Error(), "failed validation after 3 attempts") {
		t.Fatalf("err = %v, want the validation failure", err)
	}

	cards := h.cardsFor(t, "2026-09-12")
	if len(cards) != 2 {
		t.Fatalf("cards = %+v, want the two original cards", cards)
	}
	for _, c := range cards {
		switch c.ID {
		case target.ID:
			if c.Title != "Editing code" || c.Start != "10:00 AM" || c.End != "10:15 AM" {
				t.Fatalf("target = %+v, want it untouched", c)
			}
		case sibling.ID:
			if c.Title != "sibling" {
				t.Fatalf("sibling = %+v, want it untouched", c)
			}
		default:
			t.Fatalf("unexpected card %+v", c)
		}
	}
}

// The replaced card's appSites are the only place the new card's icon can come
// from when the model names no app: regeneration must not cost the card its
// icon.
func TestRegenerateCardInheritsTheReplacedCardsAppSites(t *testing.T) {
	h, _, target, _ := regenerateHarness(t)
	setCardsResponse(t, h, `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"",`+
		`"title":"Names no app","summary":"S","detailed_summary":"","appSites":[],`+
		`"distractions":[],"activityPoints":[]}]}`)

	if err := h.service.RegenerateCard(context.Background(), target); err != nil {
		t.Fatalf("RegenerateCard: %v", err)
	}

	cards := h.cardsFor(t, "2026-09-12")
	for _, c := range cards {
		if c.Title != "Names no app" {
			continue
		}
		sites := appSitesOfMetadata(c.Metadata)
		if sites == nil || sites.Primary == nil || *sites.Primary != "code.visualstudio.com" {
			t.Fatalf("appSites = %+v, want the replaced card's code.visualstudio.com", sites)
		}
		return
	}
	t.Fatalf("cards = %+v, want the regenerated card", cards)
}

// A model that does name an app keeps its choice: inheritance fills an empty
// list, it does not overrule the model.
func TestRegenerateCardKeepsTheModelsOwnAppSites(t *testing.T) {
	h, _, target, _ := regenerateHarness(t)
	setCardsResponse(t, h, `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"",`+
		`"title":"Names an app","summary":"S","detailed_summary":"","appSites":["github.com"],`+
		`"distractions":[],"activityPoints":[]}]}`)

	if err := h.service.RegenerateCard(context.Background(), target); err != nil {
		t.Fatalf("RegenerateCard: %v", err)
	}

	for _, c := range h.cardsFor(t, "2026-09-12") {
		if c.Title != "Names an app" {
			continue
		}
		sites := appSitesOfMetadata(c.Metadata)
		if sites == nil || sites.Primary == nil || *sites.Primary != "github.com" {
			t.Fatalf("appSites = %+v, want the model's github.com", sites)
		}
		return
	}
	t.Fatalf("regenerated card not found")
}

// A window with no stored observations has no evidence to rewrite from: the
// call fails loudly and spends no provider call rather than inventing a card.
func TestRegenerateCardWithoutStoredEvidenceFails(t *testing.T) {
	h, batchID, _, _ := regenerateHarness(t)

	// A card far from the analyzed window: the batch's observations cover
	// 10:00-10:15 only.
	windowStart := time.Date(2026, 9, 12, 14, 0, 0, 0, time.Local)
	h.seedCardBefore(t, batchID, windowStart, windowStart.Add(10*time.Minute), "Coding", "no evidence", "")
	var orphan domain.TimelineCard
	for _, c := range h.cardsFor(t, "2026-09-12") {
		if c.Title == "no evidence" {
			orphan = c
		}
	}
	if orphan.ID == 0 {
		t.Fatalf("orphan card was not seeded")
	}

	before := h.provider.callCount(string(ai.PurposeCards))
	err := h.service.RegenerateCard(context.Background(), orphan)
	if err == nil || !strings.Contains(err.Error(), "no stored observations") {
		t.Fatalf("err = %v, want the missing-evidence failure", err)
	}
	if after := h.provider.callCount(string(ai.PurposeCards)); after != before {
		t.Fatalf("card calls = %d, want %d: no evidence must not reach the provider", after, before)
	}
	for _, c := range h.cardsFor(t, "2026-09-12") {
		if c.ID == orphan.ID && c.Title != "no evidence" {
			t.Fatalf("orphan card = %+v, want it untouched", c)
		}
	}
}

// A card with no batch provenance (a System fallback) has nothing to attribute
// a rewrite to and no evidence trail: the call is refused before any read.
func TestRegenerateCardWithoutSourceBatchIsRefused(t *testing.T) {
	h := newHarness(t, nil)
	err := h.service.RegenerateCard(context.Background(), domain.TimelineCard{
		ID: 7, Start: "10:00 AM", End: "10:15 AM",
		StartTs: time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local).Unix(),
		EndTs:   time.Date(2026, 9, 12, 10, 15, 0, 0, time.Local).Unix(),
	})
	if !errors.Is(err, ErrNoSourceBatch) {
		t.Fatalf("err = %v, want ErrNoSourceBatch", err)
	}
}

// A batch still pending or processing over the same window will rewrite these
// cards when it lands, which would discard the regeneration's result: the call
// is refused instead of producing a card that silently disappears.
func TestRegenerateCardRefusesALiveBatchOverTheWindow(t *testing.T) {
	h := newHarness(t, map[string]string{
		string(ai.PurposeTranscribe): `{"observations":[{"from_frame":0,"to_frame":89,"observation":"working","apps":[]}]}`,
		string(ai.PurposeCards):      `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"","title":"Editing","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`,
	})
	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	frames := h.commitFrames(t, base, 92, 10*time.Second, func(int) *int { return intPtr(5) })
	// The recorder still owns the newest frame's segment, so the batch stays
	// pending — the live state a regeneration must not race.
	h.activeSegment = frames[90].SegmentPath
	h.service.tick(context.Background())

	batches := mustBatches(t, h.store)
	if len(batches) != 1 || batches[0].Status != storage.BatchPending {
		t.Fatalf("batches = %+v, want one pending", batches)
	}
	// A card of that pending window, seeded directly: the pipeline has not
	// produced one yet.
	h.seedCardBefore(t, batches[0].ID, base, base.Add(15*time.Minute), "Coding", "pending window card", "")
	var live domain.TimelineCard
	for _, c := range h.cardsFor(t, "2026-09-12") {
		if c.Title == "pending window card" {
			live = c
		}
	}

	err := h.service.RegenerateCard(context.Background(), live)
	if err == nil || !strings.Contains(err.Error(), "still being analyzed") {
		t.Fatalf("err = %v, want the live-batch refusal", err)
	}
	for _, c := range h.cardsFor(t, "2026-09-12") {
		if c.Title != "pending window card" {
			t.Fatalf("cards = %+v, want the seeded card untouched", c)
		}
	}
}

// A window the model splits into several cards is allowed: the constraint is
// that the cards tile the window, not that there is exactly one. The window
// here is widened to 10:00-10:40, because the 15-minute floor leaves no room
// for two cards inside a single batch's quarter hour.
func TestRegenerateCardAcceptsASplitWindow(t *testing.T) {
	h, _, stored := pipelineCard(t)
	target := stored
	target.End = "10:40 AM"
	target.EndTs = time.Date(2026, 9, 12, 10, 40, 0, 0, time.Local).Unix()
	setCardsResponse(t, h, `{"cards":[`+
		`{"start":"10:00 AM","end":"10:20 AM","category":"Coding","subcategory":"","title":"first half","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]},`+
		`{"start":"10:20 AM","end":"10:40 AM","category":"Coding","subcategory":"","title":"second half","summary":"S","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`)

	if err := h.service.RegenerateCard(context.Background(), target); err != nil {
		t.Fatalf("RegenerateCard: %v", err)
	}

	var spans []string
	for _, c := range h.cardsFor(t, "2026-09-12") {
		spans = append(spans, c.Start+"-"+c.End)
	}
	want := map[string]bool{"10:00 AM-10:20 AM": true, "10:20 AM-10:40 AM": true}
	if len(spans) != 2 || !want[spans[0]] || !want[spans[1]] {
		t.Fatalf("spans = %v, want the two halves tiling the window", spans)
	}
}

// The metadata written by a regeneration is the same stored shape the batch
// pipeline writes: the inspector reads distractions and activity points from
// it, and a regeneration is not allowed to produce a differently shaped row.
func TestRegenerateCardWritesTheContractMetadataShape(t *testing.T) {
	h, _, target, _ := regenerateHarness(t)
	setCardsResponse(t, h, `{"cards":[{"start":"10:00 AM","end":"10:15 AM","category":"Coding","subcategory":"editor",`+
		`"title":"With points","summary":"S","detailed_summary":"D","appSites":["code.visualstudio.com"],`+
		`"distractions":[{"start":"10:05 AM","end":"10:07 AM","title":"checked a feed","summary":""}],`+
		`"activityPoints":[{"time":"10:03 AM","description":"edited a file"}]}]}`)

	if err := h.service.RegenerateCard(context.Background(), target); err != nil {
		t.Fatalf("RegenerateCard: %v", err)
	}

	for _, c := range h.cardsFor(t, "2026-09-12") {
		if c.Title != "With points" {
			continue
		}
		var meta struct {
			Distractions   []map[string]any `json:"distractions"`
			ActivityPoints []map[string]any `json:"activityPoints"`
		}
		if err := json.Unmarshal([]byte(c.Metadata), &meta); err != nil {
			t.Fatalf("metadata = %q: %v", c.Metadata, err)
		}
		if len(meta.Distractions) != 1 || meta.Distractions[0]["startTime"] != "10:05 AM" ||
			meta.Distractions[0]["endTime"] != "10:07 AM" {
			t.Fatalf("distractions = %+v, want the startTime/endTime contract shape", meta.Distractions)
		}
		if len(meta.ActivityPoints) != 1 || meta.ActivityPoints[0]["time"] != "10:03 AM" {
			t.Fatalf("activityPoints = %+v, want the stored point", meta.ActivityPoints)
		}
		return
	}
	t.Fatalf("regenerated card not found")
}
