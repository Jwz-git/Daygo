package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// CardRepo implements the cards capability over timeline_cards
// (docs/05 §5.6.2 TimelineRepository). It is the pipeline's atomic commit
// point: ReplaceCardsInRange holds the whole rewrite in one transaction.
type CardRepo struct {
	store *Store
}

// Cards returns the repository bound to this store.
func (s *Store) Cards() *CardRepo {
	if s == nil {
		return nil
	}
	return &CardRepo{store: s}
}

// ReplaceResult reports what one ReplaceCardsInRange did (docs/05 §5.6.2).
// SkippedCards must be consumed by the caller and counted in diagnostics;
// a skipped card is a defect to look at, never a silent default.
type ReplaceResult struct {
	InsertedIDs       []int64
	DeletedVideoPaths []string
	SkippedCards      []domain.CardShell
}

const cardColumns = `id, batch_id, day, start, end, start_ts, end_ts, category, subcategory,
	title, summary, detailed_summary, video_summary_path, metadata, is_deleted, created_at, updated_at`

// CardsForDay returns the non-deleted cards of one logical day, ordered by
// start time. The day string is a logical day (4 AM boundary); callers obtain
// it from timeutil, never from a calendar date.
func (r *CardRepo) CardsForDay(ctx context.Context, day string) ([]domain.TimelineCard, error) {
	var out []domain.TimelineCard
	err := r.store.Read(ctx, "cards for day "+day, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			"SELECT "+cardColumns+" FROM timeline_cards WHERE day = ? AND is_deleted = 0 AND end_ts > start_ts AND (end_ts - start_ts) <= 14400 ORDER BY start_ts",
			day)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			c, err := scanCard(rows)
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CardsInRange returns the non-deleted cards overlapping [from, to). The
// predicate is the overlap rule of docs/03 §3.5: an activity that starts
// inside the window counts even if it ends after the window.
func (r *CardRepo) CardsInRange(ctx context.Context, from, to time.Time) ([]domain.TimelineCard, error) {
	var out []domain.TimelineCard
	err := r.store.Read(ctx, "cards in range", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT `+cardColumns+` FROM timeline_cards
			 WHERE ((start_ts < ? AND end_ts > ?) OR (start_ts >= ? AND start_ts < ?))
			   AND is_deleted = 0
			   AND end_ts > start_ts
			   AND (end_ts - start_ts) <= 14400
			 ORDER BY start_ts`,
			to.Unix(), from.Unix(), from.Unix(), to.Unix())
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			c, err := scanCard(rows)
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CardByID returns one non-deleted card.
func (r *CardRepo) CardByID(ctx context.Context, id int64) (domain.TimelineCard, error) {
	var card domain.TimelineCard
	err := r.store.Read(ctx, "card by id", func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			"SELECT "+cardColumns+" FROM timeline_cards WHERE id = ? AND is_deleted = 0", id)
		var err error
		card, err = scanCard(row)
		return err
	})
	if err != nil {
		return domain.TimelineCard{}, err
	}
	return card, nil
}

// CardsForBatch returns all non-deleted cards written by one batch.
func (r *CardRepo) CardsForBatch(ctx context.Context, batchID int64) ([]domain.TimelineCard, error) {
	var out []domain.TimelineCard
	err := r.store.Read(ctx, "cards for batch", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			"SELECT "+cardColumns+" FROM timeline_cards WHERE batch_id = ? AND is_deleted = 0 ORDER BY start_ts",
			batchID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			c, err := scanCard(rows)
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ReplaceCardsInRange rewrites the cards of [from, to) with the pipeline's
// output, in ONE transaction (docs/03 §3.5):
//
//  1. select the overlapping rows — every card in range, System fallback
//     cards included: a merge that extends into a neighbor's System card
//     must absorb it, or the two sit in parallel over the same stretch
//     (failure state lives in analysis_batches, not in cards);
//  2. soft-delete them and collect video paths for out-of-transaction cleanup;
//  3. derive start_ts/end_ts/day per card with the three-day anchor rule;
//     shells whose clock strings do not resolve go to SkippedCards — the
//     transaction still commits the resolvable ones;
//  4. insert the resolvable cards.
//
// Range serialization (read → generate → rewrite per overlapping window) is
// the analysis scheduler's job; this method only guarantees atomicity.
func (r *CardRepo) ReplaceCardsInRange(ctx context.Context, from, to time.Time,
	cards []domain.CardShell, batchID int64) (ReplaceResult, error) {

	var result ReplaceResult
	at := r.store.now().Unix()
	loc := r.store.location()

	err := r.store.Write(ctx, "replace cards in range", func(ctx context.Context, tx *sql.Tx) error {
		// Resolve shells first so the effective deletion range covers both the
		// batch window [from, to) and any backwards/forwards merged cards.
		// When the model merges with an earlier card, its start extends before
		// `from`, so effectiveFrom must expand to absorb the merged predecessor.
		type resolvedCard struct {
			shell   domain.CardShell
			startTs time.Time
			endTs   time.Time
			day     string
		}
		var resolved []resolvedCard
		effectiveFrom := from
		effectiveTo := to
		anchor := from.Add(to.Sub(from) / 2)

		for _, shell := range cards {
			startTs, err := timeutil.ResolveClock(shell.Start, anchor, loc)
			if err != nil {
				result.SkippedCards = append(result.SkippedCards, shell)
				continue
			}
			endTs, err := timeutil.ResolveClock(shell.End, anchor, loc)
			if err != nil {
				result.SkippedCards = append(result.SkippedCards, shell)
				continue
			}
			if endTs.Before(startTs) {
				candidate := endTs.AddDate(0, 0, 1)
				if candidate.Sub(startTs) <= 4*time.Hour {
					endTs = candidate
				}
			}
			if !endTs.After(startTs) || endTs.Sub(startTs) > 4*time.Hour {
				result.SkippedCards = append(result.SkippedCards, shell)
				continue
			}
			if startTs.Before(effectiveFrom) {
				effectiveFrom = startTs
			}
			if endTs.After(effectiveTo) {
				effectiveTo = endTs
			}
			day := timeutil.LogicalDay(startTs, loc)
			resolved = append(resolved, resolvedCard{
				shell:   shell,
				startTs: startTs,
				endTs:   endTs,
				day:     day,
			})
		}

		// Step 1: the overlap predicate of docs/03 §3.5 over the expanded span.
		rows, err := tx.QueryContext(ctx, `
			SELECT id, start_ts, end_ts, video_summary_path FROM timeline_cards
			WHERE ((start_ts < ? AND end_ts > ?) OR (start_ts >= ? AND start_ts < ?))
			  AND is_deleted = 0`,
			effectiveTo.Unix(), effectiveFrom.Unix(), effectiveFrom.Unix(), effectiveTo.Unix())
		if err != nil {
			return wrap("select overlapping cards", err)
		}
		var victimIDs []int64
		var videoPaths []string
		for rows.Next() {
			var id int64
			var startTs, endTs int64
			var video sql.NullString
			if err := rows.Scan(&id, &startTs, &endTs, &video); err != nil {
				_ = rows.Close()
				return wrap("scan overlapping card", err)
			}
			// Deleting an overlap removes the whole row. Refuse a rewrite whose
			// owned span does not cover that whole row; otherwise a tiny overlap
			// can silently erase hours outside the regenerated range.
			if startTs < effectiveFrom.Unix() || endTs > effectiveTo.Unix() {
				_ = rows.Close()
				return newError(KindConstraint, fmt.Sprintf(
					"replace cards in range: card %d spans outside rewrite ownership", id))
			}
			victimIDs = append(victimIDs, id)
			if video.Valid && video.String != "" {
				videoPaths = append(videoPaths, video.String)
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return wrap("iterate overlapping cards", err)
		}
		_ = rows.Close()

		for _, id := range victimIDs {
			if _, err := tx.ExecContext(ctx,
				"UPDATE timeline_cards SET is_deleted = 1, updated_at = ? WHERE id = ?", at, id); err != nil {
				return wrap("soft-delete card", err)
			}
		}

		for _, rc := range resolved {
			res, err := tx.ExecContext(ctx, `
				INSERT INTO timeline_cards
					(batch_id, day, start, end, start_ts, end_ts, category, subcategory,
					 title, summary, detailed_summary, video_summary_path, metadata,
					 is_deleted, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
				batchID, rc.day, rc.shell.Start, rc.shell.End, rc.startTs.Unix(), rc.endTs.Unix(),
				rc.shell.Category, nullable(rc.shell.Subcategory), rc.shell.Title, rc.shell.Summary,
				nullable(rc.shell.DetailedSummary), nullable(rc.shell.VideoSummaryPath),
				nullable(rc.shell.Metadata), at, at)
			if err != nil {
				return wrap("insert card", err)
			}
			id, err := res.LastInsertId()
			if err != nil {
				return wrap("insert card id", err)
			}
			result.InsertedIDs = append(result.InsertedIDs, id)
		}
		result.DeletedVideoPaths = videoPaths
		return nil
	})
	if err != nil {
		return ReplaceResult{}, err
	}
	if len(result.SkippedCards) > 0 {
		NoteSkippedCards(int64(len(result.SkippedCards)))
	}
	return result, nil
}

