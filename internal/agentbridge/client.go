package agentbridge

import (
	"context"
	"encoding/json"
	"net"
	"time"
)

// Client is a one-shot caller of the agent write channel. Each Do opens a
// connection, sends one request, reads one response, and closes — matching the
// server's one-exchange-per-connection frame (docs/05 §5.9.2). It is the write
// path the MCP server uses; it never touches the database directly.
type Client struct {
	path string
}

// NewClient targets the socket at path (agent.sock in the support dir, or next
// to a DAYGO_DB override).
func NewClient(path string) *Client { return &Client{path: path} }

// Do sends one operation and returns its data payload. args is marshaled as the
// request arguments (nil → omitted); source labels the write for the audit. A
// failure response becomes a *Error carrying the server's closed code.
func (c *Client) Do(ctx context.Context, op string, args any, source string) (json.RawMessage, error) {
	var rawArgs json.RawMessage
	if args != nil {
		b, err := json.Marshal(args)
		if err != nil {
			return nil, Errorf(CodeInvalidArgument, "arguments are not encodable")
		}
		rawArgs = b
	}
	reqLine, err := json.Marshal(Request{
		ProtocolVersion: ProtocolVersion,
		Operation:       op,
		Arguments:       rawArgs,
		Source:          source,
	})
	if err != nil {
		return nil, Errorf(CodeInternalError, "encode request failed")
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", c.path)
	if err != nil {
		return nil, Errorf(CodeInternalError, "connect to agent socket failed")
	}
	defer func() { _ = conn.Close() }()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(connDeadline))
	}

	if _, err := conn.Write(append(reqLine, '\n')); err != nil {
		return nil, Errorf(CodeInternalError, "write request failed")
	}
	line, err := readFrame(conn, maxFrameBytes)
	if err != nil {
		return nil, Errorf(CodeProtocolError, "read response failed")
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, Errorf(CodeProtocolError, "response is not valid JSON")
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, &Error{Code: resp.Error.Code, Message: resp.Error.Message}
		}
		return nil, Errorf(CodeInternalError, "unknown failure")
	}
	return resp.Data, nil
}
