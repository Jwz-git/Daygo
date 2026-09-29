package agentread

// The snake_case result structs marshaled as the external JSON (docs/05
// §5.9.1). Field order is declaration order — json marshals structs in order,
// so the diff gate stays stable — and every top-level result leads with
// schema_version. Struct (never map) output is what keeps key order fixed.
// omitempty encodes the null-omission rules: optional text and timestamps drop
// out when empty; slices are always initialized so they render as [] not null.

type StatusResult struct {
	SchemaVersion int    `json:"schema_version"`
	DatabasePath  string `json:"database_path"`
	DBUserVersion int    `json:"db_user_version"`
	GeneratedAt   string `json:"generated_at"`
}

type TimelineCard struct {
	ID              int64   `json:"id"`
	Start           string  `json:"start"`
	End             string  `json:"end"`
	StartTs         int64   `json:"start_ts"`
	EndTs           int64   `json:"end_ts"`
	Category        string  `json:"category"`
	Subcategory     string  `json:"subcategory,omitempty"`
	Title           string  `json:"title"`
	IsIdle          bool    `json:"is_idle"`
	DurationMinutes float64 `json:"duration_minutes"`
}

type TimelineResult struct {
	SchemaVersion  int            `json:"schema_version"`
	Day            string         `json:"day"`
	DayStartTs     int64          `json:"day_start_ts"`
	DayEndTs       int64          `json:"day_end_ts"`
	TrackedMinutes float64        `json:"tracked_minutes"`
	IdleMinutes    float64        `json:"idle_minutes"`
	Cards          []TimelineCard `json:"cards"`
}

type CardResult struct {
	SchemaVersion   int     `json:"schema_version"`
	ID              int64   `json:"id"`
	Day             string  `json:"day"`
	Start           string  `json:"start"`
	End             string  `json:"end"`
	StartTs         int64   `json:"start_ts"`
	EndTs           int64   `json:"end_ts"`
	Category        string  `json:"category"`
	Subcategory     string  `json:"subcategory,omitempty"`
	Title           string  `json:"title"`
	Summary         string  `json:"summary,omitempty"`
	DetailedSummary string  `json:"detailed_summary,omitempty"`
	IsIdle          bool    `json:"is_idle"`
	DurationMinutes float64 `json:"duration_minutes"`
}

type DailyResult struct {
	SchemaVersion int          `json:"schema_version"`
	Day           string       `json:"day"`
	Journal       DailyJournal `json:"journal"`
	Goal          DailyGoal    `json:"goal"`
}

type DailyJournal struct {
	Intentions  *string `json:"intentions,omitempty"`
	Notes       *string `json:"notes,omitempty"`
	Goals       *string `json:"goals,omitempty"`
	Reflections *string `json:"reflections,omitempty"`
	Status      string  `json:"status"`
	UpdatedAt   *string `json:"updated_at,omitempty"`
}

type DailyGoal struct {
	Exists                  bool              `json:"exists"`
	FocusTargetMinutes      int               `json:"focus_target_minutes"`
	DistractionLimitMinutes int               `json:"distraction_limit_minutes"`
	IsSkipped               bool              `json:"is_skipped"`
	FocusCategories         []GoalCategoryRef `json:"focus_categories"`
	DistractionCategories   []GoalCategoryRef `json:"distraction_categories"`
}

type GoalCategoryRef struct {
	CategoryID string `json:"category_id"`
	Name       string `json:"name"`
	ColorHex   string `json:"color_hex"`
	SortOrder  int    `json:"sort_order"`
}

type WeeklyResult struct {
	SchemaVersion  int              `json:"schema_version"`
	WeekStart      string           `json:"week_start"`
	WeekStartTs    int64            `json:"week_start_ts"`
	WeekEndTs      int64            `json:"week_end_ts"`
	TrackedMinutes float64          `json:"tracked_minutes"`
	FocusMinutes   float64          `json:"focus_minutes"`
	Categories     []WeeklyCategory `json:"categories"`
	Days           []WeeklyDay      `json:"days"`
	Insights       WeeklyInsights   `json:"insights"`
}

type WeeklyCategory struct {
	Name     string  `json:"name"`
	Minutes  float64 `json:"minutes"`
	Share    float64 `json:"share"`
	ColorHex string  `json:"color_hex"`
}

type WeeklyDay struct {
	Day            string           `json:"day"`
	TrackedMinutes float64          `json:"tracked_minutes"`
	FocusMinutes   float64          `json:"focus_minutes"`
	Categories     []WeeklyCategory `json:"categories"`
}

type WeeklyInsights struct {
	LongestFocusMinutes  float64 `json:"longest_focus_minutes"`
	LongestFocusDay      string  `json:"longest_focus_day"`
	PeakHour             int     `json:"peak_hour"`
	PeakHourMinutes      float64 `json:"peak_hour_minutes"`
	MostActiveDay        string  `json:"most_active_day"`
	MostActiveDayMinutes float64 `json:"most_active_day_minutes"`
	ActiveDays           int     `json:"active_days"`
	AvgDailyFocusMinutes float64 `json:"avg_daily_focus_minutes"`
}

type CategoriesResult struct {
	SchemaVersion int        `json:"schema_version"`
	Categories    []Category `json:"categories"`
}

type Category struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ColorHex  string `json:"color_hex"`
	Details   string `json:"details,omitempty"`
	SortOrder int    `json:"sort_order"`
	IsSystem  bool   `json:"is_system"`
	IsIdle    bool   `json:"is_idle"`
}
