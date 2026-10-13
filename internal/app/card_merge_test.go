package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/agentbridge"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/domain"
)

func seedMergeCards(t *testing.T, b *Backend) {
	t.Helper()
	seedTimelineDay(t, b, []domain.CardShell{
		{Start: "10:00 AM", End: "10:20 AM", Category: "Focus Work", Title: "one", Summary: "s"},
		{Start: "10:20 AM", End: "10:40 AM", Category: "Focus Work", Title: "two", Summary: "s"},
	})
}

func TestCardMergeDTOGolden(t *testing.T) {
	input := MergeCardsInputDTO{CardIDs: []int64{1, 2}, Title: "Merged", Summary: "s", Category: "Coding"}
	result := MergeCardsResultDTO{CardID: 3, MergedCardIDs: []int64{1, 2}, Day: "2026-09-12", StartTs: 1, EndTs: 2}
	for _, fixture := range []struct {
		value any
		want  string
	}{
		{input, `{"cardIds":[1,2],"title":"Merged","summary":"s","detailedSummary":"","category":"Coding"}`},
		{result, `{"cardId":3,"mergedCardIds":[1,2],"day":"2026-09-12","startTs":1,"endTs":2}`},
	} {
		got, err := json.Marshal(fixture.value)
		if err != nil || string(got) != fixture.want {
			t.Fatalf("got=%s want=%s err=%v", got, fixture.want, err)
		}
	}
}

func TestCardMergeToolMatchesBinding(t *testing.T) {
	a, ae := writerBackendWithStore(t, t.TempDir())
	b, be := writerBackendWithStore(t, t.TempDir())
	seedMergeCards(t, a)
	seedMergeCards(t, b)
	args := `{"cardIds":[2,1],"title":"Merged","summary":"Combined","category":"Focus Work"}`
	tool := (chatToolExecutor{backend: a}).Execute(context.Background(), toolCall("card_merge", args))
	if tool.Err {
		t.Fatalf("tool=%s", tool.Data)
	}
	var input MergeCardsInputDTO
	if err := json.Unmarshal([]byte(args), &input); err != nil {
		t.Fatal(err)
	}
	result, err := b.MergeCards(input)
	if err != nil {
		t.Fatal(err)
	}
	var toolResult MergeCardsResultDTO
	if err := json.Unmarshal(tool.Data, &toolResult); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result, toolResult) {
		t.Fatalf("binding=%+v tool=%+v", result, toolResult)
	}
	for _, backend := range []*Backend{a, b} {
		cards, err := backend.store().Cards().CardsForDay(context.Background(), "2026-09-12")
		if err != nil || len(cards) != 1 || cards[0].Title != "Merged" || cards[0].DurationMinutes() != 40 {
			t.Fatalf("cards=%+v err=%v", cards, err)
		}
	}
	// Invalidation events are deliberately coalesced over 200ms.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && (ae.count(EventTimelineUpdated) == 0 || be.count(EventTimelineUpdated) == 0) {
		time.Sleep(10 * time.Millisecond)
	}
	if ae.count(EventTimelineUpdated) != 1 || be.count(EventTimelineUpdated) != 1 {
		t.Fatal("missing timeline invalidation")
	}
	for _, banned := range []string{"videoSummaryPath", "segment_path", "metadata", "recordings/"} {
		if strings.Contains(string(tool.Data), banned) {
			t.Fatalf("result leaks %s", banned)
		}
	}
}

func TestCardMergeBindingRefusesReadOnlyAndBadArguments(t *testing.T) {
	b, _ := writerBackendWithStore(t, t.TempDir())
	seedMergeCards(t, b)
	input := MergeCardsInputDTO{CardIDs: []int64{1, 2}, Title: "Merged", Summary: "s", Category: "Focus Work"}
	for _, change := range []func(*MergeCardsInputDTO){func(p *MergeCardsInputDTO) { p.CardIDs = []int64{1, 1} }, func(p *MergeCardsInputDTO) { p.Category = "System" }, func(p *MergeCardsInputDTO) { p.Title = " " }} {
		bad := input
		change(&bad)
		_, err := b.MergeCards(bad)
		var appErr *apperr.Error
		if !errors.As(err, &appErr) || appErr.Code != apperr.InvalidArgument {
			t.Fatalf("invalid merge=%v", err)
		}
	}
	ro, _ := backendWithStore(t)
	if _, err := ro.MergeCards(input); err == nil {
		t.Fatal("read-only merge succeeded")
	}
}

func TestAgentBridgeCardMergeGateAuditAndReplay(t *testing.T) {
	b, _ := writerBackendWithStore(t, t.TempDir())
	seedMergeCards(t, b)
	dir, err := os.MkdirTemp("/tmp", "daygo-merge-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	var audit bytes.Buffer
	server := agentbridge.NewServer(agentWriteHandler{backend: b}, &audit)
	if err := server.Start(context.Background(), filepath.Join(dir, "agent.sock")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	client := agentbridge.NewClient(filepath.Join(dir, "agent.sock"))
	args := map[string]any{"cardIds": []int64{1, 2}, "title": "Anonymous merged title", "summary": "Anonymous merged summary", "category": "Focus Work"}
	var bridgeErr *agentbridge.Error
	if _, err := client.Do(context.Background(), "card_merge", args, agentbridge.SourceMCP); !errors.As(err, &bridgeErr) || bridgeErr.Code != agentbridge.CodeEditsDisabled {
		t.Fatalf("default gate=%v", err)
	}
	enabled := true
	if _, err := b.UpdateSettings(SettingsPatchDTO{AgentEditsEnabled: &enabled}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(context.Background(), "card_merge", args, agentbridge.SourceMCP); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(context.Background(), "card_merge", args, agentbridge.SourceCLI); !errors.As(err, &bridgeErr) || bridgeErr.Code != agentbridge.CodeNotFound {
		t.Fatalf("replay=%v", err)
	}
	// The server writes audit after the response. Close waits for handlers.
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(audit.String(), `"operation":"card_merge"`) || !strings.Contains(audit.String(), `"source":"mcp"`) || strings.Contains(audit.String(), "Anonymous") || strings.Contains(audit.String(), "cardIds") {
		t.Fatalf("audit=%s", audit.String())
	}
}
