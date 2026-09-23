package mcp

import (
	"encoding/json"

	"github.com/Jwz-git/Daygo/internal/agentbridge"
)

// Read tool names. They mirror the CLI read commands (docs/05 §5.9.1) minus
// status (CLI-only) and search (deferred): the MCP read face of §5.9.3.
const (
	toolTimeline   = "timeline"
	toolCard       = "card"
	toolDaily      = "daily"
	toolWeekly     = "weekly"
	toolCategories = "categories"
)

// dayPattern / colorPattern are the wire forms; the server-side paths
// (agentread for reads, the bridge Handler for writes) re-validate
// semantically. Double backslashes survive the Go string into the JSON schema.
const (
	dayPattern   = `^\\d{4}-\\d{2}-\\d{2}$`
	colorPattern = `^#[0-9A-Fa-f]{6}$`
)

type toolSpec struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	Write       bool
}

// catalog is the closed tool set in fixed order: five reads then the six write
// operations of §5.9.2. The write names are sourced from agentbridge.Operations
// so this face cannot drift from the bridge's accepted set. Argument schemas
// mirror docs/05 §5.9.2 / §5.12 semantics; the authoritative validation is
// server-side, so these are the client-facing hint.
var catalog = buildCatalog()

func buildCatalog() []toolSpec {
	reads := []toolSpec{
		{toolTimeline, "Query one logical day's timeline (4 AM boundary): cards, totals, idle minutes. day is yyyy-MM-dd, today, or yesterday.", schemaOptionalDay("day"), false},
		{toolCard, "Query one timeline card by id.", schemaCardID(), false},
		{toolDaily, "Query one logical day's journal and goal. day is yyyy-MM-dd, today, or yesterday.", schemaOptionalDay("day"), false},
		{toolWeekly, "Query one week's durations and category shares. weekStart is that week's Monday as yyyy-MM-dd.", schemaOptionalDay("weekStart"), false},
		{toolCategories, "List all categories including built-ins.", schemaEmpty(), false},
	}
	writes := writeToolSpecs()
	return append(reads, writes...)
}

// writeToolSpecs builds the six write specs by iterating
// agentbridge.Operations, so the names and order are the bridge's, not a
// second hand-kept list.
func writeToolSpecs() []toolSpec {
	meta := map[string]struct {
		desc   string
		schema json.RawMessage
	}{
		"category_add": {"Add a category. name is required; colorHex defaults to #8E8E93. Gated by agentEditsEnabled.",
			schema(`{"name":{"type":"string","minLength":1,"maxLength":64},"colorHex":{"type":"string","pattern":"`+colorPattern+`"},"details":{"type":"string","maxLength":2000},"sortOrder":{"type":"integer","minimum":0,"maximum":999},"isIdle":{"type":"boolean"}}`, `["name"]`)},
		"category_update": {"Update a category by categoryId; at least one field to change is required; built-ins cannot be modified.",
			schema(`{"categoryId":{"type":"string","minLength":1},"name":{"type":"string","minLength":1,"maxLength":64},"colorHex":{"type":"string","pattern":"`+colorPattern+`"},"details":{"type":"string","maxLength":2000},"sortOrder":{"type":"integer","minimum":0,"maximum":999},"isIdle":{"type":"boolean"}}`, `["categoryId"]`)},
		"category_remove": {"Delete a non-built-in category. Cards are kept.",
			schema(`{"categoryId":{"type":"string","minLength":1}}`, `["categoryId"]`)},
		"card_update": {"Update a card's category (by existing, non-built-in name) or title; cardId required, at least one of category/title.",
			schema(`{"cardId":{"type":"integer","minimum":1},"category":{"type":"string","minLength":1,"maxLength":64},"title":{"type":"string","minLength":1,"maxLength":200}}`, `["cardId"]`)},
		"card_delete": {"Soft-delete a card; it can be restored by reprocessing.",
			schema(`{"cardId":{"type":"integer","minimum":1}}`, `["cardId"]`)},
		"goal_set": {"Set one logical day's goal: focus minutes, distraction limit, skip flag, focus/distraction category ids.",
			schema(`{"day":{"type":"string","pattern":"`+dayPattern+`"},"focusTargetMinutes":{"type":"integer","minimum":0},"distractionLimitMinutes":{"type":"integer","minimum":0},"isSkipped":{"type":"boolean"},"focusCategoryIds":{"type":"array","items":{"type":"string","minLength":1},"maxItems":32,"uniqueItems":true},"distractionCategoryIds":{"type":"array","items":{"type":"string","minLength":1},"maxItems":32,"uniqueItems":true}}`, `["day"]`)},
	}
	specs := make([]toolSpec, 0, len(agentbridge.Operations))
	for _, op := range agentbridge.Operations {
		m := meta[op]
		specs = append(specs, toolSpec{Name: op, Description: m.desc, InputSchema: m.schema, Write: true})
	}
	return specs
}

// schema assembles an object schema with the given property block and required
// list, additionalProperties:false throughout.
func schema(properties, required string) json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":` + properties + `,"required":` + required + `,"additionalProperties":false}`)
}

func schemaOptionalDay(field string) json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"` + field + `":{"type":"string"}},"additionalProperties":false}`)
}

func schemaCardID() json.RawMessage {
	return schema(`{"cardId":{"type":"integer","minimum":1}}`, `["cardId"]`)
}

func schemaEmpty() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
}

func toolByName(name string) (toolSpec, bool) {
	for _, s := range catalog {
		if s.Name == name {
			return s, true
		}
	}
	return toolSpec{}, false
}
