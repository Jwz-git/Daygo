package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/factory"
	"github.com/Jwz-git/Daygo/internal/analysis"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"github.com/Jwz-git/Daygo/internal/settings"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// stagingFrameSource reads single-frame staging JPEGs by segment path.
//
// Provisional: it bypasses platform.Media, whose segment container and
// encoding format are still open decisions (#7/#8). Today the recorder
// commits each frame as its own staging JPEG with frame_index 0, so this is
// the only shape on disk. When Media lands, this adapter is replaced and the
// analysis package is untouched.
type stagingFrameSource struct {
	root string
}

func (s stagingFrameSource) FrameBytes(_ context.Context, segmentPath string, frameIndex int) ([]byte, error) {
	if frameIndex != 0 {
		return nil, fmt.Errorf("staging frames are single-frame; index %d is invalid", frameIndex)
	}
	return os.ReadFile(filepath.Join(s.root, filepath.FromSlash(segmentPath)))
}

// analysisChainSource builds the provider chain from the routing setting and
// the keychain, mirroring chat's rebuildChain wiring: factory client →
// attempt observer (llm_calls audit) → retry, wrapped in an ai.Chain.
type analysisChainSource struct {
	backend *Backend
}

func (a analysisChainSource) AnalysisChain(ctx context.Context) (*ai.Chain, error) {
	repo := a.backend.store().Providers()
	routing, err := a.backend.loadRouting(ctx, repo)
	if err != nil {
		return nil, err
	}
	sink := attemptSink{repo: a.backend.store().LlmCalls()}

	entries := make([]ai.ChainEntry, 0, len(routing.Chain))
	for _, id := range routing.Chain {
		row, err := repo.Get(ctx, id)
		if err != nil {
			// A routing slot whose provider vanished between read and here is
			// skipped; the rest of the chain still serves the batch.
			continue
		}
		secret, err := a.backend.secrets.Get(ctx, id)
		if err != nil && !secrets.IsNotFound(err) {
			return nil, err
		}
		provider, err := factory.NewClient(nil, factory.Config{
			Protocol: ai.Protocol(row.Protocol),
			Endpoint: row.Endpoint,
			Model:    row.Model,
			Secret:   secret,
		})
		if err != nil {
			continue
		}
		provider = ai.WithAttemptObserver(provider, row.ID, ai.Protocol(row.Protocol), row.Model,
			ai.AttemptObserverFunc(func(_ context.Context, attempt ai.Attempt) {
				dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				sink.RecordAttempt(dbCtx, attempt)
			}))
		provider = ai.WithRetry(provider, ai.DefaultRetryPolicy())
		entries = append(entries, ai.ChainEntry{ID: row.ID, Provider: provider})
	}
	return ai.NewChain(entries, 0), nil
}

// analysisLanguage reads the model output-language setting; empty means
// follow the interface language.
func analysisLanguage(b *Backend) func(context.Context) string {
	return func(ctx context.Context) string {
		snapshot, err := settings.New(b.store().Settings()).Load(ctx)
		if err != nil {
			return ""
		}
		return snapshot.OutputLanguage
	}
}

// startAnalysis wires and runs the analysis pipeline for a read-write
// instance. A read-only second instance does not analyze: it holds neither
// the write lock nor the capture ownership the pipeline's writes require.
func startAnalysis(ctx context.Context, b *Backend, store *storage.Store, recordingsRoot string) (*analysis.Service, error) {
	service, err := analysis.New(analysis.Config{
		Store:      store.Analysis(),
		Cards:      store.Cards(),
		Categories: store.Categories(),
		Providers:  analysisChainSource{backend: b},
		Media:      stagingFrameSource{root: recordingsRoot},
		Language:   analysisLanguage(b),
		OnCardsCommitted: func(days []string) {
			for _, day := range days {
				b.emitTimelineInvalidation(day)
			}
		},
		OnBatchFailed: func(batch storage.Batch, kind, note string) {
			b.emitter.Emit(EventBatchFailed, TimelineFailureDTO{
				BatchIDs:  []int64{batch.ID},
				StartTs:   batch.Start.Unix(),
				EndTs:     batch.End.Unix(),
				Kind:      kind,
				Message:   note,
				Retryable: true,
			})
		},
	})
	if err != nil {
		return nil, err
	}
	go service.Run(ctx)
	return service, nil
}
