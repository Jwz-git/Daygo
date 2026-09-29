package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Jwz-git/Daygo/internal/agentbridge"
	"github.com/Jwz-git/Daygo/internal/agentcli"
	"github.com/Jwz-git/Daygo/internal/agentread"
)

type fakeReader struct {
	cardErr error
}

func (f fakeReader) Timeline(_ context.Context, day string) (agentread.TimelineResult, error) {
	return agentread.TimelineResult{SchemaVersion: 1, Day: day, Cards: []agentread.TimelineCard{}}, nil
}

func (f fakeReader) Card(_ context.Context, id int64) (agentread.CardResult, error) {
	if f.cardErr != nil {
		return agentread.CardResult{}, f.cardErr
	}
	return agentread.CardResult{SchemaVersion: 1, ID: id, Day: "2026-09-20"}, nil
}

func (f fakeReader) Daily(_ context.Context, day string) (agentread.DailyResult, error) {
	return agentread.DailyResult{SchemaVersion: 1, Day: day}, nil
}

func (f fakeReader) Weekly(_ context.Context, weekStart string) (agentread.WeeklyResult, error) {
	return agentread.WeeklyResult{SchemaVersion: 1, WeekStart: weekStart}, nil
}

func (f fakeReader) Categories(_ context.Context) (agentread.CategoriesResult, error) {
	return agentread.CategoriesResult{SchemaVersion: 1, Categories: []agentread.Category{}}, nil
}

type fakeWriter struct {
	gotOp     string
	gotSource string
	gotArgs   json.RawMessage
	resp      json.RawMessage
	err       error
}

func (w *fakeWriter) Do(_ context.Context, op string, args any, source string) (json.RawMessage, error) {
	w.gotOp = op
	w.gotSource = source
	if args != nil {
		b, _ := json.Marshal(args)
		w.gotArgs = b
	}
	return w.resp, w.err
}

// roundtrip runs one request line through a fresh server and returns the raw
// output line (empty for a notification).
func roundtrip(t *testing.T, reader Reader, writer Writer, request string) string {
	t.Helper()
	var out bytes.Buffer
	srv := NewServer(reader, writer, strings.NewReader(request+"\n"), &out)
	if err := srv.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return strings.TrimSpace(out.String())
}

func decodeResp(t *testing.T, line string) rpcResponse {
	t.Helper()
	var resp rpcResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("decode response %q: %v", line, err)
	}
	return resp
}

func TestInitializeEchoesProtocolVersion(t *testing.T) {
	line := roundtrip(t, fakeReader{}, &fakeWriter{},
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"c","version":"1"}}}`)
	if !strings.Contains(line, `"protocolVersion":"2025-06-18"`) {
		t.Fatalf("initialize should echo the client version: %s", line)
	}
	if !strings.Contains(line, `"name":"daygo"`) || !strings.Contains(line, `"tools":{}`) {
		t.Fatalf("initialize missing serverInfo/capabilities: %s", line)
	}
}

func TestInitializeDefaultsProtocolVersion(t *testing.T) {
	line := roundtrip(t, fakeReader{}, &fakeWriter{},
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if !strings.Contains(line, `"protocolVersion":"`+defaultProtocolVersion+`"`) {
		t.Fatalf("initialize should default the version: %s", line)
	}
}

func TestToolsListShape(t *testing.T) {
	line := roundtrip(t, fakeReader{}, &fakeWriter{},
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	var resp struct {
		Result toolsListResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	if len(resp.Result.Tools) != len(catalog) {
		t.Fatalf("tools = %d, want %d", len(resp.Result.Tools), len(catalog))
	}
	for _, tool := range resp.Result.Tools {
		if tool.Name == "" || len(tool.InputSchema) == 0 {
			t.Fatalf("tool %q missing name/inputSchema", tool.Name)
		}
	}
}

// TestToolFacesAreSameSource is the §5.10.3 gate: the write tools equal the
// bridge's operation set (order included), and every read tool is a known CLI
// read command that is not the CLI-only status or the deferred search.
func TestToolFacesAreSameSource(t *testing.T) {
	var reads, writes []string
	for _, spec := range catalog {
		if spec.Write {
			writes = append(writes, spec.Name)
		} else {
			reads = append(reads, spec.Name)
		}
	}
	if !slices.Equal(writes, agentbridge.Operations) {
		t.Fatalf("write tools %v != bridge operations %v", writes, agentbridge.Operations)
	}
	for _, r := range reads {
		if !slices.Contains(agentcli.Commands, r) {
			t.Errorf("read tool %q is not a CLI read command", r)
		}
		if r == "status" || r == "search" {
			t.Errorf("read tool %q must not be exposed over MCP", r)
		}
	}
	if len(reads) != 5 {
		t.Fatalf("read tools = %v, want the five §5.9.3 reads", reads)
	}
}

func TestToolsCallReadReturnsEnvelope(t *testing.T) {
	line := roundtrip(t, fakeReader{}, &fakeWriter{},
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"timeline","arguments":{"day":"2026-09-20"}}}`)
	if !strings.Contains(line, `schema_version`) {
		t.Fatalf("read tool result should carry the schema_version envelope: %s", line)
	}
	if strings.Contains(line, `"isError":true`) {
		t.Fatalf("successful read must not be an error result: %s", line)
	}
}

