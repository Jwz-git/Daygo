package app

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
)

func TestShutdownCancelsManualBatchesAndKeepsCards(t *testing.T) {
	b, _ := writerBackendWithStore(t, t.TempDir())
	seedTimelineDay(t, b, []domain.CardShell{{Start: "10:00 AM", End: "10:30 AM", Category: "Focus Work", Title: "original", Summary: "original"}})
	at := time.Date(2026, 9, 12, 11, 0, 0, 0, time.Local)
	seedFailedBatch(t, b, 2, at, 2)
	seedFailedBatch(t, b, 3, at.Add(time.Hour), 2)
	// Put the card's original succeeded batch in this day's range.
	if err := b.store().Write(context.Background(), "seed succeeded window", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE analysis_batches SET start_ts = ?, end_ts = ? WHERE id = 1`, at.Add(-time.Hour).Unix(), at.Add(-30*time.Minute).Unix())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	seedFailedBatch(t, b, 4, at.Add(2*time.Hour), 1)
	if err := b.RetryBatches([]int64{4}); err != nil {
		t.Fatal(err)
	}
	if err := b.ReprocessDay("2026-09-12"); err != nil {
		t.Fatal(err)
	}
	if err := b.store().Analysis().SetBatchStatus(context.Background(), 2, storage.BatchProcessing, "", "", at); err != nil {
		t.Fatal(err)
	}
	if err := b.store().Analysis().SetBatchStatus(context.Background(), 3, storage.BatchProcessing, "", "", at); err != nil {
		t.Fatal(err)
	}
	if err := b.store().Analysis().SetBatchStatus(context.Background(), 3, storage.BatchSucceeded, "", "", at); err != nil {
		t.Fatal(err)
	}
	b.shutdown()
	batches, err := b.store().Analysis().BatchesInRange(context.Background(), at.Add(-time.Hour), at.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 4 || batches[0].Status != storage.BatchSucceeded || batches[1].Status != storage.BatchFailed || batches[1].Attempts != storage.MaxBatchAttempts || batches[2].Status != storage.BatchSucceeded || batches[3].Status != storage.BatchFailed || batches[3].Attempts != storage.MaxBatchAttempts {
		t.Fatalf("batches: %+v", batches)
	}
	if n, err := b.store().Analysis().AdoptStaleProcessing(context.Background(), at); err != nil || n != 0 {
		t.Fatalf("adopt: %d %v", n, err)
	}
	if n, err := b.store().Analysis().RequeueFailed(context.Background(), at.Add(24*time.Hour), at.Add(24*time.Hour)); err != nil || n != 0 {
		t.Fatalf("requeue: %d %v", n, err)
	}
	day, err := b.GetTimelineDay("2026-09-12")
	if err != nil || len(day.Cards) != 1 || day.Cards[0].Title != "original" {
		t.Fatalf("cards: %+v %v", day, err)
	}
	assertAppCode(t, b.RetryBatches([]int64{2}), apperr.Canceled)
}

func TestShutdownCancelsAndWaitsForCardTask(t *testing.T) {
	b, _ := writerBackendWithStore(t, t.TempDir())
	ctx, finish, err := b.beginCardTask()
	if err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() { <-ctx.Done(); close(exited); finish() }()
	b.shutdown()
	select {
	case <-exited:
	default:
		t.Fatal("shutdown did not wait for task")
	}
	_, _, err = b.beginCardTask()
	assertAppCode(t, err, apperr.Canceled)
}

func TestShutdownWaitsForSchedulerBeforeRestoringManualQueue(t *testing.T) {
	b, _ := writerBackendWithStore(t, t.TempDir())
	at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	seedFailedBatch(t, b, 1, at, 1)
	if err := b.RetryBatches([]int64{1}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.analysisCancel = cancel
	b.analysisDone = make(chan struct{})
	writerErr := make(chan error, 1)
	go func() {
		defer close(b.analysisDone)
		<-ctx.Done()
		// Model a last successful write already committed when cancellation arrives.
		err := b.store().Analysis().SetBatchStatus(context.Background(), 1, storage.BatchProcessing, "", "", at)
		if err == nil {
			err = b.store().Analysis().SetBatchStatus(context.Background(), 1, storage.BatchSucceeded, "", "", at)
		}
		writerErr <- err
	}()
	b.shutdown()
	if err := <-writerErr; err != nil {
		t.Fatal(err)
	}
	rows, err := b.store().Analysis().BatchesInRange(context.Background(), at, at.Add(time.Hour))
	if err != nil || len(rows) != 1 || rows[0].Status != storage.BatchSucceeded {
		t.Fatalf("completed writer result: %+v %v", rows, err)
	}
}

func TestShutdownLeavesAutomaticBatchRecoveryIntact(t *testing.T) {
	b, _ := writerBackendWithStore(t, t.TempDir())
	at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	seedFailedBatch(t, b, 1, at, 1)
	// Repository-only requeue represents automatic work, not a manual binding.
	if _, err := b.store().Analysis().RequeueFailed(context.Background(), at.Add(time.Hour), at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := b.store().Analysis().SetBatchStatus(context.Background(), 1, storage.BatchProcessing, "", "", at); err != nil {
		t.Fatal(err)
	}
	b.shutdown()
	n, err := b.store().Analysis().AdoptStaleProcessing(context.Background(), at)
	if err != nil || n != 1 {
		t.Fatalf("automatic recovery: %d %v", n, err)
	}
}