// UpdateCardCategory moves one card to another category name.
func (r *CardRepo) UpdateCardCategory(ctx context.Context, id int64, category string) error {
	return r.updateCardColumn(ctx, "update card category", id,
		"UPDATE timeline_cards SET category = ?, updated_at = ? WHERE id = ? AND is_deleted = 0",
		category)
}

// UpdateCardTitle renames one card.
func (r *CardRepo) UpdateCardTitle(ctx context.Context, id int64, title string) error {
	return r.updateCardColumn(ctx, "update card title", id,
		"UPDATE timeline_cards SET title = ?, updated_at = ? WHERE id = ? AND is_deleted = 0",
		title)
}

// UpdateCardSummary rewrites one card's short summary. Empty is valid: the
// pane then shows its no-summary placeholder.
func (r *CardRepo) UpdateCardSummary(ctx context.Context, id int64, text string) error {
	return r.updateCardColumn(ctx, "update card summary", id,
		"UPDATE timeline_cards SET summary = ?, updated_at = ? WHERE id = ? AND is_deleted = 0",
		text)
}

// UpdateCardDetailedSummary rewrites one card's long-form summary. Empty is
// valid: the detail pane falls back to the short summary.
func (r *CardRepo) UpdateCardDetailedSummary(ctx context.Context, id int64, text string) error {
	return r.updateCardColumn(ctx, "update card summary", id,
		"UPDATE timeline_cards SET detailed_summary = ?, updated_at = ? WHERE id = ? AND is_deleted = 0",
		text)
}

