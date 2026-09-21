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
	ctx := context.Background()
	if err := b.enterBackground(ctx); err != nil {
		t.Fatalf("enterBackground: %v", err)
	}
	if err := b.exitBackground(ctx); err != nil {
		t.Fatalf("exitBackground: %v", err)
	}
	got, set := sys.ActivationPolicy()
	if !set || got != platform.ActivationRegular {
		t.Fatalf("exitBackground must set regular policy, got (%q, set=%v)", got, set)
	}
}

// The app launches already regular (Wails sets NSApplicationActivationPolicyRegular
// in applicationWillFinishLaunching), so restoring a foreground app must not push
// the policy again: a redundant setActivationPolicy(.regular) re-orders the app's
// windows while the system is still activating it.
func TestExitBackgroundOnForegroundAppLeavesPolicyUntouched(t *testing.T) {
	sys := fake.NewSystem()
	b := NewBackend(sys, nil)
	if err := b.exitBackground(context.Background()); err != nil {
		t.Fatalf("exitBackground: %v", err)
	}
	if calls := sys.ActivationPolicyCalls(); calls != 0 {
		t.Fatalf("exitBackground on a foreground app must not set the policy, got %d calls", calls)
	}
}

func TestBackgroundTransitionsAreIdempotent(t *testing.T) {
	sys := fake.NewSystem()
	b := NewBackend(sys, nil)
	ctx := context.Background()
	if err := b.enterBackground(ctx); err != nil {
		t.Fatalf("enterBackground: %v", err)
	}
	if err := b.enterBackground(ctx); err != nil {
		t.Fatalf("enterBackground: %v", err)
	}
	if calls := sys.ActivationPolicyCalls(); calls != 1 {
		t.Fatalf("a repeated soft-quit must not re-push the accessory policy, got %d calls", calls)
	}
	if err := b.exitBackground(ctx); err != nil {
		t.Fatalf("exitBackground: %v", err)
	}
	if err := b.exitBackground(ctx); err != nil {
		t.Fatalf("exitBackground: %v", err)
	}
	if calls := sys.ActivationPolicyCalls(); calls != 2 {
		t.Fatalf("a repeated restore must not re-push the regular policy, got %d calls", calls)
	}
}

// Only a soft-quit leaves the window ordered out with the app in the
// background. Every other activation is the system bringing the app forward on
// its own, where re-showing the window makes it flash and vanish — and where
// showWindow's own activateIgnoringOtherApps calls would re-post the very
// notification that triggered it.
func TestActivationRestoresOnlyAfterSoftQuit(t *testing.T) {
	sys := fake.NewSystem()
	b := NewBackend(sys, nil)
	ctx := context.Background()
	shown := 0
	show := func() { shown++ }

	b.restoreOnActivation(show)
	if shown != 0 {
		t.Fatal("an activation while the app is in the foreground must not re-show the window")
	}
	if err := b.enterBackground(ctx); err != nil {
		t.Fatalf("enterBackground: %v", err)
	}
	b.restoreOnActivation(show)
	if shown != 1 {
		t.Fatal("an activation after a soft-quit must restore the window")
	}
	if err := b.exitBackground(ctx); err != nil {
		t.Fatalf("exitBackground: %v", err)
	}
	b.restoreOnActivation(show)
	if shown != 1 {
		t.Fatal("an activation after a restore must not show the window a second time")
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
