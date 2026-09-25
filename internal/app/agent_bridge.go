package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Jwz-git/Daygo/internal/agentbridge"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/chat"
)

// agentWriteHandler adapts the socket protocol to the same executor used by
// in-app chat and the binding write methods. The bridge owns the external gate.
type agentWriteHandler struct{ backend *Backend }

func (h agentWriteHandler) EditsEnabled(ctx context.Context) (bool, error) {
	access, err := h.backend.settingsAccess()
	if err != nil {
		return false, err
	}
	snapshot, err := access.Load(ctx)
	if err != nil {
		return false, err
	}
	canWrite, _ := h.backend.instanceOwnership()
	return snapshot.AgentEditsEnabled && canWrite, nil
}

func (h agentWriteHandler) Execute(ctx context.Context, op string, args json.RawMessage) (json.RawMessage, error) {
	if err := chat.ValidateWriteArguments(op, args); err != nil {
		return nil, agentbridge.Errorf(agentbridge.CodeInvalidArgument, "invalid write arguments")
	}
	result := (chatToolExecutor{backend: h.backend}).Execute(ctx, chat.ToolCall{Tool: op, Arguments: args})
	if !result.Err {
		return result.Data, nil
	}
	var failure struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(result.Data, &failure) != nil {
		return nil, agentbridge.Errorf(agentbridge.CodeInternalError, "internal error")
	}
	switch failure.Error.Code {
	case string(apperr.InvalidArgument):
		return nil, agentbridge.Errorf(agentbridge.CodeInvalidArgument, failure.Error.Message)
	case string(apperr.NotFound):
		return nil, agentbridge.Errorf(agentbridge.CodeNotFound, failure.Error.Message)
	default:
		return nil, agentbridge.Errorf(agentbridge.CodeInternalError, "internal error")
	}
}

// AgentConnectionDTO tells the settings page how a local MCP / CLI client
// reaches this instance (docs/05 §5.9). ExecutablePath is empty when the OS
// cannot report it; the page then falls back to the bare `daygo` name.
type AgentConnectionDTO struct {
	ExecutablePath string `json:"executablePath"`
	SocketActive   bool   `json:"socketActive"`
}

// GetAgentConnection reports the executable an MCP client should launch with
// the `mcp` argument, and whether this instance is serving agent.sock. It reads
// no settings: the write gate stays agentEditsEnabled, checked per request.
func (b *Backend) GetAgentConnection() (AgentConnectionDTO, error) {
	dto := AgentConnectionDTO{SocketActive: b.agentSocketActive.Load()}
	if path, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		dto.ExecutablePath = path
	}
	return dto, nil
}
