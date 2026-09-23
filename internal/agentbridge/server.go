package agentbridge

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

const (
	// maxFrameBytes bounds a request line (docs/05 §5.9.2: 1 MB each way).
	maxFrameBytes = 1 << 20
	// connDeadline bounds one request/response exchange so a stalled client
	// cannot hold a handler goroutine open indefinitely.
	connDeadline = 10 * time.Second
)

// Handler runs the write operations behind the socket. The host (internal/app)
// implements it against the shared service path so external writes take the
// same validation, capture guard, and events as binding-layer writes.
type Handler interface {
	// EditsEnabled reports the server-side agentEditsEnabled gate. The server
	// checks it on every request; a client's own check is never trusted
	// (docs/05 §5.9.2).
	EditsEnabled(ctx context.Context) (bool, error)
	// Execute runs one operation from Operations. args is the raw request
	// arguments (may be nil). A *Error maps onto its code; any other error
	// becomes internal_error so Handler internals never cross the socket.
	Execute(ctx context.Context, op string, args json.RawMessage) (json.RawMessage, error)
}

// Server serves the agent write channel on a Unix socket. The zero value is not
// usable; construct with NewServer.
type Server struct {
	handler Handler
	audit   io.Writer
	auditMu sync.Mutex

	listener  net.Listener
	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewServer builds a server. audit receives one JSON line per successful write
// (nil discards); it is the agent-writes.log sink the host opens 0600.
func NewServer(handler Handler, audit io.Writer) *Server {
	return &Server{handler: handler, audit: audit, done: make(chan struct{})}
}

// Start listens on path and serves in the background until ctx is cancelled or
// Close is called. It removes a stale socket first (the single-writer invariant
// makes this instance the owner) and forces 0600 before accepting.
func (s *Server) Start(ctx context.Context, path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = l.Close()
		return err
	}
	s.listener = l

	s.wg.Go(func() { s.watch(ctx) })
	s.wg.Go(func() { s.acceptLoop(ctx) })
	return nil
}

// Close stops accepting, waits for in-flight requests, and closes the listener
// (which unlinks the socket file).
func (s *Server) Close() error {
	err := s.closeListener()
	s.wg.Wait()
	return err
}

func (s *Server) closeListener() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.done)
		if s.listener != nil {
			err = s.listener.Close()
		}
	})
	return err
}

func (s *Server) watch(ctx context.Context) {
	select {
	case <-ctx.Done():
		_ = s.closeListener()
	case <-s.done:
	}
}

func (s *Server) acceptLoop(ctx context.Context) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}
		s.wg.Go(func() { s.handleConn(ctx, conn) })
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(connDeadline))

	resp, source, op, ok := s.process(ctx, conn)
	// Audit before replying: the write already committed inside Execute, so the
	// record must survive even if the client never reads the response.
	if ok {
		s.appendAudit(source, op)
	}
	_ = writeResponse(conn, resp)
}

// process reads and validates one request, then runs it. It returns the
// response plus the audit source/operation and whether the write succeeded, so
// the audit line is appended only after a successful write.
func (s *Server) process(ctx context.Context, conn net.Conn) (Response, string, string, bool) {
	line, err := readFrame(conn, maxFrameBytes)
	if err != nil {
		return failure(CodeProtocolError, "malformed request frame"), "", "", false
	}
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return failure(CodeProtocolError, "request is not valid JSON"), "", "", false
	}
	if req.ProtocolVersion != ProtocolVersion {
		return failure(CodeProtocolMismatch, "unsupported protocol_version"), "", "", false
	}
	source := req.Source
	if source == "" {
		source = SourceSocket
	}
	if !sourceAllowed(source) {
		return failure(CodeProtocolError, "unknown source"), "", "", false
	}
	if !operationKnown(req.Operation) {
		return failure(CodeUnknownOperation, "unknown operation"), "", "", false
	}
	enabled, err := s.handler.EditsEnabled(ctx)
	if err != nil {
		return failure(CodeInternalError, "internal error"), "", "", false
	}
	if !enabled {
		return failure(CodeEditsDisabled, "agent edits are disabled"), "", "", false
	}
	data, err := s.handler.Execute(ctx, req.Operation, req.Arguments)
	if err != nil {
		return failureFromErr(err), "", "", false
	}
	return Response{OK: true, Data: data}, source, req.Operation, true
}

func (s *Server) appendAudit(source, op string) {
	if s.audit == nil {
		return
	}
	b, err := json.Marshal(auditEntry{
		Ts:        time.Now().UTC().Format(time.RFC3339),
		Source:    source,
		Operation: op,
	})
	if err != nil {
		return
	}
	s.auditMu.Lock()
	defer s.auditMu.Unlock()
	_, _ = s.audit.Write(append(b, '\n'))
}

// auditEntry is one agent-writes.log line. It carries no arguments — only the
// operation, its source, and a timestamp — so screen content, titles, and
// category text never reach the audit log (docs/07).
type auditEntry struct {
	Ts        string `json:"ts"`
	Source    string `json:"source"`
	Operation string `json:"operation"`
}

func failure(code, message string) Response {
	return Response{OK: false, Error: &ResponseError{Code: code, Message: message}}
}

func failureFromErr(err error) Response {
	var e *Error
	if errors.As(err, &e) {
		return failure(e.Code, e.Message)
	}
	return failure(CodeInternalError, "internal error")
}

// readFrame reads one newline-delimited frame, rejecting anything larger than
// limit bytes. A frame without a trailing newline (client closed after writing)
// is still accepted.
func readFrame(r io.Reader, limit int) ([]byte, error) {
	br := bufio.NewReader(io.LimitReader(r, int64(limit)+1))
	data, err := br.ReadBytes('\n')
	if len(data) > limit {
		return nil, errors.New("frame too large")
	}
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(data) == 0 {
		return nil, io.EOF
	}
	return data, nil
}

func writeResponse(w io.Writer, resp Response) error {
	b, err := json.Marshal(resp)
	if err != nil {
		b, _ = json.Marshal(failure(CodeInternalError, "internal error"))
	}
	_, err = w.Write(append(b, '\n'))
	return err
}
