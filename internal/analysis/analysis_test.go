package analysis

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// framesAt builds n frames spaced by interval starting at base, each with the
// idle reading produced by idleAt (nil = no sample).
func framesAt(base time.Time, n int, interval time.Duration, idleAt func(i int) *int) []storage.AnalysisFrame {
	frames := make([]storage.AnalysisFrame, n)
	for i := range frames {
		var idle *int
		if idleAt != nil {
			idle = idleAt(i)
		}
		frames[i] = storage.AnalysisFrame{
			ID: int64(i + 1), SegmentPath: "staging/f.jpg",
			CapturedAt: base.Add(time.Duration(i) * interval), IdleSeconds: idle,
		}
	}
	return frames
}

func intPtr(v int) *int { return &v }

var base = time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)

// groupFrames must honor the per-chain image cap so no group exceeds what the
// tightest gateway in the fallback chain accepts.
func TestGroupFramesHonorsImageCap(t *testing.T) {
	frames := make([]storage.AnalysisFrame, 10)
	for i := range frames {
		frames[i] = storage.AnalysisFrame{FileSize: 1024}
	}

	groups := groupFrames(frames, 3)
	if len(groups) != 4 {
		t.Fatalf("groups = %d, want 4 (3+3+3+1)", len(groups))
	}
	for i, g := range groups {
		want := 3
		if i == len(groups)-1 {
			want = 1
		}
		if len(g) != want {
			t.Fatalf("group %d = %d frames, want %d", i, len(g), want)
		}
	}

	// 0 and out-of-range caps fall back to the ai.MaxImages default (5), so
	// 10 frames split into 5+5.
	if got := groupFrames(frames, 0); len(got) != 2 {
		t.Fatalf("default cap produced %d groups, want 2 (5+5)", len(got))
	}
	if got := groupFrames(frames, -1); len(got) != 2 {
		t.Fatalf("negative cap produced %d groups, want 2 (5+5)", len(got))
	}
}

// The off-by-one-interval rule (docs/04 §4.3.1): 90 frames at 10s span 890s,
// which is below the 900s target, so the latest batch is NOT processed even
// though it "looks complete". 91 frames span 900s exactly and qualify.
func TestSplitFramesOffByOneInterval(t *testing.T) {
	result := SplitFrames(framesAt(base, 90, 10*time.Second, nil))
	if len(result.Closed) != 0 {
		t.Fatalf("closed = %d for a 890s run, want 0 (span below target)", len(result.Closed))
	}
	if len(result.Latest.Frames) != 90 {
		t.Fatalf("latest = %d frames, want 90", len(result.Latest.Frames))
	}

	// 91 frames: span reaches 900s exactly; the 900s boundary seals the
	// first 91-frame batch… but the seal happens when ADDING a frame beyond
	// the target, so a run whose span exactly equals the target stays latest
	// until the next frame arrives. Span == 900s meets "reached target" for
	// the caller's decision.
	result = SplitFrames(framesAt(base, 91, 10*time.Second, nil))
	if len(result.Latest.Frames) != 91 || result.Latest.Span() != 900*time.Second {
		t.Fatalf("latest = %d frames / %v span, want 91 / 15m", len(result.Latest.Frames), result.Latest.Span())
	}
	// The caller-side rule: latest qualifies once span >= target.
	if result.Latest.Span() < TargetBatchDuration {
		t.Fatal("900s span reported below target")
	}

	// 92 frames: the 92nd frame sees the accumulated span >= target and seals.
	result = SplitFrames(framesAt(base, 92, 10*time.Second, nil))
	if len(result.Closed) != 1 || len(result.Closed[0].Frames) != 91 {
		t.Fatalf("closed = %d batches, first has %d frames, want 1 batch of 91",
			len(result.Closed), len(result.Closed[0].Frames))
	}
	if len(result.Latest.Frames) != 1 {
		t.Fatalf("latest = %d frames, want 1", len(result.Latest.Frames))
	}
}

