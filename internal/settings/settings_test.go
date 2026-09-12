package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// fakeRepo is an in-memory stand-in for the storage repository. Its write method
// can be made to fail so the error path is testable without a database.
type fakeRepo struct {
	values    map[string]string
	setErr    error
	getErr    error
	getAllErr error
	lastBatch map[string]string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{values: map[string]string{}}
}

func (f *fakeRepo) Get(_ context.Context, key string) (string, bool, error) {
	if f.getErr != nil {
		return "", false, f.getErr
	}
	value, ok := f.values[key]
	return value, ok, nil
}

func (f *fakeRepo) GetAll(context.Context) (map[string]string, error) {
	if f.getAllErr != nil {
		return nil, f.getAllErr
	}
	out := make(map[string]string, len(f.values))
	for key, value := range f.values {
		out[key] = value
	}
	return out, nil
}

func (f *fakeRepo) Set(_ context.Context, key, value string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.values[key] = value
	return nil
}

func (f *fakeRepo) SetMany(_ context.Context, values map[string]string) error {
	if f.setErr != nil {
		// A failed batch must leave nothing behind, matching the real
		// repository's single-transaction behavior.
		return f.setErr
	}
	f.lastBatch = make(map[string]string, len(values))
	for key, value := range values {
		f.values[key] = value
		f.lastBatch[key] = value
	}
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, key string) error {
	delete(f.values, key)
	return nil
}

func ptr[T any](value T) *T { return &value }

// An empty database yields every documented default (docs/03 §3.3.5).
func TestLoadDefaultsOnEmptyDatabase(t *testing.T) {
	s := New(newFakeRepo())

	got, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Snapshot{
		CaptureIntervalSeconds: DefaultCaptureIntervalSeconds,
		CaptureHeightPixels:    DefaultCaptureHeightPixels,
		BlockedApplicationIDs:  []string{},
		RecordingsLimitBytes:   DefaultRecordingsLimitBytes,
		ReminderEnabled:        DefaultReminderEnabled,
		ReminderTime:           DefaultReminderTime,
		Theme:                  DefaultTheme,
		Language:               DefaultLanguage,
		LaunchAtLogin:          DefaultLaunchAtLogin,
		ShowDockIcon:           DefaultShowDockIcon,
		AgentEditsEnabled:      DefaultAgentEditsEnabled,
		AnalyticsOptIn:         DefaultAnalyticsOptIn,
		CrashReportingOptIn:    DefaultCrashReportingOptIn,
		ProvidersRouting:       Routing{Chain: []string{}},
		OutputLanguage:         DefaultOutputLanguage,
		RecognitionEnhancement: DefaultRecognitionEnhancement,
		ChatMemory:             DefaultChatMemory,
		ChatEditMode:           DefaultChatEditMode,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaults mismatch:\ngot  %+v\nwant %+v", got, want)
	}
}

// The key list is the contract; a key added here without a matching default in
// defaultFor would silently read back as an empty string.
func TestEveryKeyHasADefault(t *testing.T) {
	for _, key := range AllKeys() {
		if defaultFor(key) == "" {
			t.Errorf("key %q has no default form", key)
		}
	}
}

// A closed value set snaps to a valid member rather than storing garbage that
// every later reader would have to defend against.
func TestApplyClampsClosedValueSets(t *testing.T) {
	cases := []struct {
		name      string
		patch     Patch
		wantValue any
		key       string
	}{
		{"interval below set", Patch{IntervalSeconds: ptr(7)}, DefaultCaptureIntervalSeconds, KeyCaptureIntervalSeconds},
		{"interval at zero", Patch{IntervalSeconds: ptr(0)}, DefaultCaptureIntervalSeconds, KeyCaptureIntervalSeconds},
		{"interval negative", Patch{IntervalSeconds: ptr(-3)}, DefaultCaptureIntervalSeconds, KeyCaptureIntervalSeconds},
		{"interval valid", Patch{IntervalSeconds: ptr(30)}, 30, KeyCaptureIntervalSeconds},
		{"height invalid", Patch{CaptureHeight: ptr(1440)}, DefaultCaptureHeightPixels, KeyCaptureHeightPixels},
		{"height valid", Patch{CaptureHeight: ptr(720)}, 720, KeyCaptureHeightPixels},
		{"theme invalid", Patch{Theme: ptr("neon")}, DefaultTheme, KeyAppearanceTheme},
		{"theme valid", Patch{Theme: ptr("dark")}, "dark", KeyAppearanceTheme},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			s := New(repo)
			if _, _, err := s.Apply(context.Background(), tc.patch); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			var got any
			switch tc.wantValue.(type) {
			case int:
				got = decodeInt(repo.values[tc.key], -1)
			case string:
				got = decodeString(repo.values[tc.key], "")
			}
			if got != tc.wantValue {
				t.Fatalf("stored %v = %v, want %v", tc.key, got, tc.wantValue)
			}
		})
	}
}