func TestToolsCallReadFaultIsErrorResult(t *testing.T) {
	reader := fakeReader{cardErr: &agentread.Fault{Code: agentread.CodeNotFound, Message: "card not found"}}
	line := roundtrip(t, reader, &fakeWriter{},
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"card","arguments":{"cardId":9999}}}`)
	if !strings.Contains(line, `"isError":true`) {
		t.Fatalf("a read fault must set isError: %s", line)
	}
	if !strings.Contains(line, agentread.CodeNotFound) {
		t.Fatalf("error result should carry the fault code: %s", line)
	}
}

func TestToolsCallWriteGoesThroughBridgeAsMCP(t *testing.T) {
	writer := &fakeWriter{resp: json.RawMessage(`{"ok":true}`)}
	line := roundtrip(t, fakeReader{}, writer,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"card_update","arguments":{"cardId":1,"title":"x"}}}`)
	if writer.gotOp != "card_update" {
		t.Fatalf("bridge op = %q, want card_update", writer.gotOp)
	}
	if writer.gotSource != agentbridge.SourceMCP {
		t.Fatalf("bridge source = %q, want %q", writer.gotSource, agentbridge.SourceMCP)
	}
	if !strings.Contains(string(writer.gotArgs), `"cardId":1`) {
		t.Fatalf("bridge args not forwarded: %s", writer.gotArgs)
	}
	if strings.Contains(line, `"isError":true`) {
		t.Fatalf("successful write must not be an error result: %s", line)
	}
}

func TestToolsCallWriteErrorMapsCode(t *testing.T) {
	writer := &fakeWriter{err: agentbridge.Errorf(agentbridge.CodeEditsDisabled, "agent edits are disabled")}
	line := roundtrip(t, fakeReader{}, writer,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"card_delete","arguments":{"cardId":1}}}`)
	if !strings.Contains(line, `"isError":true`) || !strings.Contains(line, agentbridge.CodeEditsDisabled) {
		t.Fatalf("write error should be an isError result carrying the code: %s", line)
	}
}

func TestUnknownMethodIsMethodNotFound(t *testing.T) {
	line := roundtrip(t, fakeReader{}, &fakeWriter{},
		`{"jsonrpc":"2.0","id":7,"method":"resources/list"}`)
	if got := decodeResp(t, line).Error; got == nil || got.Code != codeMethodNotFound {
		t.Fatalf("resp error = %+v, want method not found", got)
	}
}

func TestUnknownToolIsInvalidParams(t *testing.T) {
	line := roundtrip(t, fakeReader{}, &fakeWriter{},
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"frobnicate"}}`)
	if got := decodeResp(t, line).Error; got == nil || got.Code != codeInvalidParams {
		t.Fatalf("resp error = %+v, want invalid params", got)
	}
}

func TestNotificationGetsNoReply(t *testing.T) {
	line := roundtrip(t, fakeReader{}, &fakeWriter{},
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if line != "" {
		t.Fatalf("notification must get no reply, got %q", line)
	}
}

func TestMalformedLineIsParseError(t *testing.T) {
	line := roundtrip(t, fakeReader{}, &fakeWriter{}, `this is not json`)
	resp := decodeResp(t, line)
	if resp.Error == nil || resp.Error.Code != codeParseError {
		t.Fatalf("resp error = %+v, want parse error", resp.Error)
	}
	if string(resp.ID) != "null" {
		t.Fatalf("parse error id = %s, want null", resp.ID)
	}
}
