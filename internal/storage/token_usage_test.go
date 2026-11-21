package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenUsageBoundsAndCacheSemantics(t *testing.T) {
	s := openWriter(t, newDir(t))
	ctx := context.Background()
	start := time.Unix(1700000000, 0)
	input, output, read, write := int64(100), int64(20), int64(30), int64(10)
	for i, protocol := range []string{"openai", "anthropic", "openai", "openai"} {
		at := start.Add(time.Duration(i) * time.Second)
		call := LlmCall{Purpose: "chat", AttemptNo: 1, ProviderID: "fixture", Protocol: protocol, RequestedModel: "fixture", StartedAt: at, FinishedAt: at, Outcome: "failed"}
		if i != 2 {
			call.InputTokens = &input
			call.OutputTokens = &output
			call.CacheReadTokens = &read
			call.CacheWriteTokens = &write
		}
		if err := s.LlmCalls().Insert(ctx, call); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.LlmCalls().TokenUsage(ctx, start, start.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].InputTokens != 100 || rows[1].InputTokens != 140 || rows[1].OutputTokens != 20 || rows[2].UnknownCalls != 1 || rows[2].Calls != 1 {
		t.Fatalf("usage = %+v", rows)
	}
	empty, err := s.LlmCalls().TokenUsage(ctx, start.Add(-time.Second), start)
	if err != nil || len(empty) != 0 {
		t.Fatalf("left boundary: %+v %v", empty, err)
	}
}

func TestMigrateV25FixtureAddsUsageTimeIndex(t *testing.T) {
	dir := newDir(t)
	copyFile(t, filepath.Join("testdata", "v25-token-usage-source.db"), filepath.Join(dir, DatabaseFileName))
	store := openWriter(t, dir)
	ctx := t.Context()
	if got := userVersionOf(t, store); got < 26 {
		t.Fatalf("schema version after reopen = %d", got)
	}
	var column string
	if err := store.db.QueryRowContext(ctx, "SELECT name FROM pragma_index_info('idx_llm_calls_started_at')").Scan(&column); err != nil || column != "started_at" {
		t.Fatalf("time index column = %q, %v", column, err)
	}
	samples, err := store.LlmCalls().TokenUsage(ctx, time.Unix(100, 0), time.Unix(101, 0))
	if err != nil || len(samples) != 1 || samples[0].InputTokens != 42 || samples[0].OutputTokens != 7 {
		t.Fatalf("pre-index usage after migration = %+v err=%v", samples, err)
	}
}
