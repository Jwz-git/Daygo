package analysis

import (
	"context"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// This anonymous provider needs more than the old 4096-token allowance for
// complete output. The fixture models total generation, including reasoning;
// it never accepts a partial response just to make card parsing succeed.
type budgetProvider struct {
	calls      []ai.Request
	correction bool
}

func (p *budgetProvider) Generate(_ context.Context, request ai.Request) (ai.Result, error) {
	p.calls = append(p.calls, request)
	if request.MaxOutputTokens < 8192 {
		return ai.Result{}, ai.NewError(ai.ErrorInvalidOutput, "anonymous output exhausted its budget", 0, nil)
	}
	end := "10:15 AM"
	if p.correction && len(p.calls) == 1 {
		end = "9:59 AM"
	}
	return ai.Result{Text: `{"cards":[{"start":"10:00 AM","end":"` + end + `","category":"Coding","subcategory":"","title":"Anonymous task","summary":"Anonymous work.","detailed_summary":"","appSites":[],"distractions":[],"activityPoints":[]}]}`}, nil
}

func TestCardsOutputBudgetSupportsCompleteGenerationAndCorrection(t *testing.T) {
	for _, mode := range []string{"fresh", "ongoing", "scoped"} {
		for _, correction := range []bool{false, true} {
			name := mode + "/initial"
			if correction {
				name = mode + "/correction"
			}
			t.Run(name, func(t *testing.T) {
				h := newHarness(t, nil)
				provider := &budgetProvider{correction: correction}
				chain := ai.NewChain([]ai.ChainEntry{{ID: "anonymous", Provider: provider}}, 3)
				start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.Local)
				end := start.Add(15 * time.Minute)
				categories := []domain.Category{{Name: "Coding"}}
				var shells []domain.CardShell
				var err error
				if mode == "scoped" {
					shells, err = h.service.generateScopedCards(context.Background(), chain,
						domain.TimelineCard{Start: "10:00 AM", End: "10:15 AM"}, start, end, nil, nil, categories)
				} else {
					shells, _, err = h.service.generateCards(context.Background(), chain,
						storage.Batch{Start: start, End: end}, nil, nil, categories, start, mode == "ongoing")
				}
				if err != nil || len(shells) != 1 {
					t.Fatalf("generation returned %d cards, err=%v; want one complete card", len(shells), err)
				}
				wantCalls := 1
				if correction {
					wantCalls = 2
				}
				if len(provider.calls) != wantCalls {
					t.Fatalf("calls=%d, want %d", len(provider.calls), wantCalls)
				}
				for _, request := range provider.calls {
					if request.MaxOutputTokens != 8192 || request.Output == nil || !request.Output.Strict {
						t.Fatalf("request budget=%d, schema=%v; want bounded 8192 with strict output", request.MaxOutputTokens, request.Output)
					}
				}
			})
		}
	}
}
