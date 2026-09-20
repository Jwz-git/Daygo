package app

import (
	"context"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
)

func TestApplicationActivationRoutesToWindowAction(t *testing.T) {
	sys := fake.NewSystem()
	b := NewBackend(sys, nil)
	activated := make(chan struct{}, 1)
	b.setActivationAction(func() { activated <- struct{}{} })
	b.startSystemEventPump()

	sys.Emit(platform.EventApplicationActivated)
	select {
	case <-activated:
	case <-time.After(time.Second):
		t.Fatal("application activation was not routed to the window action")
	}
}

func TestQuitAllowedDefaultsFalse(t *testing.T) {
	b := NewBackend(fake.NewSystem(), nil)
	if b.quitAllowed() {
		t.Fatal("a fresh backend must not allow real termination; Cmd+Q/Dock quit soft-quit to background")
	}
}

func TestRequestQuitAllowsTermination(t *testing.T) {
	b := NewBackend(fake.NewSystem(), nil)
	b.requestQuit()
	if !b.quitAllowed() {
		t.Fatal("requestQuit must let the status-bar Quit terminate for real")
	}
}

func TestEnterBackgroundDropsDockIcon(t *testing.T) {
	sys := fake.NewSystem()
	b := NewBackend(sys, nil)
	if err := b.enterBackground(context.Background()); err != nil {
		t.Fatalf("enterBackground: %v", err)
	}
	got, set := sys.ActivationPolicy()
	if !set || got != platform.ActivationAccessory {
		t.Fatalf("enterBackground must set accessory policy, got (%q, set=%v)", got, set)
	}
}

func TestExitBackgroundRestoresDockIcon(t *testing.T) {
	sys := fake.NewSystem()
	b := NewBackend(sys, nil)
	if err := b.exitBackground(context.Background()); err != nil {
		t.Fatalf("exitBackground: %v", err)
	}
	got, set := sys.ActivationPolicy()
	if !set || got != platform.ActivationRegular {
		t.Fatalf("exitBackground must set regular policy, got (%q, set=%v)", got, set)
	}
}

func TestBackgroundTransitionsNilSystem(t *testing.T) {
	b := NewBackend(nil, nil)
	if err := b.enterBackground(context.Background()); err != nil {
		t.Fatalf("enterBackground with nil system must be a no-op: %v", err)
	}
	if err := b.exitBackground(context.Background()); err != nil {
		t.Fatalf("exitBackground with nil system must be a no-op: %v", err)
	}
}
