package agentcli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/storage"
)

// newDB creates an empty migrated business database, closes the writer, sets
// DAYGO_DB to point the CLI at it, and returns nothing: the env var is the
// seam Main/run use to resolve the path.
func newDB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	w, err := storage.Open(context.Background(), storage.Options{Dir: dir, Location: time.UTC})
	if err != nil {
		t.Fatalf("open writer: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	t.Setenv("DAYGO_DB", dbPath(dir))
}

func dbPath(dir string) string { return dir + "/" + storage.DatabaseFileName }

// runCLI invokes one command with captured streams and returns exit, stdout,
// stderr.
func runCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(context.Background(), args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestStatusJSONEnvelope(t *testing.T) {
	newDB(t)
	code, out, errb := runCLI("status", "--json")
	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitOK, errb)
	}
	if !strings.HasPrefix(out, "{\n  \"schema_version\": 1,") {
		t.Fatalf("stdout is not a schema_version-led envelope:\n%s", out)
	}
	if !strings.Contains(out, `"database_path"`) {
		t.Fatalf("status json missing database_path:\n%s", out)
	}
}

func TestStatusTextMode(t *testing.T) {
	newDB(t)
	code, out, _ := runCLI("status")
	if code != exitOK {
		t.Fatalf("exit = %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "database:") {
		t.Fatalf("text status missing database line:\n%s", out)
	}
}

func TestCardNotFoundExit(t *testing.T) {
	newDB(t)
	code, _, errb := runCLI("card", "9999", "--json")
	if code != exitNotFound {
		t.Fatalf("exit = %d, want %d", code, exitNotFound)
	}
	if !strings.Contains(errb, `"code": "not_found"`) {
		t.Fatalf("stderr missing not_found error envelope:\n%s", errb)
	}
	if !strings.HasPrefix(errb, "{\n  \"schema_version\": 1,") {
		t.Fatalf("error envelope must lead with schema_version:\n%s", errb)
	}
}

func TestBadCardIDIsUsageError(t *testing.T) {
	newDB(t)
	code, _, errb := runCLI("card", "abc", "--json")
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errb, `"code": "invalid_argument"`) {
		t.Fatalf("stderr missing invalid_argument:\n%s", errb)
	}
}

func TestUnknownCommand(t *testing.T) {
	code, _, errb := runCLI("frobnicate")
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errb, "unknown command") {
		t.Fatalf("stderr missing unknown command:\n%s", errb)
	}
}

func TestNoArgsIsUsageError(t *testing.T) {
	code, _, _ := runCLI()
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
}

func TestHandlesOnlyKnownCommands(t *testing.T) {
	for _, c := range Commands {
		if !Handles(c) {
			t.Errorf("Handles(%q) = false, want true", c)
		}
	}
	for _, c := range []string{"", "mcp", "serve", "status "} {
		if Handles(c) {
			t.Errorf("Handles(%q) = true, want false", c)
		}
	}
}
