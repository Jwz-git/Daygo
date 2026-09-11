package fake

import (
	"context"
	"testing"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func TestSystemSetStatusItemStoresState(t *testing.T) {
	system := NewSystem()
	want := platform.StatusItemState{Visible: true, Tooltip: "Daygo", OpenLabel: "Open", PauseLabel: "Pause", QuitLabel: "Quit", PauseEnabled: true}
	if err := system.SetStatusItem(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	system.mu.Lock()
	got := system.statusItem
	system.mu.Unlock()
	if got != want {
		t.Fatalf("status item = %+v, want %+v", got, want)
	}
}
