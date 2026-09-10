package ai

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestConnectionChecksTextImageAndStructuredOutput(t *testing.T) {
	calls := 0
	provider := providerFunc(func(_ context.Context, request Request) (Result, error) {
		calls++
		if request.Purpose != PurposeTest || request.MaxOutputTokens != connectionTestMaxTokens {
			t.Fatalf("request purpose=%q max tokens=%d", request.Purpose, request.MaxOutputTokens)
		}
		if len(request.Parts) != 2 || request.Parts[0].Kind() != PartText || request.Parts[1].Kind() != PartImage {
			t.Fatalf("parts = %#v", request.Parts)
		}
		if request.Parts[1].MediaType() != MediaPNG || len(request.Parts[1].Bytes()) == 0 {
			t.Fatal("embedded PNG was not sent")
		}
		if request.Output == nil || request.Output.Name != connectionTestSchemaName || !request.Output.Strict {
			t.Fatalf("output = %#v", request.Output)
		}
		return Result{
			Text:  `{"probeToken":"daygo-connection-v1","imageChoice":"github_octocat"}`,
			JSON:  json.RawMessage(`{"probeToken":"daygo-connection-v1","imageChoice":"github_octocat"}`),
			Model: "fixture-model",
		}, nil
	})

	started := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	times := []time.Time{started, started.Add(125 * time.Millisecond)}
	result, err := testConnection(context.Background(), provider, func() time.Time {
		value := times[0]
		times = times[1:]
		return value
	})
	if err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if calls != 1 || result.Model != "fixture-model" || result.Latency != 125*time.Millisecond {
		t.Fatalf("calls=%d result=%#v", calls, result)
	}
	want := []Capability{CapabilityText, CapabilityImage, CapabilityStructuredOutput}
	if len(result.Capabilities) != len(want) {
		t.Fatalf("capabilities = %#v", result.Capabilities)
	}
	for index := range want {
		if result.Capabilities[index] != want[index] {
			t.Fatalf("capabilities = %#v", result.Capabilities)
		}
	}
}

func TestConnectionRejectsIncorrectProbeResult(t *testing.T) {
	provider := providerFunc(func(context.Context, Request) (Result, error) {
		return Result{JSON: json.RawMessage(`{"probeToken":"wrong","imageChoice":"unknown"}`)}, nil
	})
	_, err := TestConnection(context.Background(), provider)
	if ErrorKindOf(err) != ErrorInvalidOutput {
		t.Fatalf("error = %v", err)
	}
}

func TestConnectionCapsCallerDeadline(t *testing.T) {
	provider := providerFunc(func(ctx context.Context, _ Request) (Result, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > connectionTestTimeout {
			t.Fatalf("deadline = %v, present=%v", deadline, ok)
		}
		return Result{JSON: json.RawMessage(`{"probeToken":"daygo-connection-v1","imageChoice":"github_octocat"}`)}, nil
	})
	if _, err := TestConnection(context.Background(), provider); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
}

type providerFunc func(context.Context, Request) (Result, error)

func (f providerFunc) Generate(ctx context.Context, request Request) (Result, error) {
	return f(ctx, request)
}
