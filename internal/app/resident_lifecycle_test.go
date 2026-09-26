package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
)

type residentSystemFixture struct {
	*fake.System
	available bool
	policyErr error
	policies  []platform.ActivationPolicy
}

func (s *residentSystemFixture) StatusItemAvailable(context.Context) (bool, error) {
	return s.available, nil
}
func (s *residentSystemFixture) SetActivationPolicy(ctx context.Context, p platform.ActivationPolicy) error {
	s.policies = append(s.policies, p)
	if s.policyErr != nil {
		return s.policyErr
	}
	return s.System.SetActivationPolicy(ctx, p)
}

func TestUnavailableStatusItemKeepsDockRecovery(t *testing.T) {
	sys := &residentSystemFixture{System: fake.NewSystem()}
	b := NewBackend(sys, nil)
	if err := b.enterBackground(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, _ := sys.ActivationPolicy(); got != platform.ActivationRegular {
		t.Fatalf("policy = %q, want regular without a status item", got)
	}
	shown := false
	b.restoreOnActivation(func() { shown = true })
	if !shown {
		t.Fatal("Dock activation must restore an ordered-out window")
	}
}

func TestFailedActivationPolicyCanBeRetried(t *testing.T) {
	want := errors.New("fixture: policy refused")
	sys := &residentSystemFixture{System: fake.NewSystem(), available: true, policyErr: want}
	b := NewBackend(sys, nil)
	if err := b.enterBackground(context.Background()); !errors.Is(err, want) {
		t.Fatalf("enterBackground error = %v", err)
	}
	sys.policyErr = nil
	if err := b.enterBackground(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sys.policies) != 2 {
		t.Fatalf("policy attempts = %d, want 2", len(sys.policies))
	}
	sys.policyErr = want
	if err := b.exitBackground(context.Background()); !errors.Is(err, want) {
		t.Fatalf("exitBackground error = %v", err)
	}
	sys.policyErr = nil
	if err := b.exitBackground(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, _ := sys.ActivationPolicy(); got != platform.ActivationRegular {
		t.Fatalf("policy = %q", got)
	}
}

func TestNormalQuitPreservesProcessAfterFinalizeFailure(t *testing.T) {
	b := NewBackend(nil, nil)
	b.requestQuit()
	want := errors.New("fixture: finalize failed")
	calls := 0
	err := b.prepareTermination(func() error { calls++; return want })
	if !errors.Is(err, want) || calls != 1 || b.quitAllowed() {
		t.Fatalf("error=%v calls=%d allowed=%v", err, calls, b.quitAllowed())
	}
}

func TestSystemQuitDoesNotRelaunchOrCancelForFinalizeFailure(t *testing.T) {
	b := NewBackend(fake.NewSystem(), nil)
	b.armPermissionRestart()
	b.requestSystemShutdown()
	if b.permissionRestartArmed() || !b.quitAllowed() {
		t.Fatal("system shutdown must bypass permission relaunch")
	}
	calls := 0
	if err := b.prepareTermination(func() error { calls++; return errors.New("fixture: finalize failed") }); err != nil || calls != 1 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestSystemShutdownEventRequestsTerminationOnce(t *testing.T) {
	sys := fake.NewSystem()
	b := NewBackend(sys, nil)
	requested := make(chan struct{}, 2)
	b.setShutdownRequester(func() { requested <- struct{}{} })
	b.startSystemEventPump()
	sys.Emit(platform.EventSystemShutdown)
	select {
	case <-requested:
	case <-time.After(time.Second):
		t.Fatal("shutdown was not routed")
	}
	b.requestSystemShutdown()
	select {
	case <-requested:
		t.Fatal("duplicate shutdown request")
	default:
	}
}

func TestDockPreferenceAppliesOnlyWhenRecoveryIsAvailable(t *testing.T) {
	sys := &residentSystemFixture{System: fake.NewSystem(), available: true}
	b := NewBackend(sys, nil)
	ctx := context.Background()
	if err := b.applyDockPreference(ctx, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := sys.ActivationPolicy(); got != platform.ActivationAccessory {
		t.Fatalf("policy=%q", got)
	}
	if b.needsWindowRestore() {
		t.Fatal("hiding Dock must not mark a visible window as soft-quit")
	}
	if err := b.enterBackground(ctx); err != nil {
		t.Fatal(err)
	}
	if err := b.exitBackground(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := sys.ActivationPolicy(); got != platform.ActivationAccessory {
		t.Fatalf("reopen ignored Dock preference: %q", got)
	}
	sys.available = false
	if err := b.applyDockPreference(ctx, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := sys.ActivationPolicy(); got != platform.ActivationRegular {
		t.Fatalf("missing menu must keep Dock: %q", got)
	}
}

func TestDockPreferenceChangeDoesNotReopenSoftQuit(t *testing.T) {
	sys := &residentSystemFixture{System: fake.NewSystem(), available: true}
	b := NewBackend(sys, nil)
	ctx := context.Background()
	if err := b.enterBackground(ctx); err != nil {
		t.Fatal(err)
	}
	if err := b.applyDockPreference(ctx, true); err != nil {
		t.Fatal(err)
	}
	if !b.needsWindowRestore() {
		t.Fatal("settings must not reopen a soft-quit window")
	}
	if got, _ := sys.ActivationPolicy(); got != platform.ActivationAccessory {
		t.Fatalf("soft quit policy=%q", got)
	}
}

func TestExplicitReopenRetriesFailedForegroundDockPreference(t *testing.T) {
	sys := &residentSystemFixture{System: fake.NewSystem(), available: true, policyErr: errors.New("fixture: refused")}
	b := NewBackend(sys, nil)
	if err := b.applyDockPreference(context.Background(), false); err == nil {
		t.Fatal("expected refusal")
	}
	sys.policyErr = nil
	if err := b.exitBackground(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, _ := sys.ActivationPolicy(); got != platform.ActivationAccessory {
		t.Fatalf("policy=%q", got)
	}
}

func TestUpdateSettingsAppliesDockPreferenceAfterPersistence(t *testing.T) {
	b, _ := backendWithStore(t)
	sys := &residentSystemFixture{System: fake.NewSystem(), available: true}
	b.system = sys
	show := false
	dto, err := b.UpdateSettings(SettingsPatchDTO{ShowDockIcon: &show})
	if err != nil {
		t.Fatal(err)
	}
	if dto.System.ShowDockIcon {
		t.Fatal("Dock preference was not persisted")
	}
	if got, _ := sys.ActivationPolicy(); got != platform.ActivationAccessory {
		t.Fatalf("policy=%q", got)
	}
	before := len(sys.policies)
	if _, err = b.UpdateSettings(SettingsPatchDTO{IntervalSeconds: ptrInt(5)}); err != nil {
		t.Fatal(err)
	}
	if len(sys.policies) != before {
		t.Fatal("unrelated setting reapplied policy")
	}
}