// A gap larger than MaxSplitGap seals a batch regardless of duration.
func TestSplitFramesSealsOnGap(t *testing.T) {
	frames := framesAt(base, 30, 10*time.Second, nil)
	// 30 frames = 290s; then a 3-minute gap, then more frames.
	after := base.Add(290*time.Second + 3*time.Minute)
	frames = append(frames, framesAt(after, 10, 10*time.Second, nil)...)

	result := SplitFrames(frames)
	if len(result.Closed) != 1 {
		t.Fatalf("closed = %d, want 1 (gap-sealed)", len(result.Closed))
	}
	if len(result.Closed[0].Frames) != 30 {
		t.Fatalf("gap-sealed batch has %d frames, want 30", len(result.Closed[0].Frames))
	}
	if len(result.Latest.Frames) != 10 {
		t.Fatalf("latest = %d frames, want 10", len(result.Latest.Frames))
	}
}

// A gap exactly at MaxSplitGap does NOT seal (the rule is "larger than").
func TestSplitFramesBoundaryGapDoesNotSeal(t *testing.T) {
	frames := framesAt(base, 30, 10*time.Second, nil)
	after := base.Add(290*time.Second + 2*time.Minute)
	frames = append(frames, framesAt(after, 5, 10*time.Second, nil)...)

	result := SplitFrames(frames)
	if len(result.Closed) != 0 {
		t.Fatalf("closed = %d at exactly MaxSplitGap, want 0", len(result.Closed))
	}
}

// A closed batch below MinAnalysisDuration is the caller's signal to mark it
// skipped_short; the boundary is exact (300s qualifies, 290s does not).
func TestMinAnalysisDurationBoundary(t *testing.T) {
	// 290s gap-sealed run: 30 frames at 10s.
	short := framesAt(base, 30, 10*time.Second, nil)
	after := base.Add(290*time.Second + 3*time.Minute)
	short = append(short, framesAt(after, 5, 10*time.Second, nil)...)

	result := SplitFrames(short)
	if got := result.Closed[0].Span(); got >= MinAnalysisDuration {
		t.Fatalf("290s batch qualifies for analysis, want below the 5m minimum")
	}

	// 300s: 31 frames at 10s.
	exact := framesAt(base, 31, 10*time.Second, nil)
	after = base.Add(300*time.Second + 3*time.Minute)
	exact = append(exact, framesAt(after, 5, 10*time.Second, nil)...)
	result = SplitFrames(exact)
	if got := result.Closed[0].Span(); got != 300*time.Second || got < MinAnalysisDuration {
		t.Fatalf("300s batch span = %v, want exactly 5m (qualifies)", got)
	}
}

// DetectIdle: a batch whose every frame reports long idle is idle; frames
// with small idle readings are not.
func TestDetectIdleTrueAndFalse(t *testing.T) {
	rules := DefaultIdleRules()

	// 90 frames at 10s over 15 minutes, each reporting 120s idle: the idle
	// coverage intervals overlap and blanket the span.
	idle := framesAt(base, 90, 10*time.Second, func(int) *int { return intPtr(120) })
	if !DetectIdle(idle, rules) {
		t.Fatal("fully idle batch reported active")
	}

	// Active frames: idle 0 everywhere.
	active := framesAt(base, 90, 10*time.Second, func(int) *int { return intPtr(0) })
	if DetectIdle(active, rules) {
		t.Fatal("active batch reported idle")
	}
}

// A batch shorter than MinBatchDuration is never idle — not enough evidence.
func TestDetectIdleRequiresMinimumDuration(t *testing.T) {
	rules := DefaultIdleRules()
	// 11:59 of idle frames.
	short := framesAt(base, 72, 10*time.Second, func(int) *int { return intPtr(600) })
	if DetectIdle(short, rules) {
		t.Fatal("11m50s batch judged idle below the 12m minimum")
	}
	// 12:00 exactly.
	exact := framesAt(base, 73, 10*time.Second, func(int) *int { return intPtr(600) })
	if !DetectIdle(exact, rules) {
		t.Fatal("12m batch with full coverage not judged idle")
	}
}

// NULL idle samples count against availability: more than 10% missing
// refuses the idle verdict.
func TestDetectIdleSampleAvailability(t *testing.T) {
	rules := DefaultIdleRules()
	// 90 frames, 10 with no sample: 80/90 = 0.889 < 0.90.
	withHoles := framesAt(base, 90, 10*time.Second, func(i int) *int {
		if i%9 == 0 {
			return nil
		}
		return intPtr(120)
	})
	if DetectIdle(withHoles, rules) {
		t.Fatal("batch with 11% missing samples judged idle")
	}
	// 9 missing of 90 = 0.90 exactly, allowed.
	withFewHoles := framesAt(base, 90, 10*time.Second, func(i int) *int {
		if i < 9 {
			return nil
		}
		return intPtr(120)
	})
	if !DetectIdle(withFewHoles, rules) {
		t.Fatal("batch with 10% missing samples refused idle")
	}
}