// A nil field means "not provided". A patch that omits a key must not touch it.
func TestApplyOnlyWritesProvidedKeys(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo)
	ctx := context.Background()

	// Seed a value the patch will not mention.
	if _, _, err := s.Apply(ctx, Patch{IntervalSeconds: ptr(30)}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, changed, err := s.Apply(ctx, Patch{Theme: ptr("dark")})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(changed) != 1 || changed[0] != KeyAppearanceTheme {
		t.Fatalf("changed = %v, want just the theme key", changed)
	}
	if _, ok := repo.lastBatch[KeyCaptureIntervalSeconds]; ok {
		t.Fatal("a patch that did not mention the capture interval still wrote it")
	}
	if got := decodeInt(repo.values[KeyCaptureIntervalSeconds], -1); got != 30 {
		t.Fatalf("interval = %d after an unrelated patch, want the seeded 30", got)
	}
}

// An empty patch is a no-op rather than an error: a caller that changed nothing
// still succeeds.
func TestApplyEmptyPatchIsNoOp(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo)

	snapshot, changed, err := s.Apply(context.Background(), Patch{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(changed) != 0 {
		t.Fatalf("changed = %v, want empty", changed)
	}
	if len(repo.values) != 0 {
		t.Fatalf("empty patch wrote %v", repo.values)
	}
	if snapshot.Theme != DefaultTheme {
		t.Fatalf("theme = %q, want the default", snapshot.Theme)
	}
}

// The returned snapshot is the post-normalization state, which is what makes
// the frontend able to display the effective value without predicting it.
func TestApplyReturnsNormalizedSnapshot(t *testing.T) {
	s := New(newFakeRepo())

	snapshot, _, err := s.Apply(context.Background(), Patch{IntervalSeconds: ptr(999)})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if snapshot.CaptureIntervalSeconds != DefaultCaptureIntervalSeconds {
		t.Fatalf("returned interval = %d, want the clamped default %d",
			snapshot.CaptureIntervalSeconds, DefaultCaptureIntervalSeconds)
	}
}

// A write failure must surface and must not report a changed key set.
func TestApplyReportsWriteFailure(t *testing.T) {
	repo := newFakeRepo()
	cause := errors.New("disk on fire")
	repo.setErr = cause
	s := New(repo)

	_, changed, err := s.Apply(context.Background(), Patch{Theme: ptr("dark")})
	if err == nil {
		t.Fatal("Apply succeeded despite a write failure")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("error %v does not wrap the cause", err)
	}
	if len(changed) != 0 {
		t.Fatalf("changed = %v after a failed write", changed)
	}
}

// A failed write must not leave a partial group behind.
func TestApplyFailureLeavesNothingWritten(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo)
	ctx := context.Background()

	if _, _, err := s.Apply(ctx, Patch{Theme: ptr("dark")}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	before := repo.values[KeyAppearanceTheme]

	repo.setErr = errors.New("write refused")
	_, _, err := s.Apply(ctx, Patch{Theme: ptr("light"), IntervalSeconds: ptr(30)})
	if err == nil {
		t.Fatal("Apply succeeded despite a write failure")
	}
	if repo.values[KeyAppearanceTheme] != before {
		t.Fatal("a failed patch changed the theme")
	}
}

// Normalization runs on read too: a value stored by an older build or edited
// outside the app must not reach callers unvalidated.
func TestLoadNormalizesStoredValues(t *testing.T) {
	repo := newFakeRepo()
	repo.values[KeyCaptureIntervalSeconds] = "7"        // not in the closed set
	repo.values[KeyAppearanceTheme] = `"neon"`          // not a known theme
	repo.values[KeyAppearanceLanguage] = `"zh-Hans-CN"` // folds to zh-CN
	repo.values[KeyNotificationsReminderTime] = `"9:5"` // loose but unambiguous

	snapshot, err := New(repo).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snapshot.CaptureIntervalSeconds != DefaultCaptureIntervalSeconds {
		t.Errorf("interval = %d, want the default", snapshot.CaptureIntervalSeconds)
	}
	if snapshot.Theme != DefaultTheme {
		t.Errorf("theme = %q, want the default", snapshot.Theme)
	}
	if snapshot.Language != "zh-CN" {
		t.Errorf("language = %q, want %q", snapshot.Language, "zh-CN")
	}
	if snapshot.ReminderTime != "09:05" {
		t.Errorf("reminder time = %q, want %q", snapshot.ReminderTime, "09:05")
	}
}

// Malformed stored JSON must fall back to the default rather than failing the
// whole load: one bad row should not make the settings screen unusable.
func TestLoadToleratesMalformedStoredJSON(t *testing.T) {
	repo := newFakeRepo()
	repo.values[KeyCaptureIntervalSeconds] = `{"not":"an int"}`
	repo.values[KeySystemShowDockIcon] = `not json at all`
	repo.values[KeyPrivacyBlockedApplicationIDs] = `[1,2,3]`

	snapshot, err := New(repo).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snapshot.CaptureIntervalSeconds != DefaultCaptureIntervalSeconds {
		t.Errorf("interval = %d, want the default", snapshot.CaptureIntervalSeconds)
	}
	if snapshot.ShowDockIcon != DefaultShowDockIcon {
		t.Errorf("showDockIcon = %v, want the default", snapshot.ShowDockIcon)
	}
	if len(snapshot.BlockedApplicationIDs) != 0 {
		t.Errorf("blocked IDs = %v, want empty for a wrong-typed value", snapshot.BlockedApplicationIDs)
	}
}

// The empty language string is the one documented sentinel: "follow the
// system". It must survive normalization rather than being replaced.
func TestEmptyLanguageSentinelIsPreserved(t *testing.T) {
	s := New(newFakeRepo())
	ctx := context.Background()

	snapshot, _, err := s.Apply(ctx, Patch{Language: ptr("")})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if snapshot.Language != "" {
		t.Fatalf("language = %q, want the empty follow-the-system sentinel", snapshot.Language)
	}

	// And it must not be confused with an unknown language.
	unknown, _, err := s.Apply(ctx, Patch{Language: ptr("klingon")})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if unknown.Language == "" {
		t.Fatal("an unsupported language normalized to the follow-the-system sentinel")
	}
	if unknown.Language != DefaultLanguage {
		t.Fatalf("language = %q, want the default %q", unknown.Language, DefaultLanguage)
	}
}

// outputLanguage is independent of the interface language and has its own empty
// default. Conflating the two would make the card language follow the UI.
func TestOutputLanguageIsIndependent(t *testing.T) {
	s := New(newFakeRepo())

	snapshot, _, err := s.Apply(context.Background(), Patch{Language: ptr("en")})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if snapshot.Language != "en" {
		t.Fatalf("language = %q, want %q", snapshot.Language, "en")
	}
	if snapshot.OutputLanguage != "" {
		t.Fatalf("outputLanguage = %q, want it untouched by the interface language",
			snapshot.OutputLanguage)
	}
}

func TestRecognitionEnhancementIsIndependentAndPersistent(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo)

	snapshot, changed, err := s.Apply(context.Background(), Patch{RecognitionEnhancement: ptr(true)})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !snapshot.RecognitionEnhancement {
		t.Fatal("recognition enhancement remained disabled")
	}
	if len(changed) != 1 || changed[0] != KeyLLMRecognitionEnhancement {
		t.Fatalf("changed = %v, want recognition enhancement key", changed)
	}
	if repo.values[KeyLLMRecognitionEnhancement] != "true" {
		t.Fatalf("stored value = %q, want true", repo.values[KeyLLMRecognitionEnhancement])
	}

	reloaded, err := New(repo).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reloaded.RecognitionEnhancement {
		t.Fatal("recognition enhancement was not read back")
	}
}

