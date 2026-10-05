package windows

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func TestUpdateSessionFoundPublishesUnknownVersion(t *testing.T) {
	u := &updateSession{events: make(chan platform.UpdaterEvent, 1), checking: true}
	u.reportFound()
	state, err := u.State(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if state.Checking || state.AvailableVersion == nil || *state.AvailableVersion != "" {
		t.Fatalf("found state = %+v, want available with unknown version", state)
	}
	select {
	case event := <-u.Events():
		if event.State.AvailableVersion == nil || *event.State.AvailableVersion != "" {
			t.Fatalf("found event = %+v", event)
		}
	default:
		t.Fatal("found callback did not publish event")
	}
}

func TestUpdateSessionInterruptClearsChecking(t *testing.T) {
	u := &updateSession{checking: true}
	u.interruptInstall()
	state, err := u.State(t.Context())
	if err != nil || state.Checking {
		t.Fatalf("terminal callback left check pending: %+v, %v", state, err)
	}
}

func TestUpdateSessionLaunchFailureRestoresRecordingOnce(t *testing.T) {
	var calls []string
	u := &updateSession{
		canInstall: func() bool { return true },
		prepare:    func() error { calls = append(calls, "finalize"); return nil },
		launch: func(path string) error {
			calls = append(calls, "launch:"+path)
			return errors.New("UAC cancelled")
		},
		shutdown: func() { calls = append(calls, "shutdown") },
	}
	u.SetInstallCancelled(func() { calls = append(calls, "resume") })
	if !u.canShutdown() {
		t.Fatal("owner with successful finalization must be allowed")
	}
	if u.launchInstaller("anonymous installer.exe") {
		t.Fatal("failed installer launch must not be accepted")
	}
	u.interruptInstall() // WinSparkle can also report cancellation/dismissal.
	u.requestShutdown()
	if want := []string{"finalize", "launch:anonymous installer.exe", "resume"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestUpdateSessionSuccessfulHandoffDoesNotResume(t *testing.T) {
	var calls []string
	u := &updateSession{
		canInstall: func() bool { return true },
		prepare:    func() error { calls = append(calls, "finalize"); return nil },
		launch:     func(string) error { calls = append(calls, "launch"); return nil },
		shutdown:   func() { calls = append(calls, "shutdown") },
	}
	u.SetInstallCancelled(func() { calls = append(calls, "resume") })
	if !u.canShutdown() || !u.launchInstaller("anonymous installer.exe") {
		t.Fatal("successful update must hand off to the installer")
	}
	u.interruptInstall() // A late terminal callback cannot undo the handoff.
	u.requestShutdown()
	u.requestShutdown()
	if want := []string{"finalize", "launch", "shutdown"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestUpdateSessionRefusesUnsafeHandoff(t *testing.T) {
	for _, name := range []string{"not owner", "finalization failed", "not prepared"} {
		t.Run(name, func(t *testing.T) {
			u := &updateSession{
				canInstall: func() bool { return name != "not owner" },
				prepare: func() error {
					if name == "finalization failed" {
						return errors.New("segment remains open")
					}
					return nil
				},
				launch:   func(string) error { t.Fatal("unsafe handoff launched installer"); return nil },
				shutdown: func() { t.Fatal("unsafe handoff requested shutdown") },
			}
			if name != "not prepared" && u.canShutdown() {
				t.Fatal("unsafe preparation was accepted")
			}
			if u.launchInstaller("anonymous installer.exe") {
				t.Fatal("unprepared install was accepted")
			}
			u.requestShutdown()
		})
	}
}

func TestUpdateSessionClosedIgnoresLateCallbacks(t *testing.T) {
	u := &updateSession{events: make(chan platform.UpdaterEvent, 1)}
	u.closeState()
	u.reportFound()
	u.interruptInstall()
	if _, open := <-u.Events(); open {
		t.Fatal("closed updater event channel is still open")
	}
	if u.canShutdown() || u.launchInstaller("anonymous installer.exe") {
		t.Fatal("closed updater accepted installation")
	}
}

func TestUpdateSessionCancelledInstallCanBeRetried(t *testing.T) {
	prepared, resumed := 0, 0
	u := &updateSession{
		canInstall: func() bool { return true },
		prepare:    func() error { prepared++; return nil },
		launch:     func(string) error { return nil },
		shutdown:   func() {},
	}
	u.SetInstallCancelled(func() { resumed++ })
	if !u.canShutdown() || !u.canShutdown() {
		t.Fatal("preparation must succeed, including a repeated native query")
	}
	u.interruptInstall()
	u.interruptInstall()
	if !u.canShutdown() || !u.launchInstaller("anonymous installer.exe") {
		t.Fatal("cancellation must allow a fresh installation attempt")
	}
	if prepared != 2 || resumed != 1 {
		t.Fatalf("prepared/resumed = %d/%d, want 2/1", prepared, resumed)
	}
}

func TestUpdateSessionConcurrentFoundAndClose(t *testing.T) {
	u := &updateSession{events: make(chan platform.UpdaterEvent, 8)}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 50 {
				u.reportFound()
				state, err := u.State(t.Context())
				if err != nil {
					t.Error(err)
				}
				if state.AvailableVersion != nil {
					*state.AvailableVersion = "mutate only this snapshot"
				}
			}
		})
	}
	wg.Go(u.closeState)
	wg.Wait()
}