// The qualified-frame ratio: a frame counts as idle only at >= 60s of idle.
func TestDetectIdleQualifiedFrameThreshold(t *testing.T) {
	rules := DefaultIdleRules()
	// Half the frames read 59s (just below threshold), half 120s: qualified
	// ratio 0.5 < 0.90.
	mixed := framesAt(base, 90, 10*time.Second, func(i int) *int {
		if i%2 == 0 {
			return intPtr(59)
		}
		return intPtr(120)
	})
	if DetectIdle(mixed, rules) {
		t.Fatal("batch with 50% qualified frames judged idle")
	}
}

// A hole in the coverage larger than MaxUncoveredGap breaks the idle verdict.
func TestDetectIdleUncoveredGap(t *testing.T) {
	rules := DefaultIdleRules()
	// Frames report just enough idle (60s) that coverage is a chain of
	// 60s-wide intervals every 10s — full coverage. Then punch a hole:
	// frames 40..44 report 0 idle, leaving a ~50s gap (frames 39 and 45
	// cover [t39-60, t39] and [t45-60, t45]; between t39 and t45-60 = t45-60
	// … the hole is (t39, t45-60s) = 40s+ gap if frames are 10s apart:
	// t45 - 60 - t39 = (450-60) - 390 = 0? Compute concretely below instead.
	frames := framesAt(base, 90, 10*time.Second, func(i int) *int {
		if i >= 40 && i <= 44 {
			return intPtr(0)
		}
		return intPtr(60)
	})
	// Frames 40-44 cover nothing. Frame 39 covers [390-60, 390] = [330,390].
	// Frame 45 covers [450-60, 450] = [390,450]. No hole at all — the 60s
	// readings reach across the 5 dead frames. So this IS idle; the test
	// asserts coverage semantics rather than inventing a fake hole.
	if !DetectIdle(frames, rules) {
		t.Fatal("60s idle readings every 10s should blanket a 5-frame dead stretch")
	}

	// A real hole: dead frames whose neighbors' coverage cannot bridge.
	// With 60s readings bridging 50s, kill 10 frames: neighbors cover
	// [t39, t49-60] = [390, 430]; hole = 40s > 30s.
	frames = framesAt(base, 90, 10*time.Second, func(i int) *int {
		if i >= 40 && i <= 48 {
			return intPtr(0)
		}
		return intPtr(60)
	})
	if DetectIdle(frames, rules) {
		t.Fatal("40s coverage hole accepted; max is 30s")
	}
}

// Coverage ratio: 0.949 refuses, and the tail hole beyond the last covered
// point counts as an uncovered gap.
func TestDetectIdleCoverageRatio(t *testing.T) {
	rules := DefaultIdleRules()
	// The last frame reads 0: the tail [t89, t89] is uncovered but the
	// preceding coverage reaches t89 (frame 88 covers [880-60, 880], frame 89
	// is at 890 — hole of 10s at the tail, within MaxUncoveredGap). This is
	// still idle; the ratio case needs a bigger sacrifice.
	lastActive := framesAt(base, 90, 10*time.Second, func(i int) *int {
		if i >= 88 {
			return intPtr(0)
		}
		return intPtr(60)
	})
	// Coverage ends at t87=880; span is [0,890]; hole = 10s <= 30s, and
	// coverage = 880/890 = 0.989 >= 0.95.
	if !DetectIdle(lastActive, rules) {
		t.Fatal("idle batch with a small tail hole refused")
	}
	_ = rules
}

// truncate must never split a multi-byte rune: a mid-rune cut corrupts
// failure notes in every non-ASCII language.
func TestTruncateKeepsRunesWhole(t *testing.T) {
	long := strings.Repeat("活动记录", 200) // 4 bytes per rune
	got := truncate(long, 200)
	if got == "" {
		t.Fatal("truncate returned empty")
	}
	if !utf8.ValidString(got) {
		t.Fatalf("truncated string is not valid UTF-8: %q", got[len(got)-8:])
	}
	if r, _ := utf8.DecodeLastRuneInString(got); r == utf8.RuneError {
		t.Fatal("truncated string ends in a broken rune")
	}
	// A short string passes through untouched.
	if got := truncate("活动", 200); got != "活动" {
		t.Fatalf("short string altered: %q", got)
	}
}