// Blocked application IDs are deduplicated and emptied of blanks, so the stored
// list is stable enough to compare.
func TestBlockedApplicationsAreNormalized(t *testing.T) {
	s := New(newFakeRepo())

	snapshot, _, err := s.Apply(context.Background(), Patch{
		BlockedApplicationIDs: ptr([]string{"com.b", " com.a ", "com.b", "", "  "}),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := []string{"com.b", "com.a"}
	if !reflect.DeepEqual(snapshot.BlockedApplicationIDs, want) {
		t.Fatalf("blocked IDs = %v, want %v", snapshot.BlockedApplicationIDs, want)
	}
}

// Clearing the blocked list must store an empty array, not null, so a reader
// never has to distinguish two representations of "nothing blocked".
func TestClearingBlockedApplicationsStoresEmptyArray(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo)
	ctx := context.Background()

	if _, _, err := s.Apply(ctx, Patch{BlockedApplicationIDs: ptr([]string{"com.a"})}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, _, err := s.Apply(ctx, Patch{BlockedApplicationIDs: ptr([]string{})}); err != nil {
		t.Fatalf("clear: %v", err)
	}

	if got := repo.values[KeyPrivacyBlockedApplicationIDs]; got != "[]" {
		t.Fatalf("stored %q, want %q", got, "[]")
	}
}

// A negative byte limit is meaningless; it clamps to "no limit" rather than
// making the setting unusable.
func TestNegativeSizeLimitClampsToUnlimited(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo)

	snapshot, _, err := s.Apply(context.Background(), Patch{RecordingsLimitBytes: ptr(int64(-1))})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if snapshot.RecordingsLimitBytes != DefaultRecordingsLimitBytes {
		t.Fatalf("limit = %d, want %d", snapshot.RecordingsLimitBytes, DefaultRecordingsLimitBytes)
	}
}

// A pre-chain build stored {"primary": "...", "secondary": "..."}; reading it
// must fold into a chain, not drop the routing.
func TestRoutingLegacyShapeFoldsIntoChain(t *testing.T) {
	repo := newFakeRepo()
	repo.values[KeyProvidersRouting] = `{"primary":"p1","secondary":"p2"}`

	snapshot, err := New(repo).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"p1", "p2"}
	if !reflect.DeepEqual(snapshot.ProvidersRouting.Chain, want) {
		t.Fatalf("routing chain = %v, want %v", snapshot.ProvidersRouting.Chain, want)
	}
}

