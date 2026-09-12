package chat

import (
	"encoding/json"
	"fmt"

	"github.com/Jwz-git/Daygo/internal/ai"
)

// envelopeOutput is the structured-output contract every model reply in the
// agent loop must satisfy (docs/05 §5.12: protocol-neutral JSON mode, no
// native function calling). It is deliberately a single flat object — no
// oneOf/anyOf, whose strict-mode support varies across the three protocols —
// with the discrimination done here in Go.
var envelopeOutput = ai.OutputSchema{
	Name: "daygo_chat_reply",
	Schema: json.RawMessage(`{
		"type":"object",
		"properties":{
			"kind":{"type":"string","enum":["answer","tool"]},
			"answer":{"type":"string"},
			"tool":{"type":"string"},
			"arguments":{"type":"object"}
		},
		"required":["kind"],
		"additionalProperties":false
	}`),
	// Strict stays off: unknown-tool and malformed-argument replies must be
	// digested inside the loop as tool results, not rejected by Generate.
	Strict: false,
}

// envelopeKinds are the two legal values of kind.
const (
	envelopeKindAnswer = "answer"
	envelopeKindTool   = "tool"
)

// envelope is the decoded form of one model reply.
type envelope struct {
	Kind      string
	Answer    string
	Tool      string
	Arguments json.RawMessage
}

// decodeEnvelope parses one model reply against the envelope schema and the
// semantic rules the schema cannot carry: kind=answer requires a non-empty
// answer, kind=tool requires a tool name and an arguments object (defaulting
// to {} when omitted, which the `categories` tool legitimately uses).
func decodeEnvelope(text string) (envelope, error) {
	raw, err := ai.ParseStructuredOutput(text, envelopeOutput)
	if err != nil {
		return envelope{}, err
	}
	var decoded struct {
		Kind      string          `json:"kind"`
		Answer    string          `json:"answer"`
		Tool      string          `json:"tool"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return envelope{}, err
	}
	switch decoded.Kind {
	case envelopeKindAnswer:
		if decoded.Answer == "" {
			return envelope{}, fmt.Errorf("chat: envelope kind=answer with empty answer")
		}
	case envelopeKindTool:
		if decoded.Tool == "" {
			return envelope{}, fmt.Errorf("chat: envelope kind=tool with empty tool name")
		}
		if len(decoded.Arguments) == 0 {
			decoded.Arguments = json.RawMessage(`{}`)
		}
	default:
		return envelope{}, fmt.Errorf("chat: envelope kind=%q is not answer or tool", decoded.Kind)
	}
	return envelope{
		Kind:      decoded.Kind,
		Answer:    decoded.Answer,
		Tool:      decoded.Tool,
		Arguments: decoded.Arguments,
	}, nil
}

// validateToolArguments checks a call's arguments against its catalog schema.
// The empty-arguments case passes trivially: `categories` takes no parameters
// and its schema admits {}.
func validateToolArguments(name string, arguments json.RawMessage) error {
	spec, ok := toolByName(name)
	if !ok {
		return fmt.Errorf("chat: unknown tool %q", name)
	}
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	return ai.ValidateJSON(arguments, ai.OutputSchema{
		Name:   spec.Name + "_arguments",
		Schema: spec.Arguments,
	})
}

// envelopeCorrection is appended to the conversation when a reply fails to
// parse as an envelope, so the model gets one chance to fix its shape.
const envelopeCorrection = "\n\n（系统提示：上一次输出不符合要求的 JSON 格式。必须只返回一个 JSON 对象：" +
	`{"kind":"answer","answer":"最终回答"} 或 {"kind":"tool","tool":"工具名","arguments":{参数}}。）`
