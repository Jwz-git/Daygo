package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/Jwz-git/Daygo/internal/agentread"
)

// Reader is the read face the MCP tools call — the subset of agentread.Reader
// the tool set needs (status is CLI-only). *agentread.Reader satisfies it; a
// fake satisfies it in tests.
type Reader interface {
	Timeline(ctx context.Context, day string) (agentread.TimelineResult, error)
	Card(ctx context.Context, id int64) (agentread.CardResult, error)
	Daily(ctx context.Context, day string) (agentread.DailyResult, error)
	Weekly(ctx context.Context, weekStart string) (agentread.WeeklyResult, error)
	Categories(ctx context.Context) (agentread.CategoriesResult, error)
}

// Writer is the write face — one method matching agentbridge.Client.Do, so
// writes cross the socket and never touch the database from this process.
type Writer interface {
	Do(ctx context.Context, op string, args any, source string) (json.RawMessage, error)
}

// Server is one MCP stdio session. It reads newline-delimited JSON-RPC from in
// and writes replies to out.
type Server struct {
	reader Reader
	writer Writer
	in     io.Reader
	out    io.Writer
	outMu  sync.Mutex
}

// NewServer builds a server over the given streams.
func NewServer(reader Reader, writer Writer, in io.Reader, out io.Writer) *Server {
	return &Server{reader: reader, writer: writer, in: in, out: out}
}

// Run serves until in reaches EOF or ctx is cancelled. Malformed lines get a
// parse-error reply; notifications get none.
func (s *Server) Run(ctx context.Context) error {
	r := bufio.NewReader(s.in)
	for {
		line, err := r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			s.handleLine(ctx, line)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func (s *Server) handleLine(ctx context.Context, line []byte) {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		s.write(errorResponse(nil, codeParseError, "parse error"))
		return
	}
	if req.isNotification() {
		return // notifications/initialized and the rest have no reply
	}
	s.write(s.dispatch(ctx, req))
}

func (s *Server) dispatch(ctx context.Context, req rpcRequest) rpcResponse {
	switch req.Method {
	case "initialize":
		return resultResponse(req.ID, s.initialize(req.Params))
	case "ping":
		return resultResponse(req.ID, struct{}{})
	case "tools/list":
		return resultResponse(req.ID, toolsList())
	case "tools/call":
		return s.toolsCall(ctx, req)
	default:
		return errorResponse(req.ID, codeMethodNotFound, "method not found: "+req.Method)
	}
}

func (s *Server) write(resp rpcResponse) {
	b, err := json.Marshal(resp)
	if err != nil {
		return
	}
	s.outMu.Lock()
	defer s.outMu.Unlock()
	_, _ = s.out.Write(append(b, '\n'))
}
