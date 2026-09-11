// Package domain holds the shared value types of Daygo's business logic.
//
// Types here are pure values: no database handles, no file paths that get
// opened, no platform capability. Storage returns them, services pass them
// around, and internal/app converts them into DTOs — domain types never bind
// to the frontend directly (docs/05 §4.1).
package domain

// TimelineCard is one activity card on the timeline, in the shape storage
// returns and services pass around.
//
// Start and End are the LLM's localized clock strings; StartTs and EndTs are
// derived from them. Both representations are kept deliberately (docs/03
// §3.3.2) and neither side may be recomputed by the frontend.
type TimelineCard struct {
	ID               int64
	BatchID          *int64
	Day              string // logical day yyyy-MM-dd, 4 AM boundary
	Start            string // clock string, LLM output verbatim
	End              string
	StartTs          int64 // derived from Start (docs/03 §3.5)
	EndTs            int64
	Category         string
	Subcategory      string
	Title            string
	Summary          string
	DetailedSummary  string
	VideoSummaryPath string // relative to the app support dir; empty when absent
	Metadata         string // JSON, opaque at this layer
	Deleted          bool
	CreatedAtUnix    int64
	UpdatedAtUnix    int64
}

// CardShell is what the analysis pipeline submits to ReplaceCardsInRange: the
// parsed LLM output before any timestamp derivation. IDs and derived
// timestamps do not exist yet; ReplaceCardsInRange derives them inside its
// transaction, and shells whose clock strings cannot be resolved come back in
// ReplaceResult.SkippedCards instead of being silently dropped.
type CardShell struct {
	Start            string // clock string, e.g. "10:21 AM"
	End              string
	Category         string
	Subcategory      string
	Title            string
	Summary          string
	DetailedSummary  string
	VideoSummaryPath string
	Metadata         string
}

// Category is a first-class timeline category (docs/03 §3.3.3), not a member
// of a settings array.
type Category struct {
	ID            string // UUID
	Name          string // unique; timeline_cards.category stores this string
	ColorHex      string
	Details       string
	SortOrder     int
	IsSystem      bool // built-in, cannot be deleted
	IsIdle        bool // counts toward idle rather than tracked time
	CreatedAtUnix int64
	UpdatedAtUnix int64
}
