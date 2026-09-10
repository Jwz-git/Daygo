package ai

import (
	"context"
	_ "embed"
	"encoding/json"
	"time"
)

const (
	connectionTestTimeout    = 30 * time.Second
	connectionTestMaxTokens  = 128
	connectionTestSchemaName = "daygo_connection_test"
	connectionTestProbeToken = "daygo-connection-v1"
)

//go:embed assets/connection-test.png
var connectionTestImage []byte

var connectionTestSchema = json.RawMessage(`{
	"type":"object",
	"properties":{
		"probeToken":{"type":"string","const":"daygo-connection-v1"},
		"imageChoice":{"type":"string","enum":["github_octocat","unknown"]}
	},
	"required":["probeToken","imageChoice"],
	"additionalProperties":false
}`)

const connectionTestPrompt = `This is an automated Daygo provider capability check.
Return probeToken exactly as "daygo-connection-v1".
Inspect the attached image and set imageChoice to "github_octocat" only when it shows the GitHub Octocat cat silhouette inside a dark circle; otherwise set it to "unknown".
Return only the requested structured result.`

type Capability string

const (
	CapabilityText             Capability = "text"
	CapabilityImage            Capability = "image"
	CapabilityStructuredOutput Capability = "structured_output"
)

type ConnectionTestResult struct {
	Model        string
	Latency      time.Duration
	Capabilities []Capability
}

type connectionProbe struct {
	ProbeToken  string `json:"probeToken"`
	ImageChoice string `json:"imageChoice"`
}

func TestConnection(ctx context.Context, provider Provider) (ConnectionTestResult, error) {
	return testConnection(ctx, provider, time.Now)
}

func testConnection(ctx context.Context, provider Provider, now func() time.Time) (ConnectionTestResult, error) {
	if provider == nil {
		return ConnectionTestResult{}, NewError(ErrorInvalidRequest, "provider is required", 0, nil)
	}
	ctx, cancel := context.WithTimeout(ctx, connectionTestTimeout)
	defer cancel()

	image, err := ImagePart(MediaPNG, connectionTestImage)
	if err != nil {
		return ConnectionTestResult{}, err
	}
	request := Request{
		Purpose: PurposeTest,
		Parts:   []Part{TextPart(connectionTestPrompt), image},
		Output: &OutputSchema{
			Name:   connectionTestSchemaName,
			Schema: append(json.RawMessage(nil), connectionTestSchema...),
			Strict: true,
		},
		MaxOutputTokens: connectionTestMaxTokens,
	}
	started := now()
	result, err := provider.Generate(ctx, request)
	finished := now()
	if err != nil {
		return ConnectionTestResult{}, err
	}
	var probe connectionProbe
	if err := json.Unmarshal(result.JSON, &probe); err != nil {
		return ConnectionTestResult{}, NewError(ErrorInvalidOutput, "provider test returned invalid structured output", 0, err)
	}
	if probe.ProbeToken != connectionTestProbeToken || probe.ImageChoice != "github_octocat" {
		return ConnectionTestResult{}, NewError(ErrorInvalidOutput, "provider test did not verify text and image capabilities", 0, nil)
	}
	return ConnectionTestResult{
		Model:   result.Model,
		Latency: finished.Sub(started),
		Capabilities: []Capability{
			CapabilityText,
			CapabilityImage,
			CapabilityStructuredOutput,
		},
	}, nil
}