// appsOfMetadata feeds the card prompt with the transcription stage's apps
// list; absent or malformed metadata yields nothing rather than an error.
func TestAppsOfMetadata(t *testing.T) {
	apps := appsOfMetadata(`{"apps":["Safari","Xcode"]}`)
	if len(apps) != 2 || apps[0] != "Safari" || apps[1] != "Xcode" {
		t.Fatalf("apps = %v", apps)
	}
	if got := appsOfMetadata(""); got != nil {
		t.Fatalf("empty metadata apps = %v", got)
	}
	if got := appsOfMetadata("not json"); got != nil {
		t.Fatalf("malformed metadata apps = %v", got)
	}
	if got := appsOfMetadata(`{"other":1}`); got != nil {
		t.Fatalf("apps absent = %v", got)
	}
}

// The model-facing category list never offers the built-ins: System is the
// unknown-category fallback and Idle belongs to the hardware idle fast path,
// so neither may be chosen from screen content (docs/04 §4.3.4, §4.4).
func TestCardsPromptExcludesBuiltInCategories(t *testing.T) {
	categories := []domain.Category{
		{ID: "1", Name: "System", IsSystem: true},
		{ID: "2", Name: "Idle", IsSystem: true, IsIdle: true},
		{ID: "3", Name: "Coding", Details: "writing code"},
	}
	prompt := cardsPrompt(base, base.Add(15*time.Minute), nil, nil, categories, "", cardModeOngoing)

	if !strings.Contains(prompt, "\n  Coding — writing code\n") {
		t.Fatalf("prompt missing user category:\n%s", prompt)
	}
	for _, builtIn := range []string{"\n  System", "\n  Idle"} {
		if strings.Contains(prompt, builtIn) {
			t.Fatalf("prompt offers built-in category %q:\n%s", builtIn, prompt)
		}
	}
}