// SoftDeleteCard hides one card and returns its video path for cleanup. File
// deletion is the caller's, out-of-transaction business: a file operation
// cannot roll back with the database.
func (r *CardRepo) SoftDeleteCard(ctx context.Context, id int64) (string, error) {
	var videoPath string
	at := r.store.now().Unix()
	err := r.store.Write(ctx, "soft-delete card", func(ctx context.Context, tx *sql.Tx) error {
		var video sql.NullString
		err := tx.QueryRowContext(ctx,
			"SELECT video_summary_path FROM timeline_cards WHERE id = ? AND is_deleted = 0", id).
			Scan(&video)
		if err != nil {
			return wrap("select card for delete", err)
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE timeline_cards SET is_deleted = 1, updated_at = ? WHERE id = ?", at, id); err != nil {
			return wrap("soft-delete card", err)
		}
		videoPath = video.String
		return nil
	})
	if err != nil {
		return "", err
	}
	return videoPath, nil
}

// TotalMinutesTracked sums card durations over [from, to), excluding the
// System category (docs/modules/timeline: totals exclude System). The
// denominator decision — whether idle categories count — belongs to the
// caller composing totals, not to this sum.
func (r *CardRepo) TotalMinutesTracked(ctx context.Context, from, to time.Time) (float64, error) {
	var total sql.NullFloat64
	err := r.store.Read(ctx, "total minutes tracked", func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT SUM(CASE WHEN end_ts > start_ts THEN (end_ts - start_ts) ELSE 0 END) / 60.0
			FROM timeline_cards
			WHERE ((start_ts < ? AND end_ts > ?) OR (start_ts >= ? AND start_ts < ?))
			  AND is_deleted = 0 AND category != 'System'`,
			to.Unix(), from.Unix(), from.Unix(), to.Unix())
		return row.Scan(&total)
	})
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// CardDaysByCategory lists the distinct logical days holding live cards in
// any of the named categories. The category rename path uses it to emit
// timeline:updated for exactly the days whose cards were rewritten.
func (r *CardRepo) CardDaysByCategory(ctx context.Context, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(names)), ",")
	args := make([]any, len(names))
	for i, name := range names {
		args[i] = name
	}
	var days []string
	err := r.store.Read(ctx, "card days by category", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			"SELECT DISTINCT day FROM timeline_cards WHERE is_deleted = 0 AND category IN ("+placeholders+")",
			args...)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var day string
			if err := rows.Scan(&day); err != nil {
				return err
			}
			days = append(days, day)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return days, nil
}

// EarliestCardStart returns the earliest start_ts across live cards. found is
// false when no live card exists — an empty database, not an error. The
// standup backfill uses it to bound how far back to look for missing recaps.
func (r *CardRepo) EarliestCardStart(ctx context.Context) (time.Time, bool, error) {
	var earliest sql.NullInt64
	err := r.store.Read(ctx, "earliest card start", func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx,
			"SELECT MIN(start_ts) FROM timeline_cards WHERE is_deleted = 0").Scan(&earliest)
	})
	if err != nil {
		return time.Time{}, false, err
	}
	if !earliest.Valid {
		return time.Time{}, false, nil
	}
	return time.Unix(earliest.Int64, 0), true, nil
}

// updateCardColumn is the shared single-column update: write, then translate
// zero affected rows into not_found so the binding layer maps it to 404 rather
// than reporting success over nothing.
func (r *CardRepo) updateCardColumn(ctx context.Context, op string, id int64, query string, value string) error {
	at := r.store.now().Unix()
	return r.store.Write(ctx, op, func(ctx context.Context, tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, query, value, at, id)
		if err != nil {
			return wrap(op, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return wrap(op, err)
		}
		if n == 0 {
			return newError(KindNotFound, fmt.Sprintf("%s: card %d", op, id))
		}
		return nil
	})
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func scanCard(row scanner) (domain.TimelineCard, error) {
	var c domain.TimelineCard
	var batchID sql.NullInt64
	var subcategory, detailed, video, metadata sql.NullString
	var isDeleted int
	if err := row.Scan(&c.ID, &batchID, &c.Day, &c.Start, &c.End, &c.StartTs, &c.EndTs,
		&c.Category, &subcategory, &c.Title, &c.Summary, &detailed, &video, &metadata,
		&isDeleted, &c.CreatedAtUnix, &c.UpdatedAtUnix); err != nil {
		return c, wrap("scan card", err)
	}
	if batchID.Valid {
		id := batchID.Int64
		c.BatchID = &id
	}
	c.Subcategory = subcategory.String
	c.DetailedSummary = detailed.String
	c.VideoSummaryPath = video.String
	c.Metadata = metadata.String
	c.Deleted = isDeleted == 1
	return c, nil
}
