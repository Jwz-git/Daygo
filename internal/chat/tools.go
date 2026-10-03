package chat

import "encoding/json"

// ToolSpec is one entry in the closed tool catalog (docs/05 §5.12). The write
// face is exactly the ten operations of §5.9.2 and the read face mirrors the
// §5.9.1 commands this build can serve; search and status are deferred with
// their CLI semantics. Arguments is a strict JSON Schema (additionalProperties
// false) the service validates before execution.
type ToolSpec struct {
	Name        string
	Description string
	Arguments   json.RawMessage
	Write       bool
}

// Tool names, kept as constants so the executor's switch and the tests agree
// with the catalog by construction.
const (
	ToolTimeline       = "timeline"
	ToolCard           = "card"
	ToolDaily          = "daily"
	ToolWeekly         = "weekly"
	ToolCategories     = "categories"
	ToolCategoryAdd    = "category_add"
	ToolCategoryUpdate = "category_update"
	ToolCategoryRemove = "category_remove"
	ToolCardUpdate     = "card_update"
	ToolCardDelete     = "card_delete"
	ToolGoalSet        = "goal_set"
	ToolPlan           = "plan"
	ToolPlanAdd        = "plan_add"
	ToolPlanUpdate     = "plan_update"
	ToolPlanComplete   = "plan_complete"
	ToolPlanDelete     = "plan_delete"
)

// dayPattern is the wire form every day/week argument must match; the
// executor re-validates semantically through timeutil (a syntactically valid
// but nonexistent date must still fail). The double backslashes are regex
// escapes inside a JSON string: the schema text must carry \\d for the
// pattern to reach the validator as \d.
const dayPattern = `^\\d{4}-\\d{2}-\\d{2}$`

// colorPattern is #RRGGBB.
const colorPattern = `^#[0-9A-Fa-f]{6}$`

// clockPattern is a 24-hour wall clock, "9:05" or "09:05". Hours 00–03 belong
// to the next calendar date of the logical day (timeutil.ResolveDayClock).
const clockPattern = `^([01]?\\d|2[0-3]):[0-5]\\d$`