// The 15-minute floor (2026-09-21) replaced the 09-20 rule that a short
// evidence-backed episode stands alone as its own card. A card under the floor
// is now merged into a neighbor — across a category boundary when necessary —
// and only the window's last card may be shorter, because the supplied evidence
// stops there. Both prompts flip together with the validator.
func TestOngoingCardRulesEnforceTheFifteenMinuteFloor(t *testing.T) {
	prompt := cardsPrompt(base, base.Add(15*time.Minute), nil, nil, nil, "", cardModeOngoing)
	for _, want := range []string{
		"must be 15 to 60 minutes",
		"fold it into the neighboring activity",
		"Only the last card of the window may fall short of 15",
		"takes the category of whichever activity",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("ongoing prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "Keep a brief episode as its own card") ||
		strings.Contains(prompt, "never borrow unrelated neighboring minutes") {
		t.Fatalf("ongoing prompt still carries the withdrawn no-minimum rule:\n%s", prompt)
	}

	correction := cardsCorrectionPrompt(`{"cards":[]}`, []string{"example"}, cardModeOngoing, base, base.Add(15*time.Minute))
	if !strings.Contains(correction, "Every card must be 15 to 60 minutes") ||
		!strings.Contains(correction, "except the last one must be 15 minutes or longer") {
		t.Fatalf("correction prompt missing the floor:\n%s", correction)
	}
	if strings.Contains(correction, "A short card is valid") ||
		strings.Contains(correction, "merely to satisfy a duration preference") {
		t.Fatalf("correction prompt still refuses the floor's merge:\n%s", correction)
	}
}

func TestResolveCardSpansSurfacesDegenerateShells(t *testing.T) {
	loc := time.Local
	window := base.Add(time.Hour)
	shells := []domain.CardShell{
		{Start: "10:00 AM", End: "10:30 AM", Title: "good"},
		{Start: "10:30 AM", End: "10:29 AM", Title: "inverted"},
		{Start: "half past", End: "10:45 AM", Title: "unparseable"},
	}
	spans, issues := resolveCardSpans(shells, base, window, loc)
	if len(spans) != 1 || spans[0].Title != "good" {
		t.Fatalf("spans = %+v, want only the good card", spans)
	}
	if len(issues) != 2 {
		t.Fatalf("issues = %v, want one for the inverted card and one for the unparseable card", issues)
	}
	if !strings.Contains(issues[0], "card 2 (inverted)") || !strings.Contains(issues[0], "must end after it starts") {
		t.Fatalf("issue[0] = %q, want the inverted-card violation", issues[0])
	}
	if !strings.Contains(issues[1], "card 3 (unparseable)") || !strings.Contains(issues[1], "unparseable start") {
		t.Fatalf("issue[1] = %q, want the unparseable-start violation", issues[1])
	}
}

func TestValidateCardsRejectsShortCardsExceptTheLastOne(t *testing.T) {
	spans := []cardSpan{
		{Start: base, End: base.Add(2 * time.Minute), Title: "video"},
		{Start: base.Add(2 * time.Minute), End: base.Add(6 * time.Minute), Title: "Daygo"},
		{Start: base.Add(6 * time.Minute), End: base.Add(10 * time.Minute), Title: "video"},
		{Start: base.Add(10 * time.Minute), End: base.Add(13 * time.Minute), Title: "Codex"},
	}
	issues := validateCards(spans, base, base.Add(13*time.Minute), false)
	if len(issues) != 3 {
		t.Fatalf("issues = %v, want one per card under the floor that has a successor", issues)
	}
	for i, want := range []string{"card 1 (video)", "card 2 (Daygo)", "card 3 (video)"} {
		if !strings.Contains(issues[i], want) || !strings.Contains(issues[i], "merge it into a neighboring card") {
			t.Fatalf("issue %d = %q, want the floor violation for %q", i, issues[i], want)
		}
	}

	// The card carrying the window's end is exempt: no evidence inside the
	// rewrite follows it, and the next sliding-window pass owns what does.
	tail := []cardSpan{
		{Start: base, End: base.Add(50 * time.Minute), Title: "long"},
		{Start: base.Add(50 * time.Minute), End: base.Add(63 * time.Minute), Title: "tail"},
	}
	if issues := validateCards(tail, base, base.Add(63*time.Minute), false); len(issues) != 0 {
		t.Fatalf("last card under the floor rejected: %v", issues)
	}

	// The floor and the 60-minute cap hold together: covering a 65-minute span
	// with two compliant cards leaves the tail short by construction.
	capped := []cardSpan{
		{Start: base, End: base.Add(60 * time.Minute), Title: "head"},
		{Start: base.Add(60 * time.Minute), End: base.Add(65 * time.Minute), Title: "tail"},
	}
	if issues := validateCards(capped, base, base.Add(65*time.Minute), false); len(issues) != 0 {
		t.Fatalf("60+5 split rejected: %v", issues)
	}

	// A fresh segment's single card is the last card by definition, so a batch
	// sealed below the floor still produces its one card.
	fresh := []cardSpan{{Start: base, End: base.Add(6 * time.Minute), Title: "short batch"}}
	if issues := validateCards(fresh, base, base.Add(6*time.Minute), true); len(issues) != 0 {
		t.Fatalf("short fresh batch rejected: %v", issues)
	}
}

func TestAppSitesFromListSwapsBrowserAndTarget(t *testing.T) {
	// When the model outputs browser first, then website, it should swap so the website is primary.
	swapped := appSitesFromList([]string{"Microsoft Edge", "pinterest.com"})
	if swapped == nil || *swapped.Primary != "pinterest.com" || *swapped.Secondary != "Microsoft Edge" {
		t.Fatalf("expected swapped primary pinterest.com and secondary Microsoft Edge, got %+v", swapped)
	}

	// Normal non-browser website first stays untouched.
	normal := appSitesFromList([]string{"bilibili.com", "Google Chrome"})
	if normal == nil || *normal.Primary != "bilibili.com" || *normal.Secondary != "Google Chrome" {
		t.Fatalf("expected normal primary bilibili.com and secondary Google Chrome, got %+v", normal)
	}

	// Single browser stays primary.
	single := appSitesFromList([]string{"Safari"})
	if single == nil || *single.Primary != "Safari" || single.Secondary != nil {
		t.Fatalf("expected single primary Safari, got %+v", single)
	}
}

func TestDistractionsFromModelMapsTheClockRangeOntoTheContract(t *testing.T) {
	// The model says start/end, like a card window; metadata says
	// startTime/endTime, like the DTO the inspector reads (docs/05 §5.5.2).
	mapped := distractionsFromModel([]cardsDistraction{
		{Start: " 10:05 AM ", End: "10:07 AM", Title: " checked a feed ", Summary: ""},
	})
	if len(mapped) != 1 {
		t.Fatalf("mapped = %+v, want one entry", mapped)
	}
	want := distractionMetadata{StartTime: "10:05 AM", EndTime: "10:07 AM", Title: "checked a feed"}
	if mapped[0] != want {
		t.Fatalf("mapped[0] = %+v, want %+v", mapped[0], want)
	}

	// An interruption the model could not name would render as a bare clock
	// range in the inspector, so it is dropped rather than stored empty.
	untitled := distractionsFromModel([]cardsDistraction{{Start: "10:20 AM", End: "10:21 AM", Title: "  "}})
	if len(untitled) != 0 {
		t.Fatalf("untitled = %+v, want the entry dropped", untitled)
	}

	// An empty list stores as [], never null: the wire contract declares arrays
	// and consumers call .length on them.
	empty := distractionsFromModel(nil)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty = %#v, want a non-nil empty slice", empty)
	}
}

func TestEvenlySpacedIndices(t *testing.T) {
	// 0 or negative counts return nil
	if indices := evenlySpacedIndices(0, 15); indices != nil {
		t.Fatalf("expected nil for itemCount 0, got %v", indices)
	}
	if indices := evenlySpacedIndices(10, 0); indices != nil {
		t.Fatalf("expected nil for maxCount 0, got %v", indices)
	}

	// Fewer than or equal to maxCount returns all indices
	indices10 := evenlySpacedIndices(10, 15)
	if len(indices10) != 10 {
		t.Fatalf("expected 10 indices, got %d", len(indices10))
	}
	for i, idx := range indices10 {
		if idx != i {
			t.Fatalf("expected index %d to be %d, got %d", i, i, idx)
		}
	}

	// 90 frames downsampled to 15 (typical 15-minute batch, matching Dayflow)
	indices90 := evenlySpacedIndices(90, 15)
	if len(indices90) != 15 {
		t.Fatalf("expected 15 indices, got %d", len(indices90))
	}
	if indices90[0] != 0 {
		t.Fatalf("first index must be 0, got %d", indices90[0])
	}
	if indices90[14] != 89 {
		t.Fatalf("last index must be 89, got %d", indices90[14])
	}
	// Strictly increasing
	for i := 1; i < len(indices90); i++ {
		if indices90[i] <= indices90[i-1] {
			t.Fatalf("indices not strictly increasing: [%d]=%d <= [%d]=%d",
				i, indices90[i], i-1, indices90[i-1])
		}
	}
}

func TestSampleFrames(t *testing.T) {
	frames := framesAt(base, 90, 10*time.Second, nil)
	sampled := sampleFrames(frames, 15)
	if len(sampled) != 15 {
		t.Fatalf("expected 15 sampled frames, got %d", len(sampled))
	}
	if !sampled[0].CapturedAt.Equal(frames[0].CapturedAt) {
		t.Fatalf("first sampled frame timestamp mismatch: %v vs %v",
			sampled[0].CapturedAt, frames[0].CapturedAt)
	}
	if !sampled[14].CapturedAt.Equal(frames[89].CapturedAt) {
		t.Fatalf("last sampled frame timestamp mismatch: %v vs %v",
			sampled[14].CapturedAt, frames[89].CapturedAt)
	}
}

func TestIsRateLimitError(t *testing.T) {
	if isRateLimitError(nil) {
		t.Fatal("nil error should not be rate limit")
	}
	if !isRateLimitError(ai.NewError(ai.ErrorRateLimited, "too many requests", 429, nil)) {
		t.Fatal("ErrorRateLimited should be recognized as rate limit")
	}
	if !isRateLimitError(errors.New("HTTP 429: rate limit exceeded, please retry later")) {
		t.Fatal("string containing rate limit should be recognized")
	}
	if !isRateLimitError(errors.New("exceeded your current quota, please check plan")) {
		t.Fatal("string containing quota should be recognized")
	}
	if !isRateLimitError(errors.New("TPM limit reached: tokens per minute")) {
		t.Fatal("string containing tpm/tokens per minute should be recognized")
	}
	if isRateLimitError(errors.New("context deadline exceeded")) {
		t.Fatal("timeout error should not be rate limit")
	}
}
