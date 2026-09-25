package recorder

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
	"github.com/Jwz-git/Daygo/internal/settings"
)

type testStore struct {
	mu                     sync.Mutex
	next, commits, blocked int
	abandons               int
	relativePaths          []string
}

func (s *testStore) Begin(_ context.Context, relativePath string, _ int, _ time.Time, _ *int, _, _ int, _ bool) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	s.relativePaths = append(s.relativePaths, relativePath)
	return int64(s.next), nil
}
func (s *testStore) Commit(context.Context, int64, int64) error {
	s.mu.Lock()
	s.commits++
	s.mu.Unlock()
	return nil
}
func (s *testStore) MarkBlocked(context.Context, int64) error {
	s.mu.Lock()
	s.blocked++
	s.mu.Unlock()
	return nil
}
func (s *testStore) Abandon(context.Context, int64) error {
	s.mu.Lock()
	s.abandons++
	s.mu.Unlock()
	return nil
}
func (s *testStore) AmortizeSegment(context.Context, string, int64) error {
	return nil
}

type recordingCapture struct {
	fake    *fake.Capture
	mu      sync.Mutex
	heights []int
}

func (c *recordingCapture) Capture(ctx context.Context, req platform.CaptureRequest) (platform.CaptureResult, error) {
	c.mu.Lock()
	c.heights = append(c.heights, req.TargetHeight)
	c.mu.Unlock()
	return c.fake.Capture(ctx, req)
}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	value := c.now
	c.now = c.now.Add(time.Nanosecond)
	return value
}

