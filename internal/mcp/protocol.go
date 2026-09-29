// Package mcp is the daygo MCP server (docs/05 §5.9.3,
// docs/decisions/agent-mcp-transport.md): a stdio JSON-RPC 2.0 server the
// `daygo mcp` subprocess runs. Reads go through the read-only database
// (internal/agentread); writes go through the agent.sock write channel
// (internal/agentbridge) — the same two paths the CLI uses, so no third query
// or write semantics appears. Messages are newline-delimited JSON with no
// embedded newlines (MCP stdio transport).
package mcp

import "encoding/json"

// jsonRPCVersion is the only supported JSON-RPC version.
const jsonRPCVersion = "2.0"

// defaultProtocolVersion is advertised when a client's initialize omits one.
// When the client proposes a version the server echoes it back, which is what
// every current client accepts; the constant is only the fallback.
const defaultProtocolVersion = "2025-06-18"

// serverName / serverVersion identify this server in the initialize result.
const (
	serverName    = "daygo"
	serverVersion = "0.1.0"
)

// JSON-RPC error codes (MCP uses the standard set).
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// rpcRequest is one incoming message. A request carries id; a notification
// omits it (and gets no response). Params and ID stay raw so ID echoes back
// verbatim and Params decodes per method.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (r rpcRequest) isNotification() bool { return len(r.ID) == 0 }

// rpcResponse is one outgoing reply. Exactly one of Result / Error is set.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func resultResponse(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{JSONRPC: jsonRPCVersion, ID: normalizeID(id), Result: result}
}

func errorResponse(id json.RawMessage, code int, message string) rpcResponse {
	return rpcResponse{JSONRPC: jsonRPCVersion, ID: normalizeID(id), Error: &rpcError{Code: code, Message: message}}
}

// normalizeID keeps a present id verbatim and renders a missing one as JSON
// null, which a response must carry even when the id is unknown.
func normalizeID(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return json.RawMessage("null")
	}
	return id
}
