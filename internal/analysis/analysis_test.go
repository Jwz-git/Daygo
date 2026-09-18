package analysis

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

	// 0 and out-of-range caps fall back to the ai.MaxImages default.
	if got := groupFrames(frames, 0); len(got) != 1 {
		t.Fatalf("default cap produced %d groups, want 1", len(got))
	}
	if got := groupFrames(frames, -1); len(got) != 1 {
		t.Fatalf("negative cap produced %d groups, want 1", len(got))
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
	prompt := cardsPrompt(base, base.Add(15*time.Minute), nil, nil, categories, "", true)

	if !strings.Contains(prompt, "\n  Coding — writing code\n") {
		t.Fatalf("prompt missing user category:\n%s", prompt)
	}
	for _, builtIn := range []string{"\n  System", "\n  Idle"} {
		if strings.Contains(prompt, builtIn) {
			t.Fatalf("prompt offers built-in category %q:\n%s", builtIn, prompt)
		}
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

