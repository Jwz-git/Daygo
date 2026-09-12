package chat

import "encoding/json"

// ToolSpec is one entry in the closed tool catalog (docs/05 §5.12). The write
// face is exactly the six operations of §5.9.2 and the read face mirrors the
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
)

// dayPattern is the wire form every day/week argument must match; the
// executor re-validates semantically through timeutil (a syntactically valid
// but nonexistent date must still fail). The double backslashes are regex
// escapes inside a JSON string: the schema text must carry \\d for the
// pattern to reach the validator as \d.
const dayPattern = `^\\d{4}-\\d{2}-\\d{2}$`

// colorPattern is #RRGGBB.
const colorPattern = `^#[0-9A-Fa-f]{6}$`

// toolCatalog is the closed set, fixed order. It is a value, not a registry:
// adding a tool is a deliberate act that must touch this list, the executor,
// the system prompt, and docs/05 §5.12 together.
var toolCatalog = []ToolSpec{
	{
		Name: ToolTimeline,
		Description: "查询某个逻辑日（凌晨 4 点边界）的时间线：卡片、分类、合计分钟数与失败分组。" +
			"day 为 yyyy-MM-dd。",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{"day":{"type":"string","pattern":"` + dayPattern + `"}},
			"required":["day"],
			"additionalProperties":false
		}`),
	},
	{
		Name:        ToolCard,
		Description: "按卡片 ID 查询单张时间线卡片的详情（标题、摘要、分类、时长、应用与分心记录）。",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{"cardId":{"type":"integer","minimum":1}},
			"required":["cardId"],
			"additionalProperties":false
		}`),
	},
	{
		Name:        ToolDaily,
		Description: "查询某个逻辑日的日记与当日目标（含专注 / 分心分类）。day 为 yyyy-MM-dd。",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{"day":{"type":"string","pattern":"` + dayPattern + `"}},
			"required":["day"],
			"additionalProperties":false
		}`),
	},
	{
		Name:        ToolWeekly,
		Description: "查询某一周的时长与分类占比。weekStart 为该周周一的 yyyy-MM-dd。",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{"weekStart":{"type":"string","pattern":"` + dayPattern + `"}},
			"required":["weekStart"],
			"additionalProperties":false
		}`),
	},
	{
		Name:        ToolCategories,
		Description: "列出全部分类（id、名称、颜色、说明、排序、是否内置 / 空闲分类）。",
		Arguments: json.RawMessage(`{
			"type":"object",
			"properties":{},
			"additionalProperties":false
		}`),
	},
	{
		Name: ToolCategoryAdd,
		Description: "新增一个分类。name 必填且不得与现有分类重名；colorHex 缺省 \"#8E8E93\"；" +
			"sortOrder 缺省时追加到末尾。写操作，受沙箱门禁约束。",
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
		Description: "修改一个分类。categoryId 必填（从 categories 工具获取）；至少提供一项要修改的字段；" +
			"内置分类（System / Idle）不可修改。写操作，受沙箱门禁约束。",
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
		Description: "删除一个非内置分类。卡片不会被删除。写操作，受沙箱门禁约束。",
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
		Description: "修改一张卡片的分类（按分类名，须为现有分类）或标题；cardId 必填，category 与 " +
			"title 至少提供一项。写操作，受沙箱门禁约束。",
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
		Description: "软删除一张卡片（时间线不再显示，可经重处理恢复）。写操作，受沙箱门禁约束。",
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
		Description: "设置某个逻辑日的目标：专注分钟数、分心上限、是否跳过、专注 / 分心分类（按分类 id）。" +
			"写操作，受沙箱门禁约束。",
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
