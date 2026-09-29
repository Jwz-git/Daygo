package app

import (
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
	"testing"
	"time"
)

func TestUIVisibilityKeepsHideSourcesIndependent(t *testing.T) {
	b := NewBackend(fake.NewSystem(), nil)
	emitter := &recordingEmitter{}
	b.setEventEmitter(emitter)
	if !b.GetUIVisibility().Visible {
		t.Fatal("startup should be visible")
	}
	b.setApplicationHidden(true)
	b.setApplicationHidden(true)
	if b.GetUIVisibility().Visible || emitter.count(EventUIVisibilityChanged) != 1 {
		t.Fatal("native close must hide once")
	}
	b.setWindowHidden(true)
	b.setApplicationHidden(false)
	if b.GetUIVisibility().Visible {
		t.Fatal("unhide must not undo an ordered-out soft-quit window")
	}
	b.setWindowHidden(false)
	if !b.GetUIVisibility().Visible || emitter.count(EventUIVisibilityChanged) != 2 {
		t.Fatal("explicit reopen must restore once")
	}
	b.setApplicationHidden(true)
	b.setWindowHidden(false)
	if b.GetUIVisibility().Visible {
		t.Fatal("window show must not undo native app hide")
	}
	b.setApplicationHidden(false)
	if !b.GetUIVisibility().Visible {
		t.Fatal("native unhide did not restore")
	}
}
func TestSystemHideUnhideRoutesWithoutActivationSideEffects(t *testing.T) {
	sys := &visibilityFixtureSystem{System: fake.NewSystem(), events: make(chan platform.SystemEvent, 4)}
	b := NewBackend(sys, nil)
	b.startSystemEventPump()
	defer close(sys.events)
	sys.Emit(platform.EventApplicationHidden)
	waitUIVisibility(t, b, false)
	sys.Emit(platform.EventApplicationActivated)
	if b.GetUIVisibility().Visible {
		t.Fatal("activation alone undid app hide")
	}
	sys.Emit(platform.EventApplicationUnhidden)
	waitUIVisibility(t, b, true)
}
func waitUIVisibility(t *testing.T, b *Backend, visible bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if b.GetUIVisibility().Visible == visible {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("visibility did not become %v", visible)
}

type visibilityFixtureSystem struct {
	*fake.System
	events chan platform.SystemEvent
}

func (s *visibilityFixtureSystem) Events() <-chan platform.SystemEvent { return s.events }
func (s *visibilityFixtureSystem) Emit(kind platform.SystemEventKind) {
	s.events <- platform.SystemEvent{Kind: kind, At: time.Now()}
}
