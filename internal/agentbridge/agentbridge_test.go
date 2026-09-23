package agentbridge

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a mutex-guarded audit sink. The mutex both makes concurrent
// writes safe and gives the test a happens-before edge to read what the server
// goroutine wrote.
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type fakeHandler struct {
	enabled    bool
	enabledErr error
	exec       func(op string, args json.RawMessage) (json.RawMessage, error)
}

func (h *fakeHandler) EditsEnabled(context.Context) (bool, error) {
	return h.enabled, h.enabledErr
}

func (h *fakeHandler) Execute(_ context.Context, op string, args json.RawMessage) (json.RawMessage, error) {
	if h.exec != nil {
		return h.exec(op, args)
	}
	return json.RawMessage(`{"ok":true}`), nil
}

// tempSocket returns a short socket path; a t.TempDir()-based path can exceed
// the ~104-byte sun_path limit on macOS.
func tempSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "dg")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s")
}

func startServer(t *testing.T, h Handler, audit *syncBuffer) (*Client, string) {
	t.Helper()
	sock := tempSocket(t)
	var sink io.Writer
	if audit != nil {
		sink = audit
	}
	srv := NewServer(h, sink)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := srv.Start(ctx, sock); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return NewClient(sock), sock
}

// rawExchange dials the socket, sends line (a trailing newline is added), and
// returns the response line. It is the seam for malformed and oversize frames
// the typed Client would never send.
func rawExchange(t *testing.T, sock, line string) string {
	t.Helper()
	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(line + "\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && resp == "" {
		t.Fatalf("read: %v", err)
	}
	return resp
}

func decodeErr(t *testing.T, err error) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("error %v is not a *Error", err)
	}
	return e
}

func TestExecuteSuccessAndAudit(t *testing.T) {
	audit := &syncBuffer{}
	client, _ := startServer(t, &fakeHandler{enabled: true}, audit)

	data, err := client.Do(context.Background(), "card_update", map[string]any{"cardId": 1}, SourceMCP)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("data = %s, want the handler echo", data)
	}
	line := audit.String()
	if !strings.Contains(line, `"operation":"card_update"`) || !strings.Contains(line, `"source":"mcp"`) {
		t.Fatalf("audit line missing operation/source: %q", line)
	}
	if strings.Contains(line, "cardId") {
		t.Fatalf("audit must not carry arguments: %q", line)
	}
}

func TestAllOperationsAccepted(t *testing.T) {
	client, _ := startServer(t, &fakeHandler{enabled: true}, nil)
	for _, op := range Operations {
		if _, err := client.Do(context.Background(), op, nil, ""); err != nil {
			t.Errorf("Do(%q) = %v, want success", op, err)
		}
	}
}

func TestDefaultSourceIsSocket(t *testing.T) {
	audit := &syncBuffer{}
	client, _ := startServer(t, &fakeHandler{enabled: true}, audit)
	if _, err := client.Do(context.Background(), "goal_set", nil, ""); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !strings.Contains(audit.String(), `"source":"agent.sock"`) {
		t.Fatalf("empty source should default to agent.sock: %q", audit.String())
	}
}

func TestEditsDisabled(t *testing.T) {
	audit := &syncBuffer{}
	client, _ := startServer(t, &fakeHandler{enabled: false}, audit)
	_, err := client.Do(context.Background(), "card_delete", map[string]any{"cardId": 1}, SourceMCP)
	if got := decodeErr(t, err).Code; got != CodeEditsDisabled {
		t.Fatalf("code = %q, want %q", got, CodeEditsDisabled)
	}
	if audit.String() != "" {
		t.Fatalf("a gated write must not be audited: %q", audit.String())
	}
}

func TestEditsEnabledErrorIsInternal(t *testing.T) {
	client, _ := startServer(t, &fakeHandler{enabledErr: errors.New("settings down")}, nil)
	_, err := client.Do(context.Background(), "card_delete", nil, SourceMCP)
	if got := decodeErr(t, err).Code; got != CodeInternalError {
		t.Fatalf("code = %q, want %q", got, CodeInternalError)
	}
}

func TestUnknownOperation(t *testing.T) {
	client, _ := startServer(t, &fakeHandler{enabled: true}, nil)
	_, err := client.Do(context.Background(), "drop_everything", nil, SourceMCP)
	if got := decodeErr(t, err).Code; got != CodeUnknownOperation {
		t.Fatalf("code = %q, want %q", got, CodeUnknownOperation)
	}
}

func TestHandlerNotFoundMapping(t *testing.T) {
	h := &fakeHandler{enabled: true, exec: func(string, json.RawMessage) (json.RawMessage, error) {
		return nil, Errorf(CodeNotFound, "card 9 not found")
	}}
	client, _ := startServer(t, h, nil)
	_, err := client.Do(context.Background(), "card_update", nil, SourceMCP)
	e := decodeErr(t, err)
	if e.Code != CodeNotFound {
		t.Fatalf("code = %q, want %q", e.Code, CodeNotFound)
	}
	if e.Message != "card 9 not found" {
		t.Fatalf("message = %q, want the handler message", e.Message)
	}
}

func TestPlainErrorBecomesInternal(t *testing.T) {
	h := &fakeHandler{enabled: true, exec: func(string, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("secret path /Users/x/db failed")
	}}
	client, _ := startServer(t, h, nil)
	_, err := client.Do(context.Background(), "card_update", nil, SourceMCP)
	e := decodeErr(t, err)
	if e.Code != CodeInternalError {
		t.Fatalf("code = %q, want %q", e.Code, CodeInternalError)
	}
	if strings.Contains(e.Message, "secret path") {
		t.Fatalf("internal error must not leak the handler message: %q", e.Message)
	}
}

func TestProtocolMismatch(t *testing.T) {
	_, sock := startServer(t, &fakeHandler{enabled: true}, nil)
	resp := rawExchange(t, sock, `{"protocol_version":2,"operation":"card_update"}`)
	if !strings.Contains(resp, CodeProtocolMismatch) {
		t.Fatalf("resp = %s, want protocol_mismatch", resp)
	}
}

func TestMalformedFrame(t *testing.T) {
	_, sock := startServer(t, &fakeHandler{enabled: true}, nil)
	resp := rawExchange(t, sock, `this is not json`)
	if !strings.Contains(resp, CodeProtocolError) {
		t.Fatalf("resp = %s, want protocol_error", resp)
	}
}

func TestUnknownSourceRejected(t *testing.T) {
	client, _ := startServer(t, &fakeHandler{enabled: true}, nil)
	_, err := client.Do(context.Background(), "card_update", nil, "attacker")
	if got := decodeErr(t, err).Code; got != CodeProtocolError {
		t.Fatalf("code = %q, want %q", got, CodeProtocolError)
	}
}

func TestOversizeFrameRejected(t *testing.T) {
	_, sock := startServer(t, &fakeHandler{enabled: true}, nil)
	big := strings.Repeat("a", (1<<20)+16)
	resp := rawExchange(t, sock, `{"protocol_version":1,"operation":"card_update","arguments":{"x":"`+big+`"}}`)
	if !strings.Contains(resp, CodeProtocolError) {
		t.Fatalf("resp = %s, want protocol_error for oversize", resp)
	}
}