// A legacy shape with only a primary folds to a one-entry chain.
func TestRoutingLegacyPrimaryOnly(t *testing.T) {
	repo := newFakeRepo()
	repo.values[KeyProvidersRouting] = `{"primary":"p1","secondary":""}`

	snapshot, err := New(repo).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"p1"}
	if !reflect.DeepEqual(snapshot.ProvidersRouting.Chain, want) {
		t.Fatalf("routing chain = %v, want %v", snapshot.ProvidersRouting.Chain, want)
	}
}

// The chain form round-trips through Load and SetRouting, normalized.
func TestRoutingChainRoundTrips(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo)

	if err := s.SetRouting(context.Background(), Routing{Chain: []string{"p2", "p1", "p2", ""}}); err != nil {
		t.Fatalf("SetRouting: %v", err)
	}
	snapshot, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"p2", "p1"}
	if !reflect.DeepEqual(snapshot.ProvidersRouting.Chain, want) {
		t.Fatalf("routing chain = %v, want %v (deduped, empties dropped)", snapshot.ProvidersRouting.Chain, want)
	}
}

func TestRoutingChainCapsAtEight(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo)

	chain := make([]string, 12)
	for i := range chain {
		chain[i] = fmt.Sprintf("p%d", i)
	}
	if err := s.SetRouting(context.Background(), Routing{Chain: chain}); err != nil {
		t.Fatalf("SetRouting: %v", err)
	}
	snapshot, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(snapshot.ProvidersRouting.Chain) != MaxRoutingChain {
		t.Fatalf("chain length = %d, want %d", len(snapshot.ProvidersRouting.Chain), MaxRoutingChain)
	}
}

// Unparseable routing degrades to an empty chain, never to a partial one.
func TestRoutingUnparseableDegradesToEmpty(t *testing.T) {
	repo := newFakeRepo()
	repo.values[KeyProvidersRouting] = `not json`

	snapshot, err := New(repo).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(snapshot.ProvidersRouting.Chain) != 0 {
		t.Fatalf("chain = %v, want empty", snapshot.ProvidersRouting.Chain)
	}
}

// chat.memory is user-authored free text: round-trip must preserve it, only
// trimming trailing whitespace.
func TestChatMemoryRoundTrips(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo)

	memory := "回答保持简洁。\n关注时间跟踪场景。  \n"
	if _, _, err := s.Apply(context.Background(), Patch{ChatMemory: ptr(memory)}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	snapshot, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := "回答保持简洁。\n关注时间跟踪场景。"; snapshot.ChatMemory != want {
		t.Fatalf("chat.memory = %q, want %q", snapshot.ChatMemory, want)
	}
}

