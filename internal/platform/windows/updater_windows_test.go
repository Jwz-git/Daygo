//go:build windows

package windows

import (
	"testing"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func TestUpdaterFoundPublishesUnknownVersion(t *testing.T) {
	u := &Updater{events: make(chan platform.UpdaterEvent, 1), checking: true}
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
