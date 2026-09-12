package ai

import (
	"context"
	"math/rand/v2"
	"time"
)

type Sleeper func(context.Context, time.Duration) error
type RandomDuration func(time.Duration) time.Duration

const maxRetryAfter = 30 * time.Second

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Sleep       Sleeper
	Jitter      RandomDuration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    8 * time.Second,
		Sleep:       sleepContext,
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
		result, err = p.provider.Generate(attemptCtx, request)
		if err == nil || !Retryable(err) || attempt == p.policy.MaxAttempts {
			return result, err
		}
		delay := p.policy.Jitter(p.delay(attempt))
		if retryAfter := RetryAfterOf(err); retryAfter > 0 {
			if retryAfter > maxRetryAfter {
				retryAfter = maxRetryAfter
			}
			delay = retryAfter
		}
		if sleepErr := p.policy.Sleep(ctx, delay); sleepErr != nil {
			return Result{}, canceledError(sleepErr)
		}
	}
	return result, err
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