// toolCatalog is the closed set, fixed order. It is a value, not a registry:
// adding a tool is a deliberate act that must touch this list, the executor,
// the system prompt, and docs/05 §5.12 together.
var toolCatalog = []ToolSpec{
	{
		Name: ToolTimeline,
		Description: "Query the timeline of one logical day (4 AM boundary): cards, categories, " +
			"total minutes, and failed groups. day is yyyy-MM-dd.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{"day":{"type":"string","pattern":"` + dayPattern + `"}},
			"required":["day"],
			"additionalProperties":false
		}`),
	},
	{
		Name:        ToolCard,
		Description: "Query one timeline card by id (title, summary, category, duration, apps, and distraction records).",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{"cardId":{"type":"integer","minimum":1}},
			"required":["cardId"],
			"additionalProperties":false
		}`),
	},
	{
		Name:        ToolDaily,
		Description: "Query the journal and goals of one logical day (including focus/distraction categories). day is yyyy-MM-dd.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{"day":{"type":"string","pattern":"` + dayPattern + `"}},
			"required":["day"],
			"additionalProperties":false
		}`),
	},
	{
		Name:        ToolWeekly,
		Description: "Query one week's durations and category shares. weekStart is that week's Monday as yyyy-MM-dd.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{"weekStart":{"type":"string","pattern":"` + dayPattern + `"}},
			"required":["weekStart"],
			"additionalProperties":false
		}`),
	},
	{
		Name: ToolPlan,
		Description: "Query the plan of one logical day: each time block's id, start / end (HH:mm), title, notes, " +
			"category, status (planned / done / skipped), and the recorded card minutes in its category plus " +
			"distraction minutes inside it so far. day is yyyy-MM-dd.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{"day":{"type":"string","pattern":"` + dayPattern + `"}},
			"required":["day"],
			"additionalProperties":false
		}`),
	},
	{
		Name:        ToolCategories,
		Description: "List all categories (id, name, color, details, sort order, built-in / idle flags).",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{},
			"additionalProperties":false
		}`),
	},
	{
		Name: ToolCategoryAdd,
		Description: "Add a category. name is required and must not duplicate an existing category; " +
			"colorHex defaults to \"#8E8E93\"; without sortOrder the category is appended at the end. " +
			"Write operation, gated by the sandbox.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{
				"name":{"type":"string","minLength":1,"maxLength":64},
				"colorHex":{"type":"string","pattern":"` + colorPattern + `"},
				"details":{"type":"string","maxLength":2000},
				"sortOrder":{"type":"integer","minimum":0,"maximum":999},
				"isIdle":{"type":"boolean"}
			},
			"required":["name"],
			"additionalProperties":false
		}`),
		Write: true,
	},
	{
		Name: ToolCategoryUpdate,
		Description: "Update a category. categoryId is required (get it from the categories tool); " +
			"at least one field to change must be provided; built-in categories (System / Idle) cannot " +
			"be modified. Write operation, gated by the sandbox.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{
				"categoryId":{"type":"string","minLength":1},
				"name":{"type":"string","minLength":1,"maxLength":64},
				"colorHex":{"type":"string","pattern":"` + colorPattern + `"},
				"details":{"type":"string","maxLength":2000},
				"sortOrder":{"type":"integer","minimum":0,"maximum":999},
				"isIdle":{"type":"boolean"}
			},
			"required":["categoryId"],
			"additionalProperties":false
		}`),
		Write: true,
	},
	{
		Name:        ToolCategoryRemove,
		Description: "Delete a non-built-in category. Cards are not deleted. Write operation, gated by the sandbox.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{"categoryId":{"type":"string","minLength":1}},
			"required":["categoryId"],
			"additionalProperties":false
		}`),
		Write: true,
	},
	{
		Name: ToolCardUpdate,
		Description: "Update a card's category (by category name, which must already exist and must " +
			"not be a built-in category — System and Idle are assigned by the pipeline) or title; " +
			"cardId is required, and at least one of category and title must be provided. " +
			"Write operation, gated by the sandbox.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{
				"cardId":{"type":"integer","minimum":1},
				"category":{"type":"string","minLength":1,"maxLength":64},
				"title":{"type":"string","minLength":1,"maxLength":200}
			},
			"required":["cardId"],
			"additionalProperties":false
		}`),
		Write: true,
	},
	{
		Name:        ToolCardDelete,
		Description: "Soft-delete a card (it disappears from the timeline and can be restored by reprocessing). Write operation, gated by the sandbox.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{"cardId":{"type":"integer","minimum":1}},
			"required":["cardId"],
			"additionalProperties":false
		}`),
		Write: true,
	},
	{
		Name: ToolGoalSet,
		Description: "Set the goals of one logical day: focus minutes, distraction limit, skip flag, " +
			"focus / distraction categories (by category id). Write operation, gated by the sandbox.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{
				"day":{"type":"string","pattern":"` + dayPattern + `"},
				"focusTargetMinutes":{"type":"integer","minimum":0},
				"distractionLimitMinutes":{"type":"integer","minimum":0},
				"isSkipped":{"type":"boolean"},
				"focusCategoryIds":{"type":"array","items":{"type":"string","minLength":1},"maxItems":32,"uniqueItems":true},
				"distractionCategoryIds":{"type":"array","items":{"type":"string","minLength":1},"maxItems":32,"uniqueItems":true}
			},
			"required":["day"],
			"additionalProperties":false
		}`),
		Write: true,
	},
	{
		Name: ToolPlanAdd,
		Description: "Add a plan block to one logical day. start / end are 24-hour HH:mm; times before 04:00 " +
			"belong to the next calendar date of that day, and end \"04:00\" means the end of the day. " +
			"title is required; notes are free text; categoryId is optional (from the categories tool); " +
			"remind (default true) sends a system notification when the block starts. Write operation, gated by the sandbox.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{
				"day":{"type":"string","pattern":"` + dayPattern + `"},
				"start":{"type":"string","pattern":"` + clockPattern + `"},
				"end":{"type":"string","pattern":"` + clockPattern + `"},
				"title":{"type":"string","minLength":1,"maxLength":200},
				"notes":{"type":"string","maxLength":4000},
				"categoryId":{"type":"string","maxLength":64},
				"remind":{"type":"boolean"}
			},
			"required":["day","start","end","title"],
			"additionalProperties":false
		}`),
		Write: true,
	},
	{
		Name: ToolPlanUpdate,
		Description: "Edit a plan block by blockId (from the plan tool): any of day, start, end, title, notes, " +
			"categoryId (empty string clears it), remind. At least one field to change is required. " +
			"Write operation, gated by the sandbox.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{
				"blockId":{"type":"integer","minimum":1},
				"day":{"type":"string","pattern":"` + dayPattern + `"},
				"start":{"type":"string","pattern":"` + clockPattern + `"},
				"end":{"type":"string","pattern":"` + clockPattern + `"},
				"title":{"type":"string","minLength":1,"maxLength":200},
				"notes":{"type":"string","maxLength":4000},
				"categoryId":{"type":"string","maxLength":64},
				"remind":{"type":"boolean"}
			},
			"required":["blockId"],
			"additionalProperties":false
		}`),
		Write: true,
	},
	{
		Name: ToolPlanComplete,
		Description: "Mark a plan block done (default), skipped, or back to planned. Use it when a task is finished. " +
			"Write operation, gated by the sandbox.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{
				"blockId":{"type":"integer","minimum":1},
				"status":{"type":"string","enum":["done","skipped","planned"]}
			},
			"required":["blockId"],
			"additionalProperties":false
		}`),
		Write: true,
	},
	{
		Name:        ToolPlanDelete,
		Description: "Delete a plan block. Write operation, gated by the sandbox.",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{"blockId":{"type":"integer","minimum":1}},
			"required":["blockId"],
			"additionalProperties":false
		}`),
		Write: true,
	},
}

// toolByName looks up one catalog entry.
func toolByName(name string) (ToolSpec, bool) {
	for _, spec := range toolCatalog {
		if spec.Name == name {
			return spec, true
		}
	}
	return ToolSpec{}, false
}

// isWriteTool reports whether a name belongs to the write face.
func isWriteTool(name string) bool {
	spec, ok := toolByName(name)
	return ok && spec.Write
}
