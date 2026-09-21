package app

// Timeline DTOs mirror docs/05 §5.5.2 (TimelineDayDTO, TimelineCardDTO, and
// their children). Field names and json tags match the hand-written frontend
// subset in frontend/src/api/dto.ts so the two sides can be swapped for the
// generated models without a wire change.

type TimelineDayDTO struct {
	Day              string               `json:"day"`
	DayStartTs       int64                `json:"dayStartTs"`
	DayEndTs         int64                `json:"dayEndTs"`
	Cards            []TimelineCardDTO    `json:"cards"`
	Categories       []CategoryDTO        `json:"categories"`
	TrackedMinutes   float64              `json:"trackedMinutes"`
	IdleMinutes      float64              `json:"idleMinutes"`
	Failures         []TimelineFailureDTO `json:"failures"`
	ProcessingRanges []RangeDTO           `json:"processingRanges"`
	GeneratedAtTs    int64                `json:"generatedAtTs"`
}

// TimelineCardDTO maps one timeline_cards row. VideoSummaryUrl and
// OtherVideoSummaryUrls stay null/empty in this slice: the asset handler
// belongs to the media slice, and returning a fabricated URL would create a
// dead link the UI would render as a playable video.
type TimelineCardDTO struct {
	ID                    int64              `json:"id"`
	BatchID               *int64             `json:"batchId"`
	Day                   string             `json:"day"`
	Start                 string             `json:"start"`
	End                   string             `json:"end"`
	StartTs               int64              `json:"startTs"`
	EndTs                 int64              `json:"endTs"`
	Category              string             `json:"category"`
	Subcategory           string             `json:"subcategory"`
	Title                 string             `json:"title"`
	Summary               string             `json:"summary"`
	DetailedSummary       string             `json:"detailedSummary"`
	VideoSummaryURL       *string            `json:"videoSummaryUrl"`
	OtherVideoSummaryURLs []string           `json:"otherVideoSummaryUrls"`
	AppSites              *AppSitesDTO       `json:"appSites"`
	Distractions          []DistractionDTO   `json:"distractions"`
	ActivityPoints        []ActivityPointDTO `json:"activityPoints"`
	IsIdle                bool               `json:"isIdle"`
	DurationMinutes       float64            `json:"durationMinutes"`
}

type AppSitesDTO struct {
	Primary   *string `json:"primary"`
	Secondary *string `json:"secondary"`
}

type DistractionDTO struct {
	ID              string  `json:"id"`
	StartTime       string  `json:"startTime"`
	EndTime         string  `json:"endTime"`
	Title           string  `json:"title"`
	Summary         string  `json:"summary"`
	VideoSummaryURL *string `json:"videoSummaryUrl"`
}

// ActivityPointDTO is one concrete time point inside a card (Dayflow model:
// one card per window, observations carried as per-point descriptions).
type ActivityPointDTO struct {
	Time        string `json:"time"`
	Description string `json:"description"`
}

type TimelineFailureDTO struct {
	BatchIDs  []int64 `json:"batchIds"`
	StartTs   int64   `json:"startTs"`
	EndTs     int64   `json:"endTs"`
	Kind      string  `json:"kind"`
	Message   string  `json:"message"`
	Retryable bool    `json:"retryable"`
}

// RangeDTO is one window on the day track, with the batches that own it. A card
// belongs to the window its batch covers, which is not the same as overlapping
// it: an ongoing rewrite extends a batch's span back over the card it continues,
// so a card the rerun will replace can sit entirely before the window.
type RangeDTO struct {
	StartTs  int64   `json:"startTs"`
	EndTs    int64   `json:"endTs"`
	BatchIDs []int64 `json:"batchIds"`
}

type CategoryDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ColorHex    string `json:"colorHex"`
	Details     string `json:"details"`
	SortOrder   int    `json:"sortOrder"`
	IsSystem    bool   `json:"isSystem"`
	IsIdle      bool   `json:"isIdle"`
	CreatedAtTs int64  `json:"createdAtTs"`
	UpdatedAtTs int64  `json:"updatedAtTs"`
}
