package ai

// Protocol is the closed set of provider wire protocols. Values are stable
// provider configuration identifiers; do not rename them once released.
type Protocol string

const (
	// ProtocolOpenAIChat is the OpenAI-compatible Chat Completions protocol.
	ProtocolOpenAIChat Protocol = "openai"
	// ProtocolOpenAIResponses is the OpenAI Responses protocol.
	ProtocolOpenAIResponses Protocol = "openai_responses"
	// ProtocolAnthropicMessages is the Anthropic Messages protocol.
	ProtocolAnthropicMessages Protocol = "anthropic"
	// ProtocolChatGPT is the OAuth-scoped ChatGPT plan Responses variant.
	ProtocolChatGPT Protocol = "chatgpt"
)

func (p Protocol) Valid() bool {
	return p == ProtocolOpenAIChat || p == ProtocolOpenAIResponses || p == ProtocolAnthropicMessages || p == ProtocolChatGPT
}
