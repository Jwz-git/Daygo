package ai

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRequestValidationAndImageCopy(t *testing.T) {
	bytes := []byte{1, 2, 3}
	image, err := ImagePart(MediaPNG, bytes)
	if err != nil {
		t.Fatalf("ImagePart: %v", err)
	}
	bytes[0] = 9
	if got := image.Bytes()[0]; got != 1 {
		t.Fatalf("image bytes mutated through caller: %d", got)
	}

	request := Request{Parts: []Part{TextPart("describe"), image}}
	if err := request.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	tooMany := make([]Part, MaxImages+1)
	for i := range tooMany {
		tooMany[i], err = ImagePart(MediaJPEG, []byte{1})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := (Request{Parts: tooMany}).Validate(); ErrorKindOf(err) != ErrorInvalidRequest {
		t.Fatalf("too many images error = %v", err)
	}
}

func TestParseStructuredOutput(t *testing.T) {
	output := OutputSchema{Name: "item", Schema: []byte(`{
		"type":"object",
		"properties":{"name":{"type":"string"}},
		"required":["name"],
		"additionalProperties":false
	}`), Strict: true}

	got, err := ParseStructuredOutput("Result:\n```json\n{\"name\":\"anonymous\"}\n```", output)
	if err != nil {
		t.Fatalf("ParseStructuredOutput: %v", err)
	}
	if string(got) != `{"name":"anonymous"}` {
		t.Fatalf("result = %s", got)
	}
	repaired, err := ParseStructuredOutput("Result: {\"name\":\"anonymous\",}", output)
	if err != nil || string(repaired) != `{"name":"anonymous"}` {
		t.Fatalf("trailing comma result=%s error=%v", repaired, err)
	}
	if _, err := ParseStructuredOutput(`{"name":"anonymous"`, output); ErrorKindOf(err) != ErrorInvalidOutput {
		t.Fatalf("truncated output error = %v", err)
	}
	if _, err := ParseStructuredOutput(`{"name":12}`, output); ErrorKindOf(err) != ErrorInvalidOutput {
		t.Fatalf("schema mismatch error = %v", err)
	}
}

// ValidateJSON is the schema half of ParseStructuredOutput; callers holding
// already-extracted JSON (chat tool arguments) depend on it rejecting the same
// shapes with the same error kinds.
func TestValidateJSON(t *testing.T) {
	output := OutputSchema{Name: "args", Schema: []byte(`{
		"type":"object",
		"properties":{
			"day":{"type":"string","pattern":"^\\d{4}-\\d{2}-\\d{2}$"},
			"limit":{"type":"integer","minimum":1}
		},
		"required":["day"],
		"additionalProperties":false
	}`)}

	if err := ValidateJSON([]byte(`{"day":"2026-09-12"}`), output); err != nil {
		t.Fatalf("ValidateJSON valid: %v", err)
	}
	if err := ValidateJSON([]byte(`{"day":"2026-09-12","limit":3}`), output); err != nil {
		t.Fatalf("ValidateJSON optional field: %v", err)
	}
	if err := ValidateJSON([]byte(`{"day":"today"}`), output); ErrorKindOf(err) != ErrorInvalidOutput {
		t.Fatalf("pattern mismatch error = %v", err)
	}
	if err := ValidateJSON([]byte(`{"limit":3}`), output); ErrorKindOf(err) != ErrorInvalidOutput {
		t.Fatalf("missing required error = %v", err)
	}
	if err := ValidateJSON([]byte(`{"day":"2026-09-12","extra":true}`), output); ErrorKindOf(err) != ErrorInvalidOutput {
		t.Fatalf("unknown field error = %v", err)
	}
	if err := ValidateJSON([]byte(`{"day":`), output); ErrorKindOf(err) != ErrorInvalidOutput {
		t.Fatalf("invalid JSON error = %v", err)
	}
}

func TestRetryStopsOnCancellation(t *testing.T) {
	provider := &sequenceProvider{errors: []error{
		NewError(ErrorUnavailable, "temporary", 503, nil),
		NewError(ErrorUnavailable, "temporary", 503, nil),
	}}
	ctx, cancel := context.WithCancel(context.Background())
	policy := DefaultRetryPolicy()
	policy.Sleep = func(context.Context, time.Duration) error {
		cancel()
		return context.Canceled
	}
	_, err := WithRetry(provider, policy).Generate(ctx, Request{})
	if ErrorKindOf(err) != ErrorCanceled {
		t.Fatalf("error kind = %s, error = %v", ErrorKindOf(err), err)
	}
	if provider.calls != 1 {
		t.Fatalf("calls = %d, want 1", provider.calls)
	}
}

func TestAttemptObserverReceivesRedactedMetadataForEachRetry(t *testing.T) {
	provider := &sequenceProvider{
		results: []Result{{}, {Text: "ok", Model: "actual-model"}},
		errors:  []error{NewError(ErrorUnavailable, "temporary", 503, nil), nil},
	}
	var attempts []Attempt
	observer := AttemptObserverFunc(func(_ context.Context, attempt Attempt) {
		attempts = append(attempts, attempt)
	})
	observed := WithAttemptObserver(provider, "provider-id", ProtocolOpenAIChat, "requested-model", observer)
	policy := DefaultRetryPolicy()
	policy.Sleep = func(context.Context, time.Duration) error { return nil }
	batchID := int64(42)
	ctx := WithAttemptMetadata(context.Background(), AttemptMetadata{BatchID: &batchID})
	result, err := WithRetry(observed, policy).Generate(ctx, Request{Purpose: PurposeCards})
	if err != nil || result.Text != "ok" {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	if len(attempts) != 2 || attempts[0].AttemptNo != 1 || attempts[1].AttemptNo != 2 {
		t.Fatalf("attempts = %#v", attempts)
	}
	if attempts[0].Outcome != "failed" || attempts[0].ErrorKind != ErrorUnavailable || attempts[0].HTTPStatus != 503 {
		t.Fatalf("failed attempt = %#v", attempts[0])
	}
	if attempts[1].Outcome != "succeeded" || attempts[1].ActualModel != "actual-model" ||
		attempts[1].BatchID == nil || *attempts[1].BatchID != batchID {
		t.Fatalf("successful attempt = %#v", attempts[1])
	}
}

func TestRetryHonorsCappedRetryAfter(t *testing.T) {
	providerErr := NewError(ErrorRateLimited, "temporary", 429, nil)
	providerErr.RetryAfter = time.Minute
	provider := &sequenceProvider{
		results: []Result{{}, {Text: "ok"}},
		errors:  []error{providerErr, nil},
	}
	var slept time.Duration
	policy := DefaultRetryPolicy()
	policy.Sleep = func(_ context.Context, delay time.Duration) error {
		slept = delay
		return nil
	}
	policy.Jitter = func(time.Duration) time.Duration { return 0 }
	result, err := WithRetry(provider, policy).Generate(context.Background(), Request{})
	if err != nil || result.Text != "ok" {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	if slept != 30*time.Second {
		t.Fatalf("slept = %s, want 30s", slept)
	}
}

type sequenceProvider struct {
	mu      sync.Mutex
	calls   int
	results []Result
	errors  []error
}

func (p *sequenceProvider) Generate(context.Context, Request) (Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	index := p.calls
	p.calls++
	var result Result
	var err error
	if index < len(p.results) {
		result = p.results[index]
	}
	if index < len(p.errors) {
		err = p.errors[index]
	}
	return result, err
}

// A hung attempt must be cut off by the per-attempt deadline: without it the
// provider never returns, no error is classified, and the pipeline blocks
// forever on a deadline-less scheduler context.
func TestRetryTimesOutHungAttempt(t *testing.T) {
	provider := &hungProvider{}
	policy := DefaultRetryPolicy()
	policy.RequestTimeout = 20 * time.Millisecond
	policy.Sleep = func(context.Context, time.Duration) error { return nil }
	policy.Jitter = func(time.Duration) time.Duration { return 0 }

	_, err := WithRetry(provider, policy).Generate(context.Background(), Request{})
	if ErrorKindOf(err) != ErrorTimeout {
		t.Fatalf("error kind = %s, error = %v", ErrorKindOf(err), err)
	}
	if provider.calls != 3 {
		t.Fatalf("calls = %d, want 3 (timeout is retryable)", provider.calls)
	}
}

type hungProvider struct {
	calls int
}

func (p *hungProvider) Generate(ctx context.Context, _ Request) (Result, error) {
	p.calls++
	<-ctx.Done()
	return Result{}, ctx.Err()
}

func TestErrorClassificationPreservesContext(t *testing.T) {
	err := NewError(ErrorUnavailable, "request failed", 503, context.DeadlineExceeded)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("error chain lost deadline")
	}
	if HTTPStatusOf(err) != 503 {
		t.Fatalf("HTTP status = %d", HTTPStatusOf(err))
	}
}

// The per-request image cap: 0 and out-of-range values fall back to the
// MaxImages default, valid ones clamp nothing.
func TestClampMaxImages(t *testing.T) {
	cases := []struct {
		configured int
		want       int
	}{
		{0, MaxImages},
		{-3, MaxImages},
		{1, 1},
		{5, 5},
		{20, 20},
		{21, MaxImages},
	}
	for _, c := range cases {
		if got := ClampMaxImages(c.configured); got != c.want {
			t.Fatalf("ClampMaxImages(%d) = %d, want %d", c.configured, got, c.want)
		}
	}
}

// Validate enforces the request's own cap, not the global constant: a
// provider with a lower gateway limit must reject an over-sized request
// before it leaves the machine.
func TestValidateEnforcesRequestImageCap(t *testing.T) {
	mk := func(n int) Request {
		parts := make([]Part, 0, n+1)
		parts = append(parts, TextPart("describe"))
		for range n {
			part, err := ImagePart(MediaPNG, []byte{1})
			if err != nil {
				t.Fatal(err)
			}
			parts = append(parts, part)
		}
		return Request{Parts: parts}
	}

	if err := mk(5).Validate(); err != nil {
		t.Fatalf("default cap rejects 5 images: %v", err)
	}
	five := mk(5)
	five.MaxImages = 4
	if ErrorKindOf(five.Validate()) != ErrorInvalidRequest {
		t.Fatal("cap 4 accepted 5 images")
	}
	five.MaxImages = 5
	if err := five.Validate(); err != nil {
		t.Fatalf("cap 5 rejected 5 images: %v", err)
	}
}
