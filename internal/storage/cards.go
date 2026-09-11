package storage

import (
	"context"
	"database/sql"
	"fmt"
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
			"SELECT "+cardColumns+" FROM timeline_cards WHERE day = ? AND is_deleted = 0 ORDER BY start_ts",
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
//  1. select the overlapping rows, keeping other batches' System cards so
//     failure markers stay visible;
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
		// Step 1: the overlap predicate of docs/03 §3.5. A System card is
		// deleted only when this batch wrote it; other batches' failure
		// markers stay visible through a neighboring rewrite.
		rows, err := tx.QueryContext(ctx, `
			SELECT id, video_summary_path FROM timeline_cards
			WHERE ((start_ts < ? AND end_ts > ?) OR (start_ts >= ? AND start_ts < ?))
			  AND is_deleted = 0
			  AND (category != 'System' OR batch_id = ?)`,
			to.Unix(), from.Unix(), from.Unix(), to.Unix(), batchID)
		if err != nil {
			return wrap("select overlapping cards", err)
		}
		var victimIDs []int64
		var videoPaths []string
		for rows.Next() {
			var id int64
			var video sql.NullString
			if err := rows.Scan(&id, &video); err != nil {
				_ = rows.Close()
				return wrap("scan overlapping card", err)
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
				endTs = endTs.AddDate(0, 0, 1)
			}
			day := timeutil.LogicalDay(startTs, loc)

			res, err := tx.ExecContext(ctx, `
				INSERT INTO timeline_cards
					(batch_id, day, start, end, start_ts, end_ts, category, subcategory,
					 title, summary, detailed_summary, video_summary_path, metadata,
					 is_deleted, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
				batchID, day, shell.Start, shell.End, startTs.Unix(), endTs.Unix(),
				shell.Category, nullable(shell.Subcategory), shell.Title, shell.Summary,
				nullable(shell.DetailedSummary), nullable(shell.VideoSummaryPath),
				nullable(shell.Metadata), at, at)
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
