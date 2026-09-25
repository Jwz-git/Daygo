package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jwz-git/Daygo/internal/agentbridge"
)

func TestAgentBridgeWriteGateAndSharedGoalEvent(t *testing.T) {
	backend, emitter := writerBackendWithStore(t, t.TempDir())
	var audit bytes.Buffer
	shortDir, err := os.MkdirTemp("/tmp", "daygo-bridge-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(shortDir) })
	path := filepath.Join(shortDir, "agent.sock")
	server := agentbridge.NewServer(agentWriteHandler{backend: backend}, &audit)
	if err := server.Start(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	client := agentbridge.NewClient(path)
	args := map[string]any{"day": "2026-09-12", "focusTargetMinutes": 120}
	_, err = client.Do(context.Background(), "goal_set", args, agentbridge.SourceMCP)
	var bridgeErr *agentbridge.Error
	if !errors.As(err, &bridgeErr) || bridgeErr.Code != agentbridge.CodeEditsDisabled {
		t.Fatalf("default gate = %v, want edits_disabled", err)
	}
	enabled := true
	if _, err := backend.UpdateSettings(SettingsPatchDTO{AgentEditsEnabled: &enabled}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(context.Background(), "goal_set", map[string]any{"day": "bad"}, agentbridge.SourceMCP); !errors.As(err, &bridgeErr) || bridgeErr.Code != agentbridge.CodeInvalidArgument {
		t.Fatalf("invalid arguments = %v", err)
	}
	if _, err := client.Do(context.Background(), "goal_set", args, agentbridge.SourceMCP); err != nil {
		t.Fatal(err)
	}
	goal, _, found, err := backend.store().Goals().Get(context.Background(), "2026-09-12")
	if err != nil || !found || goal.FocusTargetMinutes != 120 {
		t.Fatalf("goal = %+v, found=%v, err=%v", goal, found, err)
	}
	if emitter.count(EventGoalUpdated) != 1 || !strings.Contains(audit.String(), `"source":"mcp"`) {
		t.Fatalf("event/audit missing: events=%d audit=%s", emitter.count(EventGoalUpdated), audit.String())
	}
	enabled = false
	if _, err := backend.UpdateSettings(SettingsPatchDTO{AgentEditsEnabled: &enabled}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(context.Background(), "goal_set", args, agentbridge.SourceCLI); !errors.As(err, &bridgeErr) || bridgeErr.Code != agentbridge.CodeEditsDisabled {
		t.Fatalf("disabled gate = %v", err)
	}
}

func TestGetAgentConnectionReportsSocketState(t *testing.T) {
	backend, _ := writerBackendWithStore(t, t.TempDir())
	got, err := backend.GetAgentConnection()
	if err != nil {
		t.Fatal(err)
	}
	if got.SocketActive || got.ExecutablePath == "" || !filepath.IsAbs(got.ExecutablePath) {
		t.Fatalf("before start = %+v, want inactive with an absolute executable path", got)
	}
	backend.agentSocketActive.Store(true)
	if got, _ := backend.GetAgentConnection(); !got.SocketActive {
		t.Fatalf("after start = %+v, want active", got)
	}
}
