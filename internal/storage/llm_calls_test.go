package storage

import (
	"context"
	"testing"
	"time"
)

func TestLlmCallRepoInsertRoundTrip(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.LlmCalls()
	ctx := context.Background()

	started := time.Unix(1700000000, 0)
	finished := started.Add(1500 * time.Millisecond)
	actualModel := "fixture-model"
	httpStatus := 200
	inputTokens, outputTokens := int64(120), int64(45)

	err := repo.Insert(ctx, LlmCall{
		Purpose:        "chat",
		AttemptNo:      1,
		ProviderID:     "fixture-provider",
		Protocol:       "openai",
		RequestedModel: "fixture-model",
		ActualModel:    &actualModel,
		StartedAt:      started,
		FinishedAt:     finished,
		Outcome:        "succeeded",
		HTTPStatus:     &httpStatus,
		InputTokens:    &inputTokens,
		OutputTokens:   &outputTokens,
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	var (
		purpose   string
		latencyMS int64
		outcome   string
		model     string
	)
	err = store.db.QueryRowContext(ctx,
		`SELECT purpose, latency_ms, outcome, actual_model FROM llm_calls WHERE provider_id = 'fixture-provider'`).
		Scan(&purpose, &latencyMS, &outcome, &model)
	if err != nil {
		t.Fatalf("read back llm_call: %v", err)
	}
	if purpose != "chat" || outcome != "succeeded" || model != "fixture-model" {
		t.Fatalf("llm_call = %q/%q/%q", purpose, outcome, model)
	}
	if latencyMS != 1500 {
		t.Fatalf("latency_ms = %d, want 1500 (derived from timestamps)", latencyMS)
	}
}

func TestLlmCallRepoInsertRejectsIncompleteMetadata(t *testing.T) {
	store := openWriter(t, newDir(t))
	repo := store.LlmCalls()
	ctx := context.Background()

	err := repo.Insert(ctx, LlmCall{
		ProviderID:     "fixture-provider",
		Protocol:       "openai",
		RequestedModel: "fixture-model",
		StartedAt:      time.Unix(1700000000, 0),
		FinishedAt:     time.Unix(1700000001, 0),
		Outcome:        "succeeded",
	})
	if err == nil {
		t.Fatal("Insert without purpose must fail")
	}
	if kind, ok := KindOf(err); !ok || kind != KindConstraint {
		t.Fatalf("error kind = %v, want constraint", err)
	}

	err = repo.Insert(ctx, LlmCall{
		Purpose:        "chat",
		AttemptNo:      1,
		ProviderID:     "fixture-provider",
		Protocol:       "openai",
		RequestedModel: "fixture-model",
		StartedAt:      time.Unix(1700000001, 0),
		FinishedAt:     time.Unix(1700000000, 0),
		Outcome:        "succeeded",
	})
	if err == nil {
		t.Fatal("Insert with finished before started must fail")
	}
	if kind, ok := KindOf(err); !ok || kind != KindConstraint {
		t.Fatalf("error kind = %v, want constraint", err)
	}

	// No row may have landed from either rejected call.
	var count int
	if err := store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM llm_calls").Scan(&count); err != nil {
		t.Fatalf("count llm_calls: %v", err)
	}
	if count != 0 {
		t.Fatalf("llm_calls rows = %d, want 0", count)
	}
}
