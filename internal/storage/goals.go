package storage

import (
	"context"
	"database/sql"
	"time"
)

// Goal category roles are the closed set from docs/03 §3.3.4.
const (
	GoalRoleFocus       = "focus"
	GoalRoleDistraction = "distraction"
)

// GoalCategoryRef is one day_goal_categories row joined with its category row
// for presentation (docs/05 §5.5.2 GoalCategoryRefDTO). Role is GoalRoleFocus
// or GoalRoleDistraction.
type GoalCategoryRef struct {
	CategoryID string
	Name       string
	ColorHex   string
	SortOrder  int
	Role       string
}

// DayGoal is one row of day_goals.
type DayGoal struct {
	Day                     string // logical day yyyy-MM-dd
	FocusTargetMinutes      int
	DistractionLimitMinutes int
	IsSkipped               bool
	UpdatedAt               time.Time
}

// GoalRepo is the typed access layer over day_goals / day_goal_categories.
type GoalRepo struct {
	store *Store
}

// Goals returns the repository bound to this store.
func (s *Store) Goals() *GoalRepo {
	if s == nil {
		return nil
	}
	return &GoalRepo{store: s}
}

// Get returns the goal for one logical day plus its category references,
// ordered by sort_order then name. found is false when no row exists; that is
// "no goal set yet", not an error.
func (r *GoalRepo) Get(ctx context.Context, day string) (DayGoal, []GoalCategoryRef, bool, error) {
	var goal DayGoal
	var isSkipped int
	var updatedAt int64
	found := false
	err := r.store.Read(ctx, "goal get", func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`SELECT day, focus_target_minutes, distraction_limit_minutes, is_skipped, updated_at
			 FROM day_goals WHERE day = ?`, day)
		if err := row.Scan(&goal.Day, &goal.FocusTargetMinutes, &goal.DistractionLimitMinutes,
			&isSkipped, &updatedAt); err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}
		goal.IsSkipped = isSkipped != 0
		goal.UpdatedAt = time.Unix(updatedAt, 0)
		found = true
		return nil
	})
	if err != nil || !found {
		return DayGoal{}, nil, false, err
	}

	var refs []GoalCategoryRef
	err = r.store.Read(ctx, "goal categories get", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT gc.category_id, c.name, c.color_hex, gc.sort_order, gc.role
			FROM day_goal_categories gc
			JOIN categories c ON c.id = gc.category_id
			WHERE gc.day = ?
			ORDER BY gc.sort_order, c.name`, day)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var ref GoalCategoryRef
			if err := rows.Scan(&ref.CategoryID, &ref.Name, &ref.ColorHex, &ref.SortOrder, &ref.Role); err != nil {
				return err
			}
			refs = append(refs, ref)
		}
		return rows.Err()
	})
	if err != nil {
		return DayGoal{}, nil, false, err
	}
	return goal, refs, true, nil
}

// Save inserts or replaces one day's goal and, in the same transaction, fully
// replaces its category references. Category existence is validated by the
// caller before this runs; the foreign key here is the backstop, not the
// user-facing validation.
func (r *GoalRepo) Save(ctx context.Context, goal DayGoal, refs []GoalCategoryRef) error {
	at := r.store.now().Unix()
	skipped := 0
	if goal.IsSkipped {
		skipped = 1
	}
	return r.store.Write(ctx, "goal save", func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO day_goals (day, focus_target_minutes, distraction_limit_minutes, is_skipped, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(day) DO UPDATE SET
				focus_target_minutes      = excluded.focus_target_minutes,
				distraction_limit_minutes = excluded.distraction_limit_minutes,
				is_skipped                = excluded.is_skipped,
				updated_at                = excluded.updated_at`,
			goal.Day, goal.FocusTargetMinutes, goal.DistractionLimitMinutes, skipped, at); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM day_goal_categories WHERE day = ?`, goal.Day); err != nil {
			return err
		}
		for i, ref := range refs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO day_goal_categories (day, category_id, role, sort_order)
				VALUES (?, ?, ?, ?)`,
				goal.Day, ref.CategoryID, ref.Role, i); err != nil {
				return err
			}
		}
		return nil
	})
}
