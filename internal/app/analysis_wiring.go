package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/ai/factory"
	"github.com/Jwz-git/Daygo/internal/analysis"
	"github.com/Jwz-git/Daygo/internal/platform"
	platformfactory "github.com/Jwz-git/Daygo/internal/platform/factory"
	"github.com/Jwz-git/Daygo/internal/platform/secrets"
	"github.com/Jwz-git/Daygo/internal/settings"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// mediaFrameSource adapts platform.Media to analysis.FrameSource.
type mediaFrameSource struct {
	media platform.Media
}

func (s mediaFrameSource) FrameBytes(ctx context.Context, segmentPath string, frameIndex int) ([]byte, error) {
	if s.media == nil {
		return nil, fmt.Errorf("media frame source: media is unavailable")
	}
	return s.media.DecodeFrame(ctx, platform.DecodeRequest{
		SegmentPath: segmentPath,
		FrameIndex:  frameIndex,
	})
}

// retryableFailureKind reports whether a failed batch is worth retrying. An
// exhausted attempt count means the cooldown/requeue loop already gave up;
// auth failures do not heal on their own (the user must fix the key), so the
// UI should say "needs attention" rather than "will retry". no_provider is
// the same story: nothing retries its way out of an empty chain.
func retryableFailureKind(kind string, attempts int) bool {
	switch kind {
	case "auth", "invalid_request", "no_provider":
		return false
	}
	return attempts < storage.MaxBatchAttempts
}

// analysisChainSource builds the provider chain from the routing setting and
// the keychain, mirroring chat's rebuildChain wiring: factory client →
// attempt observer (llm_calls audit) → retry, wrapped in an ai.Chain.
type analysisChainSource struct {
	backend *Backend
}

func (a analysisChainSource) AnalysisChain(ctx context.Context) (*ai.Chain, error) {
	repo := a.backend.store().Providers()
	routing, err := a.backend.loadRouting(ctx)
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

// ImageCap is the per-request image limit the analysis grouping uses: the
// minimum non-default cap across the routing chain, so a group sized for one
// provider never exceeds a fallback's gateway limit. With no configured cap
// anywhere this stays 0 (the ai.MaxImages default).
func (a analysisChainSource) ImageCap(ctx context.Context) int {
	repo := a.backend.store().Providers()
	routing, err := a.backend.loadRouting(ctx)
	if err != nil {
		return 0
	}
	cap := 0
	for _, id := range routing.Chain {
		row, err := repo.Get(ctx, id)
		if err != nil {
			continue
		}
		if row.MaxImages > 0 && (cap == 0 || row.MaxImages < cap) {
			cap = row.MaxImages
		}
	}
	return cap
}

// analysisLanguage reads the model output-language setting; empty means
// follow the interface language, which we resolve at this edge so the
// analysis prompt carries a concrete BCP 47 tag rather than the weak
// "match the user's message" fallback (see resolveOutputLanguage).
func analysisLanguage(b *Backend) func(context.Context) string {
	return func(ctx context.Context) string {
		snapshot, err := settings.New(b.store().Settings()).Load(ctx)
		if err != nil {
			return b.interfaceLanguage(settings.Snapshot{})
		}
		return resolveOutputLanguage(snapshot)
	}
}

// startAnalysis wires and runs the analysis pipeline for a read-write
// instance. A read-only second instance does not analyze: it holds neither
// the write lock nor the capture ownership the pipeline's writes require.
func startAnalysis(ctx context.Context, b *Backend, store *storage.Store, recordingsRoot string) (*analysis.Service, error) {
	media, _ := b.mediaSnapshot()
	if media == nil {
		media = platformfactory.NewMedia(recordingsRoot)
	}
	service, err := analysis.New(analysis.Config{
		Store:      store.Analysis(),
		Cards:      store.Cards(),
		Categories: store.Categories(),
		Providers:  analysisChainSource{backend: b},
		Media:       mediaFrameSource{media: media},
		Language:    analysisLanguage(b),
		BatchPacing: analysis.DefaultBatchPacing,
		Workers:     1,
		// The service's zone must be the storage layer's zone: it prefilters
		// card windows here while ReplaceCardsInRange derives start_ts/end_ts
		// and day with store.location(). Two zones would split one decision.
		Location: store.Location(),
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
				Retryable: retryableFailureKind(kind, batch.Attempts),
			})
		},
	})
	if err != nil {
		return nil, err
	}
	go service.Run(ctx)
	return service, nil
}
