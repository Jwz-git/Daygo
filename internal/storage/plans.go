package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Plan block statuses: the explicit completion mark (docs/modules/plan.md).
// Coverage by recorded cards is derived on read and never stored.
const (
	PlanStatusPlanned = "planned"
	PlanStatusDone    = "done"
	PlanStatusSkipped = "skipped"
)

// ValidPlanStatus reports whether status is one of the closed set.
func ValidPlanStatus(status string) bool {
	switch status {
	case PlanStatusPlanned, PlanStatusDone, PlanStatusSkipped:
		return true
	default:
		return false
	}
}

// PlanBlock is one row of plan_blocks, joined with its category for
// presentation. CategoryID is empty when the block has no category (or its
// category was deleted).
type PlanBlock struct {
	ID            int64
	Day           string // logical day yyyy-MM-dd
	StartTs       int64
	EndTs         int64
	Title         string
	Notes         *string
	CategoryID    string
	CategoryName  string
	CategoryColor string
	Status        string
	CompletedAt   *time.Time
	Remind        bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// PlanRepo is the typed access layer over plan_blocks.
type PlanRepo struct {
	store *Store
}

// Plans returns the repository bound to this store.
func (s *Store) Plans() *PlanRepo {
	if s == nil {
		return nil
	}
	return &PlanRepo{store: s}
}

const planSelect = `
	SELECT p.id, p.day, p.start_ts, p.end_ts, p.title, p.notes,
	       COALESCE(p.category_id, ''), COALESCE(c.name, ''), COALESCE(c.color_hex, ''),
	       p.status, p.completed_at, p.remind, p.created_at, p.updated_at
	FROM plan_blocks p
	LEFT JOIN categories c ON c.id = p.category_id`

func scanPlanBlock(scan func(...any) error) (PlanBlock, error) {
	var block PlanBlock
	var completedAt sql.NullInt64
	var remind int
	var createdAt, updatedAt int64
	if err := scan(&block.ID, &block.Day, &block.StartTs, &block.EndTs, &block.Title, &block.Notes,
		&block.CategoryID, &block.CategoryName, &block.CategoryColor,
		&block.Status, &completedAt, &remind, &createdAt, &updatedAt); err != nil {
		return PlanBlock{}, err
	}
	if completedAt.Valid {
		at := time.Unix(completedAt.Int64, 0)
		block.CompletedAt = &at
	}
	block.Remind = remind != 0
	block.CreatedAt = time.Unix(createdAt, 0)
	block.UpdatedAt = time.Unix(updatedAt, 0)
	return block, nil
}

// ForDay returns one logical day's blocks ordered by start time, then id.
func (r *PlanRepo) ForDay(ctx context.Context, day string) ([]PlanBlock, error) {
	blocks := []PlanBlock{}
	err := r.store.Read(ctx, "plan day", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, planSelect+` WHERE p.day = ? ORDER BY p.start_ts, p.id`, day)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			block, err := scanPlanBlock(rows.Scan)
			if err != nil {
				return err
			}
			blocks = append(blocks, block)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return blocks, nil
}

// Get returns one block, or a KindNotFound error.
func (r *PlanRepo) Get(ctx context.Context, id int64) (PlanBlock, error) {
	var block PlanBlock
	err := r.store.Read(ctx, "plan get", func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, planSelect+` WHERE p.id = ?`, id)
		found, err := scanPlanBlock(row.Scan)
		if err == sql.ErrNoRows {
			return newError(KindNotFound, fmt.Sprintf("plan get: block %d", id))
		}
		if err != nil {
			return err
		}
		block = found
		return nil
	})
	return block, err
}

// Insert adds a block and returns its id. Status starts as planned.
func (r *PlanRepo) Insert(ctx context.Context, block PlanBlock) (int64, error) {
	at := r.store.now().Unix()
	var id int64
	err := r.store.Write(ctx, "plan insert", func(ctx context.Context, tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO plan_blocks (day, start_ts, end_ts, title, notes, category_id, status, remind, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			block.Day, block.StartTs, block.EndTs, block.Title, block.Notes, nullableID(block.CategoryID),
			PlanStatusPlanned, boolInt(block.Remind), at, at)
		if err != nil {
			return err
		}
		id, err = result.LastInsertId()
		return err
	})
	return id, err
}

// Update replaces a block's editable fields (time, title, notes, category,
// remind). The completion mark is changed only through SetStatus.
func (r *PlanRepo) Update(ctx context.Context, block PlanBlock) error {
	at := r.store.now().Unix()
	return r.store.Write(ctx, "plan update", func(ctx context.Context, tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE plan_blocks
			SET day = ?, start_ts = ?, end_ts = ?, title = ?, notes = ?, category_id = ?, remind = ?, updated_at = ?
			WHERE id = ?`,
			block.Day, block.StartTs, block.EndTs, block.Title, block.Notes, nullableID(block.CategoryID),
			boolInt(block.Remind), at, block.ID)
		if err != nil {
			return err
		}
		return requireOneRow(result, fmt.Sprintf("plan update: block %d", block.ID))
	})
}

// SetStatus records the completion mark. done stamps completed_at; any other
// status clears it.
func (r *PlanRepo) SetStatus(ctx context.Context, id int64, status string) error {
	at := r.store.now().Unix()
	var completedAt any
	if status == PlanStatusDone {
		completedAt = at
	}
	return r.store.Write(ctx, "plan status", func(ctx context.Context, tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE plan_blocks SET status = ?, completed_at = ?, updated_at = ? WHERE id = ?`,
			status, completedAt, at, id)
		if err != nil {
			return err
		}
		return requireOneRow(result, fmt.Sprintf("plan status: block %d", id))
	})
}

// Delete removes a block.
func (r *PlanRepo) Delete(ctx context.Context, id int64) error {
	return r.store.Write(ctx, "plan delete", func(ctx context.Context, tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `DELETE FROM plan_blocks WHERE id = ?`, id)
		if err != nil {
			return err
		}
		return requireOneRow(result, fmt.Sprintf("plan delete: block %d", id))
	})
}

func requireOneRow(result sql.Result, op string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return newError(KindNotFound, op)
	}
	return nil
}

func nullableID(id string) any {
	if id == "" {
		return nil
	}
	return id
}
