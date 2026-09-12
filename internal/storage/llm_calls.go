package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// LlmCall is one attempt's sanitized metadata (docs/03 §3.3.1). It carries no
// endpoint, request/response body, image, key, or cost: those must never reach
// this table. Nullable columns are pointers; NULL means "not reported".
type LlmCall struct {
	BatchID          *int64
	Purpose          string
	AttemptNo        int
	ProviderID       string
	Protocol         string
	RequestedModel   string
	ActualModel      *string
	StartedAt        time.Time
	FinishedAt       time.Time
	Outcome          string
	ErrorKind        *string
	HTTPStatus       *int
	InputTokens      *int64
	OutputTokens     *int64
	CacheReadTokens  *int64
	CacheWriteTokens *int64
}

// LlmCallRepo is the typed access layer over llm_calls.
type LlmCallRepo struct {
	store *Store
}

// LlmCalls returns the repository bound to this store.
func (s *Store) LlmCalls() *LlmCallRepo {
	if s == nil {
		return nil
	}
	return &LlmCallRepo{store: s}
}

// Insert records one attempt. latency_ms is derived here from the timestamps
// so it can never disagree with them.
func (r *LlmCallRepo) Insert(ctx context.Context, call LlmCall) error {
	if call.Purpose == "" || call.ProviderID == "" || call.Protocol == "" ||
		call.RequestedModel == "" || call.Outcome == "" || call.AttemptNo < 1 {
		return newError(KindConstraint, fmt.Sprintf("llm call insert: missing required metadata (purpose %q, provider %q, protocol %q, model %q, outcome %q, attempt %d)",
			call.Purpose, call.ProviderID, call.Protocol, call.RequestedModel, call.Outcome, call.AttemptNo))
	}
	if call.FinishedAt.Before(call.StartedAt) {
		return newError(KindConstraint, fmt.Sprintf("llm call insert: finished (%s) before started (%s)", call.FinishedAt, call.StartedAt))
	}
	latencyMS := call.FinishedAt.Sub(call.StartedAt).Milliseconds()
	return r.store.Write(ctx, "llm call insert", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO llm_calls (
				batch_id, purpose, attempt_no, provider_id, protocol, requested_model,
				actual_model, started_at, finished_at, latency_ms, outcome, error_kind,
				http_status, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			call.BatchID, call.Purpose, call.AttemptNo, call.ProviderID, call.Protocol, call.RequestedModel,
			call.ActualModel, call.StartedAt.Unix(), call.FinishedAt.Unix(), latencyMS, call.Outcome, call.ErrorKind,
			call.HTTPStatus, call.InputTokens, call.OutputTokens, call.CacheReadTokens, call.CacheWriteTokens)
		return err
	})
}
