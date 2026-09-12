package chat

import (
	"encoding/json"
	"strings"
	"testing"
)

// The catalog is the closed set from docs/05 §5.12: five readable commands
// (search and status deferred) plus exactly the six §5.9.2 write operations,
// no more, no less.
func TestToolCatalogIsTheClosedSet(t *testing.T) {
	want := []struct {
		name  string
		write bool
	}{
		{ToolTimeline, false},
		{ToolCard, false},
		{ToolDaily, false},
		{ToolWeekly, false},
		{ToolCategories, false},
		{ToolCategoryAdd, true},
		{ToolCategoryUpdate, true},
		{ToolCategoryRemove, true},
		{ToolCardUpdate, true},
		{ToolCardDelete, true},
		{ToolGoalSet, true},
	}
	if len(toolCatalog) != len(want) {
		t.Fatalf("catalog has %d tools, want %d", len(toolCatalog), len(want))
	}
	for i, expected := range want {
		if toolCatalog[i].Name != expected.name {
			t.Fatalf("catalog[%d] = %s, want %s", i, toolCatalog[i].Name, expected.name)
		}
		if toolCatalog[i].Write != expected.write {
			t.Fatalf("catalog[%d] (%s) write = %v, want %v", i, toolCatalog[i].Name, toolCatalog[i].Write, expected.write)
		}
		if toolCatalog[i].Description == "" {
			t.Fatalf("catalog[%d] (%s) has no description", i, toolCatalog[i].Name)
		}
	}

	if isWriteTool(ToolTimeline) {
		t.Fatal("timeline must not be a write tool")
	}
	if !isWriteTool(ToolCardDelete) {
		t.Fatal("card_delete must be a write tool")
	}
	if isWriteTool("nonexistent") {
		t.Fatal("unknown tool must not report as write")
	}
}

func TestValidateToolArguments(t *testing.T) {
	valid := []struct {
		tool  string
		args  string
		label string
	}{
		{ToolTimeline, `{"day":"2026-09-12"}`, "timeline day"},
		{ToolCard, `{"cardId":42}`, "card id"},
		{ToolCategories, `{}`, "categories empty"},
		{ToolCategories, ``, "categories omitted args"},
		{ToolCategoryAdd, `{"name":"Deep Work","colorHex":"#FF0000","sortOrder":3,"isIdle":false}`, "category add full"},
		{ToolCategoryAdd, `{"name":"Deep Work"}`, "category add minimal"},
		{ToolCardUpdate, `{"cardId":7,"category":"Coding"}`, "card update category"},
		{ToolGoalSet, `{"day":"2026-09-12","focusTargetMinutes":120,"focusCategoryIds":["a","b"]}`, "goal set"},
	}
	for _, tc := range valid {
		var raw json.RawMessage
		if tc.args != "" {
			raw = json.RawMessage(tc.args)
		}
		if err := validateToolArguments(tc.tool, raw); err != nil {
			t.Errorf("%s: %v", tc.label, err)
		}
	}

	invalid := []struct {
		tool  string
		args  string
		label string
	}{
		{ToolTimeline, `{"day":"today"}`, "day pattern"},
		{ToolTimeline, `{}`, "missing required day"},
		{ToolTimeline, `{"day":"2026-09-12","limit":5}`, "unknown field"},
		{ToolCard, `{"cardId":0}`, "cardId below minimum"},
		{ToolCard, `{"cardId":3.5}`, "cardId not an integer"},
		{ToolCategoryAdd, `{"name":""}`, "empty name"},
		{ToolCategoryAdd, `{"name":"X","colorHex":"red"}`, "bad colorHex"},
		{ToolGoalSet, `{"focusTargetMinutes":120}`, "missing day"},
		{ToolGoalSet, `{"day":"2026-09-12","focusTargetMinutes":-1}`, "negative minutes"},
		{ToolGoalSet, `{"day":"2026-09-12","focusCategoryIds":["a","a"]}`, "duplicate ids"},
	}
	for _, tc := range invalid {
		if err := validateToolArguments(tc.tool, json.RawMessage(tc.args)); err == nil {
			t.Errorf("%s: expected rejection, got nil", tc.label)
		}
	}

	if err := validateToolArguments("no_such_tool", json.RawMessage(`{}`)); err == nil {
		t.Error("unknown tool must be rejected")
	}
}

