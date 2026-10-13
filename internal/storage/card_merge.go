package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

// ErrInvalidCardMerge identifies a rejected merge without matching error text.
var ErrInvalidCardMerge = errors.New("invalid card merge")

func invalidCardMerge(reason string) error { return fmt.Errorf("%w: %s", ErrInvalidCardMerge, reason) }

// MergeCards creates a human-edited card and soft-deletes exactly the selected
// sources, in one transaction. No files or source feedback are deleted. The
// persisted timestamps determine the range; clock strings are never re-parsed.
func (r *CardRepo) MergeCards(ctx context.Context, ids []int64, content domain.CardShell) (domain.TimelineCard, error) {
	if len(ids) < 2 || len(ids) > 32 {
		return domain.TimelineCard{}, invalidCardMerge("select 2 to 32 cards")
	}
	selected := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || selected[id] {
			return domain.TimelineCard{}, invalidCardMerge("card ids must be positive and distinct")
		}
		selected[id] = true
	}
	content.Title, content.Category = strings.TrimSpace(content.Title), strings.TrimSpace(content.Category)
	if content.Title == "" || utf8.RuneCountInString(content.Title) > 200 ||
		strings.TrimSpace(content.Summary) == "" || utf8.RuneCountInString(content.Summary) > 4000 ||
		utf8.RuneCountInString(content.DetailedSummary) > 16000 || content.Category == "" || utf8.RuneCountInString(content.Category) > 64 {
		return domain.TimelineCard{}, invalidCardMerge("title, summary and an existing user category are required; text exceeds limits or is blank")
	}
	var merged domain.TimelineCard
	err := r.store.Write(ctx, "merge cards", func(ctx context.Context, tx *sql.Tx) error {
		var builtin bool
		err := tx.QueryRowContext(ctx, "SELECT is_system FROM categories WHERE name=?", content.Category).Scan(&builtin)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && builtin) {
			return invalidCardMerge("target category must be an existing non-built-in category")
		}
		if err != nil {
			return err
		}
		cards := make([]domain.TimelineCard, 0, len(ids))
		for _, id := range ids {
			card, err := scanCard(tx.QueryRowContext(ctx, "SELECT "+cardColumns+" FROM timeline_cards WHERE id=? AND is_deleted=0", id))
			if err != nil {
				return err
			}
			var systemCount int
			if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM categories WHERE name=? AND is_system=1", card.Category).Scan(&systemCount); err != nil {
				return err
			}
			if systemCount > 0 {
				return invalidCardMerge("built-in category cards cannot be merged")
			}
			if card.EndTs <= card.StartTs || card.EndTs-card.StartTs > 14400 {
				return invalidCardMerge("source card has an invalid time range")
			}
			cards = append(cards, card)
		}
		sort.Slice(cards, func(i, j int) bool {
			if cards[i].StartTs == cards[j].StartTs {
				return cards[i].ID < cards[j].ID
			}
			return cards[i].StartTs < cards[j].StartTs
		})
		first, last := cards[0], cards[0]
		for _, card := range cards[1:] {
			if card.StartTs > last.EndTs {
				return invalidCardMerge("selected cards must be contiguous or overlapping; gaps cannot be filled")
			}
			if card.EndTs > last.EndTs {
				last = card
			}
		}
		if last.EndTs-first.StartTs > 14400 {
			return invalidCardMerge("merged range must not exceed four hours")
		}
		rows, err := tx.QueryContext(ctx, "SELECT id FROM timeline_cards WHERE is_deleted=0 AND start_ts < ? AND end_ts > ?", last.EndTs, first.StartTs)
		if err != nil {
			return err
		}
		unselected := false
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return err
			}
			if !selected[id] {
				unselected = true
			}
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return err
		}
		if unselected {
			return invalidCardMerge("an unselected live card overlaps the merge range")
		}
		metadata, err := mergedCardMetadata(cards)
		if err != nil {
			return err
		}
		at := r.store.now().Unix()
		for _, card := range cards {
			if _, err := tx.ExecContext(ctx, "UPDATE timeline_cards SET is_deleted=1, is_user_edited=1, updated_at=? WHERE id=?", at, card.ID); err != nil {
				return err
			}
		}
		day := timeutil.LogicalDay(time.Unix(first.StartTs, 0), r.store.location())
		var batchID any
		for _, card := range cards {
			if card.BatchID != nil {
				batchID = *card.BatchID
				break
			}
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO timeline_cards
			(batch_id, day, start, end, start_ts, end_ts, category, title, summary, detailed_summary,
			 metadata, is_deleted, is_user_edited, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 1, ?, ?)`,
			batchID, day, first.Start, last.End, first.StartTs, last.EndTs, content.Category,
			content.Title, content.Summary, nullable(content.DetailedSummary), metadata, at, at)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		merged, err = scanCard(tx.QueryRowContext(ctx, "SELECT "+cardColumns+" FROM timeline_cards WHERE id=?", id))
		return err
	})
	if err != nil {
		return domain.TimelineCard{}, err
	}
	return merged, nil
}

// Known display evidence is carried forward in time order. Complete original
// metadata, videos and feedback remain on the soft-deleted source rows.
func mergedCardMetadata(cards []domain.TimelineCard) (string, error) {
	points, distractions := []json.RawMessage{}, []json.RawMessage{}
	ids := make([]int64, 0, len(cards))
	apps := []string{}
	for _, card := range cards {
		ids = append(ids, card.ID)
		if card.Metadata == "" {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal([]byte(card.Metadata), &fields) != nil || fields == nil {
			return "", invalidCardMerge("source metadata is invalid")
		}
		for key, target := range map[string]*[]json.RawMessage{"activityPoints": &points, "distractions": &distractions} {
			if raw, ok := fields[key]; ok {
				var items []json.RawMessage
				if json.Unmarshal(raw, &items) != nil {
					return "", invalidCardMerge("source evidence is invalid")
				}
				for _, item := range items {
					var object map[string]json.RawMessage
					if json.Unmarshal(item, &object) != nil || object == nil {
						return "", invalidCardMerge("source evidence is invalid")
					}
					if key == "distractions" {
						delete(object, "id")
						item, _ = json.Marshal(object)
					}
					*target = append(*target, item)
				}
			}
		}
		if raw, ok := fields["appSites"]; ok {
			var sites struct {
				Primary   *string `json:"primary"`
				Secondary *string `json:"secondary"`
			}
			if json.Unmarshal(raw, &sites) != nil {
				return "", invalidCardMerge("source app sites are invalid")
			}
			for _, app := range []*string{sites.Primary, sites.Secondary} {
				if app != nil && *app != "" {
					found := false
					for _, existing := range apps {
						if existing == *app {
							found = true
						}
					}
					if !found {
						apps = append(apps, *app)
					}
				}
			}
		}
	}
	meta := map[string]any{"mergedCardIds": ids, "activityPoints": points, "distractions": distractions}
	if len(apps) > 0 {
		sites := map[string]string{"primary": apps[0]}
		if len(apps) > 1 {
			sites["secondary"] = apps[1]
		}
		meta["appSites"] = sites
	}
	raw, err := json.Marshal(meta)
	return string(raw), err
}
