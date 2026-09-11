package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"

	"github.com/Jwz-git/Daygo/internal/domain"
)

// CategoryRepo is the categories access layer (docs/03 §3.3.3). Categories
// are first-class entities: timeline_cards.category stores the name string,
// so a rename must rewrite existing cards in the same transaction — that is
// this repository's job, never the frontend's.
type CategoryRepo struct {
	store *Store
}

// Categories returns the repository bound to this store.
func (s *Store) Categories() *CategoryRepo {
	if s == nil {
		return nil
	}
	return &CategoryRepo{store: s}
}

// List returns every category ordered by sort_order, then name.
func (r *CategoryRepo) List(ctx context.Context) ([]domain.Category, error) {
	var out []domain.Category
	err := r.store.Read(ctx, "categories list", func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at
			FROM categories
			ORDER BY sort_order, name`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			c, err := scanCategory(rows)
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

// ByName returns the category stored under name.
func (r *CategoryRepo) ByName(ctx context.Context, name string) (domain.Category, bool, error) {
	var c domain.Category
	err := r.store.Read(ctx, "categories by name", func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at
			FROM categories WHERE name = ?`, name)
		var err error
		c, err = scanCategory(row)
		return err
	})
	if err != nil {
		if IsKind(err, KindNotFound) {
			return domain.Category{}, false, nil
		}
		return domain.Category{}, false, err
	}
	return c, true, nil
}

// Save replaces the category set wholesale (docs/05: SaveCategories is an
// idempotent whole-set overwrite) in one transaction that also rewrites
// timeline_cards for every rename. Either every change lands or none does.
//
// Rules enforced here, inside the transaction:
//   - names in the input must be unique and non-empty;
//   - built-in categories (is_system) must be present, keep their name, and
//     stay system — the overlap predicate and totals both key on the literal
//     "System" name (docs/03 §3.5), so it cannot drift;
//   - a rename to a name another input row still holds is a constraint error,
//     not a silent last-write-wins;
//   - deleting a category is allowed (it is not is_system); existing cards
//     keep their stored name string, which the caller surfaces as an unknown
//     category rather than this layer rewriting history.
//
// Input rows with an empty ID are new; storage generates the UUID so the
// identifier is assigned exactly once, at the write.
func (r *CategoryRepo) Save(ctx context.Context, cats []domain.Category) error {
	for i, c := range cats {
		if c.Name == "" {
			return newError(KindConstraint, fmt.Sprintf("save categories: row %d has an empty name", i))
		}
		if c.IsSystem {
			return newError(KindConstraint, fmt.Sprintf(
				"save categories: row %d (%q) claims is_system; built-ins are assigned by the database", i, c.Name))
		}
	}

	at := r.store.now().Unix()
	return r.store.Write(ctx, "categories save", func(ctx context.Context, tx *sql.Tx) error {
		existing, err := loadCategories(ctx, tx)
		if err != nil {
			return err
		}

		// Merge the built-ins into the input so the write set always contains
		// them; their identity is fixed and cannot come from the caller.
		merged := make([]domain.Category, 0, len(cats)+len(existing))
		builtIns := 0
		for _, e := range existing {
			if e.IsSystem {
				merged = append(merged, e)
				builtIns++
			}
		}
		merged = append(merged, cats...)

		if err := checkCategoryNames(merged); err != nil {
			return err
		}

		// Names that survive under a different id are renames: the card rows
		// must follow the name before it is released, or cards would point at
		// a category that no longer exists under that string.
		byID := make(map[string]domain.Category, len(existing))
		for _, e := range existing {
			byID[e.ID] = e
		}
		renames := make(map[string]string) // old name -> new name
		for _, c := range cats {
			e, ok := byID[c.ID]
			if !ok || e.Name == c.Name {
				continue
			}
			renames[e.Name] = c.Name
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM categories WHERE is_system = 0`); err != nil {
			return wrap("delete categories", err)
		}
		for i := range merged {
			if err := upsertCategory(ctx, tx, merged[i], at); err != nil {
				return err
			}
		}
		for oldName, newName := range renames {
			if _, err := tx.ExecContext(ctx, `
				UPDATE timeline_cards SET category = ?, updated_at = ? WHERE category = ?`,
				newName, at, oldName); err != nil {
				return wrap("rewrite cards for rename "+oldName+" -> "+newName, err)
			}
		}
		return nil
	})
}

// checkCategoryNames rejects duplicate names across the merged set. The
// UNIQUE constraint would also catch this, but only after some rows were
// written; rejecting first keeps the transaction a pure no-op on bad input.
func checkCategoryNames(cats []domain.Category) error {
	seen := make(map[string]struct{}, len(cats))
	for _, c := range cats {
		if _, dup := seen[c.Name]; dup {
			return newError(KindConstraint, fmt.Sprintf("save categories: duplicate name %q", c.Name))
		}
		seen[c.Name] = struct{}{}
	}
	return nil
}

// loadCategories reads the full current set inside the caller's transaction.
func loadCategories(ctx context.Context, tx *sql.Tx) ([]domain.Category, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at
		FROM categories`)
	if err != nil {
		return nil, wrap("load categories", err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Category
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, wrap("iterate categories", rows.Err())
}

// upsertCategory writes one row, inserting new categories and refreshing
// built-ins in place. The conflict target is the id: built-ins are never
// deleted by Save, so they arrive as conflicts and keep their created_at.
func upsertCategory(ctx context.Context, tx *sql.Tx, c domain.Category, at int64) error {
	if c.ID == "" {
		id, err := newCategoryID()
		if err != nil {
			return err
		}
		c.ID = id
	}
	isSystem, isIdle := 0, 0
	if c.IsSystem {
		isSystem = 1
	}
	if c.IsIdle {
		isIdle = 1
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO categories (id, name, color_hex, details, sort_order, is_system, is_idle, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, color_hex = excluded.color_hex, details = excluded.details,
			sort_order = excluded.sort_order, is_system = excluded.is_system,
			is_idle = excluded.is_idle, updated_at = excluded.updated_at`,
		c.ID, c.Name, c.ColorHex, c.Details, c.SortOrder, isSystem, isIdle, at, at)
	if err != nil {
		return wrap("upsert category "+c.Name, err)
	}
	return nil
}

// newCategoryID generates a random UUIDv4-shaped identifier. Uniqueness is
// the only requirement; the format matches CategoryDTO's UUID contract.
func newCategoryID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", wrap("generate category id", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// scanner abstracts *sql.Row and *sql.Rows for shared column scanning.
type scanner interface {
	Scan(dest ...any) error
}

func scanCategory(row scanner) (domain.Category, error) {
	var c domain.Category
	var isSystem, isIdle int
	var details string
	if err := row.Scan(&c.ID, &c.Name, &c.ColorHex, &details, &c.SortOrder,
		&isSystem, &isIdle, &c.CreatedAtUnix, &c.UpdatedAtUnix); err != nil {
		return c, wrap("scan category", err)
	}
	c.Details = details
	c.IsSystem = isSystem == 1
	c.IsIdle = isIdle == 1
	return c, nil
}