func TestDecodeEnvelope(t *testing.T) {
	reply, err := decodeEnvelope(`{"kind":"answer","answer":"今天你专注了 3 小时。"}`)
	if err != nil {
		t.Fatalf("answer envelope: %v", err)
	}
	if reply.Kind != envelopeKindAnswer || reply.Answer != "今天你专注了 3 小时。" {
		t.Fatalf("decoded = %+v", reply)
	}

	reply, err = decodeEnvelope(`{"kind":"tool","tool":"timeline","arguments":{"day":"2026-09-12"}}`)
	if err != nil {
		t.Fatalf("tool envelope: %v", err)
	}
	if reply.Kind != envelopeKindTool || reply.Tool != "timeline" ||
		string(reply.Arguments) != `{"day":"2026-09-12"}` {
		t.Fatalf("decoded = %+v", reply)
	}

	// Omitted arguments default to {} for parameterless tools.
	reply, err = decodeEnvelope(`{"kind":"tool","tool":"categories"}`)
	if err != nil {
		t.Fatalf("tool envelope without arguments: %v", err)
	}
	if string(reply.Arguments) != `{}` {
		t.Fatalf("arguments = %s, want {}", reply.Arguments)
	}

	// Fenced output is recovered by the shared extractor.
	if _, err := decodeEnvelope("```json\n{\"kind\":\"answer\",\"answer\":\"ok\"}\n```"); err != nil {
		t.Fatalf("fenced envelope: %v", err)
	}

	failures := []string{
		`{"kind":"answer"}`,                        // empty answer
		`{"kind":"tool","arguments":{}}`,           // missing tool name
		`{"kind":"other","answer":"x"}`,            // bad kind (schema level)
		`{"tool":"timeline"}`,                      // missing kind
		`{"kind":"answer","answer":"x","extra":1}`, // unknown field (schema level)
		`no json at all`,                           // no JSON (extractor level)
	}
	for _, text := range failures {
		if _, err := decodeEnvelope(text); err == nil {
			t.Errorf("envelope %q must fail", text)
		}
	}
}

func TestAgentSystemPrompt(t *testing.T) {
	readonly := agentSystemPrompt("readonly", "2026-09-12", "2026-09-07")
	for _, want := range []string{
		"2026-09-12", "2026-09-07",
		ToolTimeline, ToolCard, ToolDaily, ToolWeekly, ToolCategories,
		ToolCategoryAdd, ToolCategoryUpdate, ToolCategoryRemove,
		ToolCardUpdate, ToolCardDelete, ToolGoalSet,
		"readonly",
	} {
		if !strings.Contains(readonly, want) {
			t.Errorf("readonly prompt missing %q", want)
		}
	}
	if !strings.Contains(readonly, "不要重试写操作") {
		t.Error("readonly prompt must instruct the model not to retry writes")
	}

	edits := agentSystemPrompt("edits", "2026-09-12", "2026-09-07")
	if !strings.Contains(edits, "已启用") {
		t.Error("edits prompt must announce enabled writes")
	}
	if strings.Contains(edits, "不要重试写操作") {
		t.Error("edits prompt must not carry the readonly instruction")
	}

	// Any non-edits value renders the closed gate.
	if agentSystemPrompt("garbage", "2026-09-12", "2026-09-07") == edits {
		t.Error("garbage editMode must not render as edits")
	}
}

func TestNormalizeEditMode(t *testing.T) {
	for _, in := range []string{"", "readonly", "garbage", "EDITS", "edits "} {
		if got := normalizeEditMode(in); got != "readonly" {
			t.Errorf("normalizeEditMode(%q) = %q, want readonly", in, got)
		}
	}
	if got := normalizeEditMode("edits"); got != "edits" {
		t.Errorf("normalizeEditMode(edits) = %q", got)
	}
}
