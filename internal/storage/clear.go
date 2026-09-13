package storage

import (
	"context"
	"database/sql"
)

// ClearHistoryData wipes recorded and analyzed history in one transaction:
// frames, pending captures, batches, observations, cards, LLM call metadata,
// journal entries, day goals, and chat conversations. Configuration —
// app_settings, providers, categories — survives; it is the user's setup,
// not history. This backs the test-only one-click reset binding.
func (s *Store) ClearHistoryData(ctx context.Context) (int64, error) {
	var cleared int64
	err := s.Write(ctx, "clear history data", func(ctx context.Context, tx *sql.Tx) error {
		for _, table := range []string{
			"timeline_cards",
			"observations",
			"batch_screenshots",
			"analysis_batches",
			"screenshots",
			"pending_captures",
			"llm_calls",
			"journal_entries",
			"day_goal_categories",
			"day_goals",
			"chat_conversations",
		} {
			result, err := tx.ExecContext(ctx, "DELETE FROM "+table)
			if err != nil {
				return wrap("clear history data: "+table, err)
			}
			if n, err := result.RowsAffected(); err == nil {
				cleared += n
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return cleared, nil
}
