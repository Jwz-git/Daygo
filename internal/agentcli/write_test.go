package agentcli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jwz-git/Daygo/internal/agentbridge"
)

// fakeWrites is a bridge Handler with a switchable gate; it records the last
// operation so the test can assert the CLI forwarded it unchanged.
type fakeWrites struct {
	enabled bool
	lastOp  string
	lastArg string
	err     error
}

func (f *fakeWrites) EditsEnabled(context.Context) (bool, error) { return f.enabled, nil }

func (f *fakeWrites) Execute(_ context.Context, op string, args json.RawMessage) (json.RawMessage, error) {
	f.lastOp, f.lastArg = op, string(args)
	if f.err != nil {
		return nil, f.err
	}
	return json.RawMessage(`{"day":"2026-09-12"}`), nil
}

// startBridge serves agent.sock next to a DAYGO_DB override, the location the
// CLI resolves. The directory is under /tmp because macOS caps Unix socket
// paths near 104 bytes and t.TempDir is longer than that.
func startBridge(t *testing.T, handler agentbridge.Handler) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "daygo-cli-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("DAYGO_DB", filepath.Join(dir, "daygo.sqlite"))
	server := agentbridge.NewServer(handler, nil)
	if err := server.Start(context.Background(), filepath.Join(dir, "agent.sock")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
}

func TestWriteForwardsThroughSocket(t *testing.T) {
	handler := &fakeWrites{enabled: true}
	startBridge(t, handler)
	code, out, errOut := runCLI("write", "goal_set", `{"day":"2026-09-12","focusTargetMinutes":120}`, "--json")
	if code != exitOK || errOut != "" {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
	if handler.lastOp != "goal_set" || !strings.Contains(handler.lastArg, `"focusTargetMinutes":120`) {
		t.Fatalf("forwarded %q %s", handler.lastOp, handler.lastArg)
	}
	var envelope struct {
		SchemaVersion int             `json:"schema_version"`
		Data          json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil || envelope.SchemaVersion != 1 || !strings.Contains(string(envelope.Data), "2026-09-12") {
		t.Fatalf("stdout = %q (%v)", out, err)
	}
}

func TestWriteGateAndErrorCodes(t *testing.T) {
	handler := &fakeWrites{}
	startBridge(t, handler)
	args := []string{"write", "card_delete", `{"cardId":7}`, "--json"}

	code, _, errOut := runCLI(args...)
	if code != exitUnexpected || !strings.Contains(errOut, `"code": "edits_disabled"`) || handler.lastOp != "" {
		t.Fatalf("disabled: exit=%d stderr=%q op=%q", code, errOut, handler.lastOp)
	}

	handler.enabled = true
	handler.err = agentbridge.Errorf(agentbridge.CodeNotFound, "card not found")
	if code, _, errOut := runCLI(args...); code != exitNotFound || !strings.Contains(errOut, "card not found") {
		t.Fatalf("not found: exit=%d stderr=%q", code, errOut)
	}
	handler.err = agentbridge.Errorf(agentbridge.CodeInvalidArgument, "bad arguments")
	if code, _, _ := runCLI(args...); code != exitUsage {
		t.Fatalf("invalid argument: exit=%d, want %d", code, exitUsage)
	}
}

func TestWriteUsageErrorsNeverDial(t *testing.T) {
	handler := &fakeWrites{enabled: true}
	startBridge(t, handler)
	for _, args := range [][]string{
		{"write"},
		{"write", "goal_set"},
		{"write", "drop_table", `{}`},
		{"write", "goal_set", `{not json`},
	} {
		if code, _, _ := runCLI(args...); code != exitUsage {
			t.Errorf("%v: exit=%d, want %d", args, code, exitUsage)
		}
	}
	if handler.lastOp != "" {
		t.Fatalf("usage error reached the socket: %q", handler.lastOp)
	}
}

func TestWriteWithoutRunningDaygo(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "daygo-cli-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("DAYGO_DB", filepath.Join(dir, "daygo.sqlite"))
	code, _, errOut := runCLI("write", "goal_set", `{"day":"2026-09-12"}`)
	if code != exitUnexpected || !strings.Contains(errOut, "connect to agent socket failed") {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
}