// chat.editMode is a closed set: the two valid values round-trip and anything
// else — written by a patch or found already stored — reads as readonly, the
// safe side of the sandbox gate.
func TestChatEditModeNormalization(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo)

	for _, mode := range []string{ChatEditModeReadonly, ChatEditModeEdits} {
		if _, _, err := s.Apply(context.Background(), Patch{ChatEditMode: ptr(mode)}); err != nil {
			t.Fatalf("Apply %s: %v", mode, err)
		}
		snapshot, err := s.Load(context.Background())
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if snapshot.ChatEditMode != mode {
			t.Fatalf("chat.editMode = %q, want %q", snapshot.ChatEditMode, mode)
		}
	}

	if _, _, err := s.Apply(context.Background(), Patch{ChatEditMode: ptr("yolo")}); err != nil {
		t.Fatalf("Apply invalid mode: %v", err)
	}
	snapshot, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snapshot.ChatEditMode != ChatEditModeReadonly {
		t.Fatalf("chat.editMode = %q after invalid patch, want readonly", snapshot.ChatEditMode)
	}

	// A stray stored value reads as readonly without being rewritten.
	if err := repo.Set(context.Background(), KeyChatEditMode, `"write"`); err != nil {
		t.Fatalf("seed stray value: %v", err)
	}
	got, err := s.Get(context.Background(), KeyChatEditMode)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if want := `"readonly"`; got != want {
		t.Fatalf("Get(chat.editMode) = %s, want %s", got, want)
	}
}

func TestLoadPropagatesRepositoryFailure(t *testing.T) {
	repo := newFakeRepo()
	cause := errors.New("database gone")
	repo.getAllErr = cause

	if _, err := New(repo).Load(context.Background()); !errors.Is(err, cause) {
		t.Fatalf("Load error = %v, want it to wrap the cause", err)
	}
}

// Get applies the same defaulting and normalization as a full load, so a single
// key read never disagrees with the snapshot.
func TestGetAgreesWithLoad(t *testing.T) {
	repo := newFakeRepo()
	repo.values[KeyAppearanceTheme] = `"neon"`
	s := New(repo)

	single, err := s.Get(context.Background(), KeyAppearanceTheme)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	snapshot, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if decodeString(single, "") != snapshot.Theme {
		t.Fatalf("Get returned %q but Load returned %q", decodeString(single, ""), snapshot.Theme)
	}
}

func TestGetUnknownKeyReturnsEmpty(t *testing.T) {
	s := New(newFakeRepo())
	value, err := s.Get(context.Background(), "no.such.key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if value != "" {
		t.Fatalf("value = %q for an unknown key, want empty", value)
	}
}

// Every stored value must be valid JSON, so a raw SQL reader never has to
// interpret a bare word.
func TestStoredValuesAreValidJSON(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo)

	_, _, err := s.Apply(context.Background(), Patch{
		IntervalSeconds:        ptr(30),
		BlockedApplicationIDs:  ptr([]string{"com.a"}),
		JournalReminderEnabled: ptr(true),
		JournalReminderTime:    ptr("07:30"),
		Theme:                  ptr("dark"),
		Language:               ptr("en"),
		ShowDockIcon:           ptr(false),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for key, raw := range repo.values {
		if !json.Valid([]byte(raw)) {
			t.Errorf("key %q stored invalid JSON: %q", key, raw)
		}
	}
}

// A reminder time is never rejected: the user asked for a reminder, and a
// settings write that fails with no explanation is worse than one at the
// default time.
func TestReminderTimeFallsBackOnGarbage(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo)
	ctx := context.Background()

	cases := map[string]string{
		"":           DefaultReminderTime,
		"not a time": DefaultReminderTime,
		"25:00":      DefaultReminderTime,
		"12:60":      DefaultReminderTime,
		"09:05":      "09:05",
		"9:05":       "09:05",
		"23:59":      "23:59",
		"00:00":      "00:00",
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			snapshot, _, err := s.Apply(ctx, Patch{JournalReminderTime: ptr(input)})
			if err != nil {
				t.Fatalf("Apply(%q): %v", input, err)
			}
			if snapshot.ReminderTime != want {
				t.Fatalf("time = %q for input %q, want %q", snapshot.ReminderTime, input, want)
			}
		})
	}
}
