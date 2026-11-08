package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
)

type failedRelaunchSystem struct {
	*fake.System
	err error
}

func (s *failedRelaunchSystem) Relaunch(context.Context) error { return s.err }

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
		t.Fatal("a fresh backend must have no explicit quit override")
	}
}

func TestDesktopQuitIntent(t *testing.T) {
	b := NewBackend(fake.NewSystem(), nil)
	if got := b.quitIntent(true); got != quitTerminate {
		t.Fatalf("ordinary macOS quit must terminate, got %v", got)
	}
	if got := b.quitIntent(false); got != quitBackground {
		t.Fatalf("other hosts must keep their existing fallback, got %v", got)
	}
	b.armPermissionRestart()
	if got := b.quitIntent(true); got != quitRelaunchForPermission {
		t.Fatalf("permission restart must relaunch, got %v", got)
	}
	b.requestQuit()
	if got := b.quitIntent(true); got != quitTerminate {
		t.Fatalf("explicit termination must take precedence over permission restart, got %v", got)
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
		t.Fatalf("repeated backgrounding must not re-push the accessory policy, got %d calls", calls)
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

// Normal AppKit activation owns the foreground transition. Re-showing in that
// path can cause a flash or re-post the very notification that triggered it.
func TestActivationRestoresOnlyAfterBackgrounding(t *testing.T) {
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
		t.Fatal("an activation after backgrounding must restore the window")
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

func TestPermissionRestartDefaultsDisarmed(t *testing.T) {
	b := NewBackend(fake.NewSystem(), nil)
	if b.permissionRestartArmed() {
		t.Fatal("a fresh backend must not arm the permission restart")
	}
}

func TestArmAndDisarmPermissionRestart(t *testing.T) {
	b := NewBackend(fake.NewSystem(), nil)
	if err := b.SetPermissionRestartArmed(true); err != nil {
		t.Fatalf("SetPermissionRestartArmed(true): %v", err)
	}
	if !b.permissionRestartArmed() {
		t.Fatal("arming must let the next quit terminate and relaunch")
	}
	if err := b.SetPermissionRestartArmed(false); err != nil {
		t.Fatalf("SetPermissionRestartArmed(false): %v", err)
	}
	if b.permissionRestartArmed() {
		t.Fatal("disarming must restore ordinary quit behavior")
	}
}

func TestBeginPermissionRestartSchedulesRelaunch(t *testing.T) {
	sys := fake.NewSystem()
	b := NewBackend(sys, nil)
	if err := b.beginPermissionRestart(); err != nil {
		t.Fatal(err)
	}
	if got := sys.Relaunches(); got != 1 {
		t.Fatalf("beginPermissionRestart must schedule exactly one relaunch, got %d", got)
	}
}

func TestBeginPermissionRestartWithoutRelauncherFails(t *testing.T) {
	b := NewBackend(nil, nil)
	if err := b.beginPermissionRestart(); err == nil {
		t.Fatal("missing relauncher must prevent quit")
	}
}

func TestRelaunchForPermissionPreservesProcessOnRelaunchFailure(t *testing.T) {
	want := errors.New("fixture: relaunch failed")
	b := NewBackend(&failedRelaunchSystem{System: fake.NewSystem(), err: want}, nil)
	quit := false
	b.setShutdownRequester(func() { quit = true })
	if err := b.RelaunchForPermission(); !errors.Is(err, want) {
		t.Fatalf("RelaunchForPermission error = %v", err)
	}
	if quit {
		t.Fatal("failed relaunch must not terminate the process")
	}
}

func TestRelaunchForPermissionWithoutShellIsUnavailable(t *testing.T) {
	// Without a desktop shell (no shutdown requester installed) the binding must
	// report native_unavailable rather than pretend it restarted.
	b := NewBackend(fake.NewSystem(), nil)
	if err := b.RelaunchForPermission(); err == nil {
		t.Fatal("RelaunchForPermission without a shell must fail rather than silently no-op")
	}
}
