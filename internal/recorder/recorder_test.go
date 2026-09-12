package recorder

import (
	"context"
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
	"github.com/Jwz-git/Daygo/internal/settings"
	"os"
	"sync"
	"testing"
	"time"
)

type testStore struct {
	mu                     sync.Mutex
	next, commits, blocked int
	relativePaths          []string
}

func (s *testStore) Begin(_ context.Context, relativePath string, _ time.Time, _ *int, _, _ int, _ bool) (int64, error) {
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
	if err := r.Pause(); err != nil {
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
