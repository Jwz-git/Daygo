package ai

import (
	"context"
	"math/rand/v2"
	"time"
)

type Sleeper func(context.Context, time.Duration) error
type RandomDuration func(time.Duration) time.Duration

const maxRetryAfter = 30 * time.Second

// DefaultRequestTimeout bounds one provider attempt. The provider clients
// otherwise inherit a deadline-less context from the scheduler, so a peer
// that accepts the connection but never answers would block the pipeline
// forever — no error to classify, no retry, no fallback.
const DefaultRequestTimeout = 2 * time.Minute

type RetryPolicy struct {
	MaxAttempts    int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	RequestTimeout time.Duration
	Sleep          Sleeper
	Jitter         RandomDuration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:    3,
		BaseDelay:      500 * time.Millisecond,
		MaxDelay:       8 * time.Second,
		RequestTimeout: DefaultRequestTimeout,
		Sleep:          sleepContext,
		Jitter: func(limit time.Duration) time.Duration {
			if limit <= 0 {
				return 0
			}
			return rand.N(limit)
		},
	}
}

type retryProvider struct {
	provider Provider
	policy   RetryPolicy
}

func WithRetry(provider Provider, policy RetryPolicy) Provider {
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = 1
	}
	if policy.BaseDelay <= 0 {
		policy.BaseDelay = 500 * time.Millisecond
	}
	if policy.MaxDelay <= 0 {
		policy.MaxDelay = 8 * time.Second
	}
	if policy.RequestTimeout <= 0 {
		policy.RequestTimeout = DefaultRequestTimeout
	}
	if policy.Sleep == nil {
		policy.Sleep = sleepContext
	}
	if policy.Jitter == nil {
		policy.Jitter = func(limit time.Duration) time.Duration { return limit }
	}
	return &retryProvider{provider: provider, policy: policy}
}

func (p *retryProvider) Generate(ctx context.Context, request Request) (Result, error) {
	var result Result
	var err error
	for attempt := 1; attempt <= p.policy.MaxAttempts; attempt++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Result{}, canceledError(ctxErr)
		}
		attemptCtx := context.WithValue(ctx, attemptNumberKey{}, attempt)
		// The per-attempt deadline fires inside the provider call as a plain
		// context.DeadlineExceeded, which classifies as ErrorTimeout —
		// retryable and chain-fallback-eligible like any other timeout.
		attemptCtx, cancel := context.WithTimeout(attemptCtx, p.policy.RequestTimeout)
		result, err = p.provider.Generate(attemptCtx, request)
		cancel()
		if err == nil || !Retryable(err) || attempt == p.policy.MaxAttempts {
			return result, err
		}
		delay := p.policy.Jitter(p.delay(attempt))
		if retryAfter := RetryAfterOf(err); retryAfter > 0 {
			if retryAfter > maxRetryAfter {
				retryAfter = maxRetryAfter
			}
			delay = retryAfter
		} else if ErrorKindOf(err) == ErrorRateLimited {
			delay = p.rateLimitDelay(attempt)
			if p.policy.Jitter != nil {
				delay += p.policy.Jitter(2 * time.Second)
			}
			if delay > maxRetryAfter {
				delay = maxRetryAfter
			}
		}
		if sleepErr := p.policy.Sleep(ctx, delay); sleepErr != nil {
			return Result{}, canceledError(sleepErr)
		}
	}
	return result, err
}

// DefaultRateLimitDelay is the base backoff for ErrorRateLimited when no
// Retry-After header is provided. A per-minute token rate limit needs 15–30s
// to replenish tokens, matching Dayflow's longBackoff strategy.
const DefaultRateLimitDelay = 15 * time.Second

func (p *retryProvider) rateLimitDelay(attempt int) time.Duration {
	delay := DefaultRateLimitDelay
	for i := 1; i < attempt && delay < maxRetryAfter; i++ {
		delay *= 2
		if delay > maxRetryAfter {
			delay = maxRetryAfter
		}
	}
	return delay
}

func (p *retryProvider) delay(attempt int) time.Duration {
	delay := p.policy.BaseDelay
	for i := 1; i < attempt && delay < p.policy.MaxDelay; i++ {
		delay *= 2
		if delay > p.policy.MaxDelay {
			delay = p.policy.MaxDelay
		}
	}
	return delay
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Fallback routing lives in chain.go (Chain). The former single-secondary
// sticky model (WithFallback/RouteState) was replaced by the ordered cycling
// chain per decisions/providers-fallback-chain.

func canceledError(err error) error {
	if ErrorKindOf(err) == ErrorTimeout {
		return NewError(ErrorTimeout, "request deadline exceeded", 0, err)
	}
	return NewError(ErrorCanceled, "request canceled", 0, err)
}
