package agentread

import (
	"context"
	"database/sql"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/storage"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// readTimeout bounds each command's storage work.
const readTimeout = 10 * time.Second

// Reader is one read-only session against the business database. It owns the
// storage.Store it opened; callers must Close it.
type Reader struct {
	store *storage.Store
	loc   *time.Location
	now   func() time.Time
}

// Open resolves the database path (DAYGO_DB override or the support dir) and
// opens it read-only. loc is the host zone; nil means time.Local.
func Open(ctx context.Context, loc *time.Location) (*Reader, error) {
	path, err := DatabasePath()
	if err != nil {
		return nil, faultf(CodeInternal, "resolve database path failed")
	}
	return OpenAt(ctx, path, loc)
}

// OpenAt opens an explicit database file read-only. It is the seam tests use to
// point at a fixture without setting process-wide environment.
func OpenAt(ctx context.Context, path string, loc *time.Location) (*Reader, error) {
	if loc == nil {
		loc = time.Local
	}
	store, err := storage.OpenReadOnly(ctx, path, loc, nil)
	if err != nil {
		if storage.IsKind(err, storage.KindNotFound) {
			return nil, faultf(CodeNoData, "no Daygo database at "+path)
		}
		return nil, faultf(CodeInternal, "open database failed")
	}
	return &Reader{store: store, loc: loc, now: time.Now}, nil
}

// Close releases the read-only connection.
func (r *Reader) Close() error {
	if r == nil || r.store == nil {
		return nil
	}
	return r.store.Close()
}

// resolveDay maps "", "today", "yesterday", or a yyyy-MM-dd literal onto a
// logical day (docs/05 §5.9.1: aliases resolve at the entry layer only, so a
// client never derives the 4 AM boundary itself).
func (r *Reader) resolveDay(arg string) (string, error) {
	today := timeutil.LogicalDay(r.now(), r.loc)
	switch arg {
	case "", "today":
		return today, nil
	case "yesterday":
		base, err := time.ParseInLocation("2006-01-02", today, r.loc)
		if err != nil {
			return "", faultf(CodeInternal, "resolve yesterday failed")
		}
		return base.AddDate(0, 0, -1).Format("2006-01-02"), nil
	}
	if _, _, err := timeutil.DayWindow(arg, r.loc); err != nil {
		return "", faultf(CodeInvalidArgument, "day must be yyyy-MM-dd, today, or yesterday")
	}
	return arg, nil
}

// Status reports the database path and its schema version. It is a CLI-only
// command (the MCP tool face has no status; docs/05 §5.9.3).
func (r *Reader) Status(ctx context.Context) (StatusResult, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	var userVersion int
	err := r.store.Read(ctx, "agent status", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion)
	})
	if err != nil {
		return StatusResult{}, faultf(CodeInternal, "read database status failed")
	}
	return StatusResult{
		SchemaVersion: SchemaVersion,
		DatabasePath:  r.store.Path(),
		DBUserVersion: userVersion,
		GeneratedAt:   formatTime(r.now().Unix(), r.loc),
	}, nil
}

// Timeline returns the visible intersection of cards with one logical day;
// totals exclude System and count only minutes inside that day.
func (r *Reader) Timeline(ctx context.Context, dayArg string) (TimelineResult, error) {
	day, err := r.resolveDay(dayArg)
	if err != nil {
		return TimelineResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	start, end, _ := timeutil.DayWindow(day, r.loc)

	cards, err := r.store.Cards().CardsForDay(ctx, day)
	if err != nil {
		return TimelineResult{}, faultf(CodeInternal, "read timeline failed")
	}
	categories, err := r.store.Categories().List(ctx)
	if err != nil {
		return TimelineResult{}, faultf(CodeInternal, "read timeline failed")
	}
	idle := idleFlags(categories)

	result := TimelineResult{
		SchemaVersion: SchemaVersion,
		Day:           day,
		DayStartTs:    start.Unix(),
		DayEndTs:      end.Unix(),
		Cards:         make([]TimelineCard, 0, len(cards)),
	}
	for _, card := range cards {
		isIdle := idle[card.Category]
		visibleStart := max(card.StartTs, start.Unix())
		visibleEnd := min(card.EndTs, end.Unix())
		minutes := float64(visibleEnd-visibleStart) / 60
		result.Cards = append(result.Cards, TimelineCard{
			ID:              card.ID,
			Start:           card.Start,
			End:             card.End,
			StartTs:         visibleStart,
			EndTs:           visibleEnd,
			Category:        card.Category,
			Subcategory:     card.Subcategory,
			Title:           card.Title,
			IsIdle:          isIdle,
			DurationMinutes: minutes,
		})
		if card.Category == "System" {
			continue
		}
		if isIdle {
			result.IdleMinutes += minutes
		} else {
			result.TrackedMinutes += minutes
		}
	}
	return result, nil
}

// Card returns one non-deleted card by id.
func (r *Reader) Card(ctx context.Context, id int64) (CardResult, error) {
	if id <= 0 {
		return CardResult{}, faultf(CodeInvalidArgument, "card id must be a positive integer")
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	card, err := r.store.Cards().CardByID(ctx, id)
	if err != nil {
		if storage.IsKind(err, storage.KindNotFound) {
			return CardResult{}, faultf(CodeNotFound, "card not found")
		}
		return CardResult{}, faultf(CodeInternal, "read card failed")
	}
	categories, err := r.store.Categories().List(ctx)
	if err != nil {
		return CardResult{}, faultf(CodeInternal, "read card failed")
	}
	idle := idleFlags(categories)
	return CardResult{
		SchemaVersion:   SchemaVersion,
		ID:              card.ID,
		Day:             card.Day,
		Start:           card.Start,
		End:             card.End,
		StartTs:         card.StartTs,
		EndTs:           card.EndTs,
		Category:        card.Category,
		Subcategory:     card.Subcategory,
		Title:           card.Title,
		Summary:         card.Summary,
		DetailedSummary: card.DetailedSummary,
		IsIdle:          idle[card.Category],
		DurationMinutes: durationMinutes(card),
	}, nil
}

// idleFlags indexes is_idle by category name.
func idleFlags(list []domain.Category) map[string]bool {
	flags := make(map[string]bool, len(list))
	for _, c := range list {
		flags[c.Name] = c.IsIdle
	}
	return flags
}

func durationMinutes(card domain.TimelineCard) float64 {
	if card.EndTs <= card.StartTs {
		return 0
	}
	return float64(card.EndTs-card.StartTs) / 60.0
}
