// Package agentbridge is the agent write channel (docs/05 §5.9.2): a 0600 Unix
// socket that accepts one JSON request per connection, runs the six write
// operations through a host-supplied Handler, and returns one JSON response.
// It is transport and protocol only — the Handler (internal/app) owns the
// shared service path, so external writes take the same validation, capture
// guard, and events as binding-layer writes.
package agentbridge

import (
	"encoding/json"
	"slices"
)

// ProtocolVersion is the agent.sock protocol version. It is frozen after the
// first public release; a breaking change needs a new version with dual-end
// compatibility (docs/05 §5.10.1).
const ProtocolVersion = 1

// Operations is the closed write set (docs/05 §5.9.2), fixed order. The server
// rejects anything outside it with unknown_operation before the Handler runs,
// so the Handler never sees an operation the contract does not name.
var Operations = []string{
	"category_add",
	"category_update",
	"category_remove",
	"card_update",
	"card_delete",
	"goal_set",
}

// Error codes, the closed set of docs/05 §5.9.2. They are physically distinct
// from the binding-layer apperr codes (§5.7): the Handler maps its apperr
// errors onto these before they cross the socket.
const (
	CodeProtocolError    = "protocol_error"
	CodeProtocolMismatch = "protocol_mismatch"
	CodeEditsDisabled    = "edits_disabled"
	CodeInvalidArgument  = "invalid_argument"
	CodeNotFound         = "not_found"
	CodeUnknownOperation = "unknown_operation"
	CodeInternalError    = "internal_error"
)

// Source labels the write's origin for the agent-writes.log audit
// (docs/decisions/agent-mcp-transport.md §5). It is advisory, a closed set,
// and defaults to SourceSocket when the request omits it — not a security
// boundary (that is agentEditsEnabled, checked server-side).
const (
	SourceSocket = "agent.sock"
	SourceMCP    = "mcp"
	SourceCLI    = "cli"
)

func sourceAllowed(s string) bool {
	switch s {
	case SourceSocket, SourceMCP, SourceCLI:
		return true
	default:
		return false
	}
}

func operationKnown(op string) bool {
	return slices.Contains(Operations, op)
}

// Request is the one-line request frame (docs/05 §5.9.2). Source is optional.
type Request struct {
	ProtocolVersion int             `json:"protocol_version"`
	Operation       string          `json:"operation"`
	Arguments       json.RawMessage `json:"arguments,omitempty"`
	Source          string          `json:"source,omitempty"`
}

// Response is the one-line response frame. Exactly one of Data / Error is set:
// Data on ok, Error otherwise.
type Response struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error *ResponseError  `json:"error,omitempty"`
}

// ResponseError is the failure body of a Response.
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error is the typed failure a Handler returns so the server can map it onto a
// closed response code. A plain (non-*Error) failure becomes internal_error
// with a generic message, so Handler internals never leak across the socket.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Errorf builds a Handler Error with one of the closed codes.
func Errorf(code, message string) *Error { return &Error{Code: code, Message: message} }
