package ai

import (
	"context"
	"time"
)

type Attempt struct {
	BatchID          *int64
	Purpose          Purpose
	AttemptNo        int
	ProviderID       string
	Protocol         Protocol
	RequestedModel   string
	ActualModel      string
	StartedAt        time.Time
	FinishedAt       time.Time
	Outcome          string
	ErrorKind        ErrorKind
	HTTPStatus       int
	InputTokens      *int64
	OutputTokens     *int64
	CacheReadTokens  *int64
	CacheWriteTokens *int64
}

type AttemptObserver interface {
	Observe(context.Context, Attempt)
}

type AttemptObserverFunc func(context.Context, Attempt)

func (f AttemptObserverFunc) Observe(ctx context.Context, attempt Attempt) {
	f(ctx, attempt)
}

type AttemptMetadata struct {
	BatchID *int64
}

type attemptMetadataKey struct{}
type attemptNumberKey struct{}

func WithAttemptMetadata(ctx context.Context, metadata AttemptMetadata) context.Context {
	return context.WithValue(ctx, attemptMetadataKey{}, metadata)
}

type observedProvider struct {
	provider       Provider
	providerID     string
	protocol       Protocol
	requestedModel string
	observer       AttemptObserver
	now            func() time.Time
}

func WithAttemptObserver(provider Provider, providerID string, protocol Protocol, requestedModel string, observer AttemptObserver) Provider {
	return withAttemptObserverClock(provider, providerID, protocol, requestedModel, observer, time.Now)
}

func withAttemptObserverClock(provider Provider, providerID string, protocol Protocol, requestedModel string, observer AttemptObserver, now func() time.Time) Provider {
	if observer == nil {
		return provider
	}
	if now == nil {
		now = time.Now
	}
	return &observedProvider{
		provider:       provider,
		providerID:     providerID,
		protocol:       protocol,
		requestedModel: requestedModel,
		observer:       observer,
		now:            now,
	}
}

func (p *observedProvider) Generate(ctx context.Context, request Request) (Result, error) {
	started := p.now()
	result, err := p.provider.Generate(ctx, request)
	attempt := Attempt{
		Purpose:          request.Purpose,
		AttemptNo:        attemptNumber(ctx),
		ProviderID:       p.providerID,
		Protocol:         p.protocol,
		RequestedModel:   p.requestedModel,
		ActualModel:      result.Model,
		StartedAt:        started,
		FinishedAt:       p.now(),
		Outcome:          "succeeded",
		InputTokens:      result.Usage.InputTokens,
		OutputTokens:     result.Usage.OutputTokens,
		CacheReadTokens:  result.Usage.CacheReadTokens,
		CacheWriteTokens: result.Usage.CacheWriteTokens,
	}
	if metadata, ok := ctx.Value(attemptMetadataKey{}).(AttemptMetadata); ok {
		attempt.BatchID = metadata.BatchID
	}
	if err != nil {
		attempt.Outcome = "failed"
		attempt.ErrorKind = ErrorKindOf(err)
		attempt.HTTPStatus = HTTPStatusOf(err)
	}
	p.observer.Observe(ctx, attempt)
	return result, err
}

func attemptNumber(ctx context.Context) int {
	if number, ok := ctx.Value(attemptNumberKey{}).(int); ok && number > 0 {
		return number
	}
	return 1
}
