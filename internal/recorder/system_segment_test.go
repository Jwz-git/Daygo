package recorder

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
	"github.com/Jwz-git/Daygo/internal/settings"
)

type systemSegmentCapture struct {
	*fake.SegmentCapture
	mu                             sync.Mutex
	closes                         int
	err                            error
	entered, release               chan struct{}
	inCapture, closedDuringCapture bool
}

func (c *systemSegmentCapture) Capture(ctx context.Context, req platform.CaptureRequest) (platform.CaptureResult, error) {
	c.mu.Lock()
	c.inCapture = true
	c.mu.Unlock()
	if c.entered != nil {
		close(c.entered)
		<-c.release
	}
	result, err := c.SegmentCapture.Capture(ctx, req)
	c.mu.Lock()
	c.inCapture = false
	c.mu.Unlock()
	return result, err
}

func (c *systemSegmentCapture) CloseActiveSegment(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes++
	c.closedDuringCapture = c.closedDuringCapture || c.inCapture
	return c.err
}

func newSystemSegmentRecorder(t *testing.T, capture *systemSegmentCapture) (*Recorder, *testStore) {
	t.Helper()
	store := &testStore{}
	r, err := New(Config{Capture: capture, Store: store, Settings: settings.Snapshot{CaptureIntervalSeconds: 10, CaptureHeightPixels: 18}, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	r.state = StateCapturing
	return r, store
}

func TestSystemPauseFinalizesSegment(t *testing.T) {
	for _, kind := range []platform.SystemEventKind{platform.EventSleep, platform.EventScreenLocked, platform.EventScreensaverStart} {
		t.Run(string(kind), func(t *testing.T) {
			c := &systemSegmentCapture{SegmentCapture: fake.NewSegmentCapture()}
			r, _ := newSystemSegmentRecorder(t, c)
			r.lastSegmentPath, r.lastSegmentSize = "segments/anonymous-active.mp4", 1000
			r.HandleSystemEvent(platform.SystemEvent{Kind: kind})
			if r.State() != StatePaused || c.closes != 1 || r.ActiveSegmentPath() != "" {
				t.Fatalf("state=%s closes=%d active=%q", r.State(), c.closes, r.ActiveSegmentPath())
			}
		})
	}
}

func TestSystemPauseWaitsForInFlightSegmentCapture(t *testing.T) {
	c := &systemSegmentCapture{SegmentCapture: fake.NewSegmentCapture(), entered: make(chan struct{}), release: make(chan struct{})}
	r, store := newSystemSegmentRecorder(t, c)
	captured := make(chan error, 1)
	go func() { captured <- r.capture(context.Background()) }()
	<-c.entered
	paused := make(chan struct{})
	go func() { r.HandleSystemEvent(platform.SystemEvent{Kind: platform.EventSleep}); close(paused) }()
	deadline := time.After(time.Second)
	for r.State() != StatePaused {
		select {
		case <-deadline:
			t.Fatal("system pause did not take effect")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	select {
	case <-paused:
		t.Error("pause returned before in-flight capture settled")
	default:
	}
	close(c.release)
	if err := <-captured; err != nil {
		t.Fatal(err)
	}
	<-paused
	store.mu.Lock()
	commits := store.commits
	store.mu.Unlock()
	if c.closedDuringCapture || commits != 1 || r.ActiveSegmentPath() != "" {
		t.Fatalf("close raced capture=%v commits=%d active=%q", c.closedDuringCapture, commits, r.ActiveSegmentPath())
	}
}

func TestSystemFinalizeFailureStaysVisibleAndCanRetry(t *testing.T) {
	want := errors.New("anonymous finalize failure")
	c := &systemSegmentCapture{SegmentCapture: fake.NewSegmentCapture(), err: want}
	r, _ := newSystemSegmentRecorder(t, c)
	r.lastSegmentPath, r.lastSegmentSize = "segments/anonymous-active.mp4", 1000
	r.HandleSystemEvent(platform.SystemEvent{Kind: platform.EventScreenLocked})
	if !errors.Is(r.LastError(), want) || r.ActiveSegmentPath() == "" {
		t.Fatal("finalize failure lost its error or active protection")
	}
	r.HandleSystemEvent(platform.SystemEvent{Kind: platform.EventScreenUnlocked})
	r.mu.Lock()
	generation := r.resumeGeneration
	r.mu.Unlock()
	r.resumeAfterSystemEvent(generation)
	if r.State() != StatePaused {
		t.Fatal("auto-resumed after failed finalization")
	}
	c.mu.Lock()
	c.err = nil
	c.mu.Unlock()
	if err := r.Resume(); err != nil {
		t.Fatal(err)
	}
	if r.State() != StateCapturing || r.ActiveSegmentPath() != "" {
		t.Fatal("retry did not finalize before resuming")
	}
}
