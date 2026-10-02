package app

import (
	"context"
	"log"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// Called under timelineTaskMu, after the queue transaction commits. The rows
// returned by the repository are their pre-requeue states.
func (b *Backend) rememberManualBatches(rows []storage.Batch) {
	if b.manualBatches == nil {
		b.manualBatches = make(map[int64]storage.Batch)
	}
	for _, row := range rows {
		b.manualBatches[row.ID] = row
	}
}

func (b *Backend) beginCardTask() (context.Context, func(), error) {
	b.timelineTaskMu.Lock()
	defer b.timelineTaskMu.Unlock()
	if b.timelineClosing {
		return nil, nil, apperr.E(apperr.Canceled, "analysis is shutting down", nil)
	}
	if b.timelineTaskCtx == nil {
		b.timelineTaskCtx, b.timelineTaskCancel = context.WithCancel(context.Background())
	}
	ctx, cancel := context.WithTimeout(b.timelineTaskCtx, cardRegenerationTimeout)
	b.timelineTaskWG.Add(1)
	return ctx, func() { cancel(); b.timelineTaskWG.Done() }, nil
}

// Real termination only: soft quit/window close never calls this. Stop writers
// before restoring queued manual work, so no late provider response can race
// the restoration. Successful results stay committed. Crash adoption stays as-is.
func (b *Backend) stopTimelineTasks() {
	b.timelineTaskMu.Lock()
	b.timelineClosing = true
	if b.timelineTaskCancel != nil {
		b.timelineTaskCancel()
	}
	if b.analysisCancel != nil {
		b.analysisCancel()
	}
	done := b.analysisDone
	rows := make([]storage.Batch, 0, len(b.manualBatches))
	for _, row := range b.manualBatches {
		rows = append(rows, row)
	}
	b.timelineTaskMu.Unlock()
	b.timelineTaskWG.Wait()
	if done != nil {
		<-done
	}
	if len(rows) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timelineTimeout)
	defer cancel()
	if err := b.store().Analysis().CancelManualBatches(ctx, rows, b.clock.Now()); err != nil {
		log.Printf("analysis: cancel manual batches on shutdown: %v", err)
	}
}