func TestRecorderPauseResumeAndStop(t *testing.T) {
	dir := t.TempDir()
	store := &testStore{}
	events := make(chan Event, 16)
	r, err := New(Config{Capture: fake.NewCapture(), Store: store, Settings: settings.Snapshot{CaptureIntervalSeconds: 1, CaptureHeightPixels: 18}, Directory: dir, Clock: &testClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, OnEvent: func(e Event) { events <- e }})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StateCapturing)
	waitForCommit(t, store)
	if lastFrameAt := r.LastFrameAt(); lastFrameAt == nil {
		t.Fatal("LastFrameAt is nil after a committed capture")
	}
	store.mu.Lock()
	relativePath := store.relativePaths[0]
	store.mu.Unlock()
	if !platform.ValidSegmentPath(relativePath) {
		t.Fatalf("relative capture path %q is not canonical", relativePath)
	}
	if err := r.Pause(0); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StatePaused)
	if err := r.Resume(); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StateCapturing)
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StateIdle)
	store.mu.Lock()
	commits := store.commits
	store.mu.Unlock()
	if commits < 1 {
		t.Fatalf("commits=%d", commits)
	}
	if _, err := os.Stat(dir + "/staging"); err != nil {
		t.Fatal(err)
	}
}
func TestRecorderUpdateSettingsAffectsNextCapture(t *testing.T) {
	dir := t.TempDir()
	store := &testStore{}
	capture := &recordingCapture{fake: fake.NewCapture()}
	clock := &testClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	r, err := New(Config{Capture: capture, Store: store, Settings: settings.Snapshot{CaptureIntervalSeconds: 1, CaptureHeightPixels: 1080}, Directory: dir, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer r.Stop()
	deadline := time.Now().Add(3 * time.Second)
	for {
		capture.mu.Lock()
		count := len(capture.heights)
		capture.mu.Unlock()
		if count >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("initial capture did not run")
		}
		time.Sleep(5 * time.Millisecond)
	}
	r.UpdateSettings(settings.Snapshot{CaptureIntervalSeconds: 1, CaptureHeightPixels: 720})
	for {
		capture.mu.Lock()
		heights := append([]int(nil), capture.heights...)
		capture.mu.Unlock()
		if len(heights) >= 2 {
			if heights[1] != 720 {
				t.Fatalf("updated capture height = %d, want 720", heights[1])
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("updated capture did not run")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRecorderWaitsForEverySystemBlockerBeforeResuming(t *testing.T) {
	dir := t.TempDir()
	store := &testStore{}
	events := make(chan Event, 16)
	r, err := New(Config{
		Capture:   fake.NewCapture(),
		Store:     store,
		Settings:  settings.Snapshot{CaptureIntervalSeconds: 1, CaptureHeightPixels: 18},
		Directory: dir,
		OnEvent:   func(event Event) { events <- event },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer r.Stop()
	waitState(t, events, StateCapturing)

	r.HandleSystemEvent(platform.SystemEvent{Kind: platform.EventSleep})
	waitState(t, events, StatePaused)
	r.HandleSystemEvent(platform.SystemEvent{Kind: platform.EventScreenLocked})
	r.HandleSystemEvent(platform.SystemEvent{Kind: platform.EventWake})
	if state := r.State(); state != StatePaused {
		t.Fatalf("state after wake while locked = %q, want paused", state)
	}
	if err := r.Resume(); err == nil {
		t.Fatal("manual resume succeeded while the screen is locked")
	}

	r.HandleSystemEvent(platform.SystemEvent{Kind: platform.EventScreenUnlocked})
	waitState(t, events, StateCapturing)
}
func waitForCommit(t *testing.T, s *testStore) {
	deadline := time.Now().Add(3 * time.Second)
	for {
		s.mu.Lock()
		n := s.commits
		s.mu.Unlock()
		if n > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("initial capture did not commit")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func waitState(t *testing.T, ch <-chan Event, want State) {
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case e := <-ch:
			if e.Err != nil {
				t.Fatalf("state %q: %v", e.State, e.Err)
			}
			if e.State == want {
				return
			}
		case <-timer.C:
			t.Fatalf("did not observe state %q", want)
		}
	}
}

var _ platform.Capture = (*fake.Capture)(nil)

// A capture error must not stop the loop: a resident recorder survives
// transient failures (display switch, permission hiccup) and keeps capturing.
// A longer failure run must also remain recoverable without user intervention.
func TestRecorderSurvivesTransientCaptureErrors(t *testing.T) {
	dir := t.TempDir()
	store := &testStore{}
	capture := &flakyCapture{failuresLeft: 1}
	r, err := New(Config{Capture: capture, Store: store, Settings: settings.Snapshot{CaptureIntervalSeconds: 1, CaptureHeightPixels: 18}, Directory: dir, Clock: &testClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer r.Stop()
	waitForCommits(t, store, 1)
	if r.State() != StateCapturing {
		t.Fatalf("state after one transient failure = %q, want capturing", r.State())
	}
	if got := r.LastError(); got != nil {
		t.Fatalf("last error after recovery = %v, want nil", got)
	}

	// Beyond the former failure limit the loop keeps trying and recovers.
	capture.mu.Lock()
	capture.failuresLeft = 4
	capture.mu.Unlock()
	waitForCommits(t, store, 2)
	if got := r.State(); got != StateCapturing {
		t.Fatalf("state after recovery = %q, want capturing", got)
	}
	if got := r.LastError(); got != nil {
		t.Fatalf("last error after recovery = %v, want nil", got)
	}
}

// waitForCommits blocks until the store has at least n commits.
func waitForCommits(t *testing.T, s *testStore, n int) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		got := s.commits
		s.mu.Unlock()
		if got >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("commits did not reach %d", n)
}

func waitStateIdle(t *testing.T, r *Recorder) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if r.State() == StateIdle {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("recorder did not reach idle")
}

// flakyCapture fails its next N captures, then succeeds.
type flakyCapture struct {
	mu           sync.Mutex
	failuresLeft int
}

func (c *flakyCapture) Capture(_ context.Context, req platform.CaptureRequest) (platform.CaptureResult, error) {
	c.mu.Lock()
	fail := c.failuresLeft > 0
	if fail {
		c.failuresLeft--
	}
	c.mu.Unlock()
	if fail {
		return platform.CaptureResult{}, errors.New("transient capture failure")
	}
	img := image.NewRGBA(image.Rect(0, 0, 16, 9))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 50}); err != nil {
		return platform.CaptureResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(req.OutputPath), 0o700); err != nil {
		return platform.CaptureResult{}, err
	}
	return platform.CaptureResult{Outcome: platform.CaptureWritten}, os.WriteFile(req.OutputPath, buf.Bytes(), 0o600)
}

func TestRecorderSegmentCaptureWithCloser(t *testing.T) {
	dir := t.TempDir()
	store := &testStore{}
	events := make(chan Event, 16)
	capture := fake.NewSegmentCapture()
	r, err := New(Config{
		Capture:   capture,
		Store:     store,
		Settings:  settings.Snapshot{CaptureIntervalSeconds: 1, CaptureHeightPixels: 18},
		Directory: dir,
		Clock:     &testClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		OnEvent:   func(e Event) { events <- e },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StateCapturing)
	waitForCommit(t, store)

	store.mu.Lock()
	rel := store.relativePaths[0]
	store.mu.Unlock()

	if rel != "segments/fake-segment.mp4" {
		t.Fatalf("expected segment path segments/fake-segment.mp4, got %s", rel)
	}

	if err := r.Pause(0); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StatePaused)

	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StateIdle)
}

// #8 regression: a system blocker (sleep/lock/screensaver) arriving during a
// timed user pause must not orphan the pause's auto-resume. The blocker must
// not bump resumeGeneration; the timed pause still expires (clearing the user
// hold without capturing an unavailable display), and when the blocker lifts
// the recorder returns to capturing. Before the fix the sleep branch bumped the
// generation, the pause timer no-op'd, userPaused stayed set, and the unblock
// refused to re-arm — freezing the recorder in StatePaused forever.
func TestRecorderTimedPauseSurvivesSystemBlocker(t *testing.T) {
	dir := t.TempDir()
	store := &testStore{}
	events := make(chan Event, 32)
	r, err := New(Config{Capture: fake.NewCapture(), Store: store, Settings: settings.Snapshot{CaptureIntervalSeconds: 1, CaptureHeightPixels: 18}, Directory: dir, Clock: &testClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, OnEvent: func(e Event) { events <- e }})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer r.Stop()
	waitState(t, events, StateCapturing)

	if err := r.Pause(60 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StatePaused)
	r.HandleSystemEvent(platform.SystemEvent{Kind: platform.EventScreensaverStart})

	// The pause timer fires while blocked: it drops the user hold but must not
	// resume onto an unavailable display, so the recorder stays paused.
	time.Sleep(150 * time.Millisecond)
	if state := r.State(); state != StatePaused {
		t.Fatalf("state after timed pause expired while blocked = %q, want paused", state)
	}

	// Lifting the blocker re-arms the resume; the recorder returns to capturing.
	r.HandleSystemEvent(platform.SystemEvent{Kind: platform.EventScreensaverStop})
	waitState(t, events, StateCapturing)
}

func TestRecorderTimedPauseAutoResumes(t *testing.T) {
	dir := t.TempDir()
	store := &testStore{}
	events := make(chan Event, 16)
	r, err := New(Config{Capture: fake.NewCapture(), Store: store, Settings: settings.Snapshot{CaptureIntervalSeconds: 1, CaptureHeightPixels: 18}, Directory: dir, Clock: &testClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, OnEvent: func(e Event) { events <- e }})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StateCapturing)

	if err := r.Pause(40 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StatePaused)
	// The timer fires on real time; the recorder returns to capturing on its
	// own without a Resume call.
	waitState(t, events, StateCapturing)

	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StateIdle)
}

// slowCloser delays the second CloseActiveSegment (the run() teardown call) to
// widen the window in which a pending resume timer can race the teardown. The
// first call (from Pause) stays fast so the test reaches Stop() promptly.
type slowCloser struct {
	inner *fake.SegmentCapture
	mu    sync.Mutex
	calls int
	delay time.Duration
}

type failingCloser struct {
	*fake.SegmentCapture
	err error
}

func (c *failingCloser) CloseActiveSegment(context.Context) error { return c.err }

func TestStopReportsSegmentFinalizeFailure(t *testing.T) {
	want := errors.New("fixture: finalize failed")
	r, err := New(Config{Capture: &failingCloser{SegmentCapture: fake.NewSegmentCapture(), err: want}, Store: &testStore{}, Settings: settings.Snapshot{CaptureIntervalSeconds: 1, CaptureHeightPixels: 18}, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := r.Stop(); !errors.Is(err, want) {
		t.Fatalf("Stop error = %v, want finalize failure", err)
	}
	if err := r.Start(t.Context()); !errors.Is(err, want) {
		t.Fatalf("Start after failed finalize = %v, want original failure", err)
	}
}

func (s *slowCloser) Capture(ctx context.Context, req platform.CaptureRequest) (platform.CaptureResult, error) {
	return s.inner.Capture(ctx, req)
}

func (s *slowCloser) CloseActiveSegment(ctx context.Context) error {
	s.mu.Lock()
	s.calls++
	n := s.calls
	s.mu.Unlock()
	if n >= 2 {
		time.Sleep(s.delay)
	}
	return s.inner.CloseActiveSegment(ctx)
}

// #17 regression: Stop() during a timed pause must cancel the pending
// auto-resume timer. run()'s teardown finalizes the active segment (slow disk
// I/O) before it sets StateIdle; if the resume timer fires in that window and
// Stop did not bump resumeGeneration, it wins the lock while state is still
// StatePaused and emits a spurious StateCapturing after the user asked to stop.
func TestRecorderStopCancelsPendingResumeTimer(t *testing.T) {
	dir := t.TempDir()
	store := &testStore{}
	events := make(chan Event, 32)
	capture := &slowCloser{inner: fake.NewSegmentCapture(), delay: 200 * time.Millisecond}
	r, err := New(Config{Capture: capture, Store: store, Settings: settings.Snapshot{CaptureIntervalSeconds: 1, CaptureHeightPixels: 18}, Directory: dir, Clock: &testClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, OnEvent: func(e Event) { events <- e }})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StateCapturing)

	// Arm a resume timer that will fire during Stop()'s teardown window.
	if err := r.Pause(50 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	waitState(t, events, StatePaused)
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	// Stop blocks until teardown (including the slow finalize) completes, well
	// past the 50ms timer. Give any stray emit a moment to arrive, then assert
	// no StateCapturing followed the pause we already consumed above.
	time.Sleep(50 * time.Millisecond)
	for {
		select {
		case e := <-events:
			if e.State == StateCapturing {
				t.Fatal("spurious StateCapturing emitted after Stop during teardown")
			}
		default:
			return
		}
	}
}

// #9 regression: repeated capture failures must remain visible as errors while
// the resident recorder retries; they must not masquerade as a user Stop().
func TestRecorderFailureRemainsLive(t *testing.T) {
	dir := t.TempDir()
	store := &testStore{}
	events := make(chan Event, 64)
	capture := &flakyCapture{failuresLeft: 4}
	r, err := New(Config{Capture: capture, Store: store, Settings: settings.Snapshot{CaptureIntervalSeconds: 1, CaptureHeightPixels: 18}, Directory: dir, Clock: &testClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, OnEvent: func(e Event) { events <- e }})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	defer r.Stop()
	deadline := time.NewTimer(12 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case e := <-events:
			if e.Err == nil {
				continue
			}
			if e.State != StateCapturing {
				t.Fatalf("failure event state = %q, want capturing", e.State)
			}
			if e.Err.Error() != "transient capture failure" {
				t.Fatalf("failure event error = %v", e.Err)
			}
			waitForCommits(t, store, 1)
			if got := r.State(); got != StateCapturing {
				t.Fatalf("state after recovery = %q", got)
			}
			return
		case <-deadline.C:
			t.Fatal("recorder did not recover from repeated capture errors")
		}
	}
}
