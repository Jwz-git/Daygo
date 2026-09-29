//go:build darwin

package darwin

import (
	"sync"
	"testing"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func TestSystemShutdownMappingAndFullQueue(t *testing.T) {
	s := &System{events: make(chan platform.SystemEvent, 2)}
	s.push(1, 1)
	s.push(2, 2)
	s.push(11, 3)
	var found bool
	for len(s.events) > 0 {
		if (<-s.events).Kind == platform.EventSystemShutdown {
			found = true
		}
	}
	if !found || systemEventKind(11) != platform.EventSystemShutdown {
		t.Fatal("terminal shutdown event was lost")
	}
}

func TestSystemCloseRejectsLateCallbacks(t *testing.T) {
	s := &System{events: make(chan platform.SystemEvent, 32)}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			s.push(8, 1)
			s.pushAction(platform.EventStatusItemClick, 1)
		}
	}()
	go func() { defer wg.Done(); s.Close(); s.Close() }()
	wg.Wait()
	for range s.events {
	}
}

func TestSystemEventKindMapsApplicationActivation(t *testing.T) {
	if got := systemEventKind(8); got != platform.EventApplicationActivated {
		t.Fatalf("systemEventKind(8) = %q, want %q", got, platform.EventApplicationActivated)
	}
}

func TestSystemEventKindMapsApplicationVisibility(t *testing.T) {
	for input, want := range map[uint32]platform.SystemEventKind{9: platform.EventApplicationHidden, 10: platform.EventApplicationUnhidden} {
		if got := systemEventKind(input); got != want || !got.Paired() {
			t.Fatalf("visibility event %d maps to %q", input, got)
		}
	}
}
