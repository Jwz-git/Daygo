package storage

import (
	"context"
	"database/sql"
	"time"
)

// TokenUsageSample contains sanitized usage only. Input includes cache tokens;
// OpenAI already includes them, whereas Anthropic reports them separately.
type TokenUsageSample struct {
	StartedAt    int64
	InputTokens  int64
	OutputTokens int64
	Calls        int64
	UnknownCalls int64
}

func (r *LlmCallRepo) TokenUsage(ctx context.Context, start, end time.Time) ([]TokenUsageSample, error) {
	result := make([]TokenUsageSample, 0)
	err := r.store.Read(ctx, "token usage", func(ctx context.Context, db *sql.Tx) error {
		rows, err := db.QueryContext(ctx, `SELECT started_at,
   SUM(MAX(COALESCE(input_tokens, 0), 0) + CASE WHEN protocol = 'anthropic' THEN MAX(COALESCE(cache_read_tokens, 0), 0) + MAX(COALESCE(cache_write_tokens, 0), 0) ELSE 0 END),
   SUM(MAX(COALESCE(output_tokens, 0), 0)), COUNT(*),
   SUM(CASE WHEN input_tokens IS NULL OR output_tokens IS NULL THEN 1 ELSE 0 END)
   FROM llm_calls WHERE started_at >= ? AND started_at < ? GROUP BY started_at ORDER BY started_at`, start.Unix(), end.Unix())
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var sample TokenUsageSample
			if err := rows.Scan(&sample.StartedAt, &sample.InputTokens, &sample.OutputTokens, &sample.Calls, &sample.UnknownCalls); err != nil {
				return err
			}
			result = append(result, sample)
		}
		return rows.Err()
	})
	return result, err
}
