package mcp

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Jwz-git/Daygo/internal/agentbridge"
	"github.com/Jwz-git/Daygo/internal/agentread"
)

type initResult struct {
	ProtocolVersion string            `json:"protocolVersion"`
	Capabilities    map[string]any    `json:"capabilities"`
	ServerInfo      map[string]string `json:"serverInfo"`
}

type toolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []toolInfo `json:"tools"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

func (s *Server) initialize(params json.RawMessage) initResult {
	version := defaultProtocolVersion
	if len(params) > 0 {
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if json.Unmarshal(params, &p) == nil && p.ProtocolVersion != "" {
			version = p.ProtocolVersion
		}
	}
	return initResult{
		ProtocolVersion: version,
		Capabilities:    map[string]any{"tools": map[string]any{}},
		ServerInfo:      map[string]string{"name": serverName, "version": serverVersion},
	}
}

func toolsList() toolsListResult {
	tools := make([]toolInfo, 0, len(catalog))
	for _, spec := range catalog {
		tools = append(tools, toolInfo{Name: spec.Name, Description: spec.Description, InputSchema: spec.InputSchema})
	}
	return toolsListResult{Tools: tools}
}

func (s *Server) toolsCall(ctx context.Context, req rpcRequest) rpcResponse {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if len(req.Params) == 0 || json.Unmarshal(req.Params, &p) != nil || p.Name == "" {
		return errorResponse(req.ID, codeInvalidParams, "tools/call requires a name")
	}
	spec, ok := toolByName(p.Name)
	if !ok {
		return errorResponse(req.ID, codeInvalidParams, "unknown tool: "+p.Name)
	}

	var text string
	var isErr bool
	if spec.Write {
		text, isErr = s.callWrite(ctx, spec.Name, p.Arguments)
	} else {
		text, isErr = s.callRead(ctx, spec.Name, p.Arguments)
	}
	return resultResponse(req.ID, toolCallResult{
		Content: []toolContent{{Type: "text", Text: text}},
		IsError: isErr,
	})
}

func (s *Server) callRead(ctx context.Context, name string, args json.RawMessage) (string, bool) {
	switch name {
	case toolTimeline:
		res, err := s.reader.Timeline(ctx, stringArg(args, "day"))
		return marshalOrFault(res, err)
	case toolCard:
		id, ferr := cardIDArg(args)
		if ferr != nil {
			return faultText(ferr), true
		}
		res, err := s.reader.Card(ctx, id)
		return marshalOrFault(res, err)
	case toolDaily:
		res, err := s.reader.Daily(ctx, stringArg(args, "day"))
		return marshalOrFault(res, err)
	case toolWeekly:
		res, err := s.reader.Weekly(ctx, stringArg(args, "weekStart"))
		return marshalOrFault(res, err)
	case toolCategories:
		res, err := s.reader.Categories(ctx)
		return marshalOrFault(res, err)
	}
	return errText(agentread.CodeInternal, "unknown read tool"), true
}

func (s *Server) callWrite(ctx context.Context, name string, args json.RawMessage) (string, bool) {
	var payload any
	if len(args) > 0 {
		payload = args // json.RawMessage re-emits verbatim through the client
	}
	data, err := s.writer.Do(ctx, name, payload, agentbridge.SourceMCP)
	if err != nil {
		return faultText(err), true
	}
	if len(data) == 0 {
		return `{"ok":true}`, false
	}
	return string(data), false
}

func marshalOrFault(res any, err error) (string, bool) {
	if err != nil {
		return faultText(err), true
	}
	b, mErr := json.Marshal(res)
	if mErr != nil {
		return errText(agentread.CodeInternal, "encode result failed"), true
	}
	return string(b), false
}

// faultText renders a read fault or a bridge error as a compact JSON error the
// model can parse. Both carry closed, privacy-safe codes/messages; anything
// else collapses to a generic internal error.
func faultText(err error) string {
	var rf *agentread.Fault
	if errors.As(err, &rf) {
		return errText(rf.Code, rf.Message)
	}
	var be *agentbridge.Error
	if errors.As(err, &be) {
		return errText(be.Code, be.Message)
	}
	return errText(agentread.CodeInternal, "internal error")
}

func errText(code, message string) string {
	b, _ := json.Marshal(struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{Error: struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: code, Message: message}})
	return string(b)
}

func stringArg(args json.RawMessage, field string) string {
	if len(args) == 0 {
		return ""
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(args, &m) != nil {
		return ""
	}
	raw, ok := m[field]
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return ""
}

func cardIDArg(args json.RawMessage) (int64, error) {
	var m struct {
		CardID *int64 `json:"cardId"`
	}
	if len(args) == 0 || json.Unmarshal(args, &m) != nil || m.CardID == nil {
		return 0, &agentread.Fault{Code: agentread.CodeInvalidArgument, Message: "cardId is required"}
	}
	return *m.CardID, nil
}
