package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/Jwz-git/Daygo/internal/domain"
)

// ApplyPreset replaces the user category set with a first-run preset in one
// transaction. Each preset row is matched to an existing user category by
// name, ignoring case and outer spaces: a match keeps its id, stored name and
// idle flag and takes the preset's color, details and archetype; an unmatched
// row is inserted. Input ids and flags are ignored — a preset carries only
// name, color, details and archetype. Built-in names and duplicates within
// the preset are rejected.
//
// The other user categories are dropped unless something already references
// them (a card, live or retired, a plan block or a day goal); a referenced
// category is kept after the preset rather than deleted from under its
// history. The app layer offers this only before onboarding completes.
func (r *CategoryRepo) ApplyPreset(ctx context.Context, preset []domain.Category) error {
	if len(preset) == 0 {
		return newError(KindConstraint, "apply category preset: preset is empty")
	}
	rows := make([]domain.Category, len(preset))
	seen := make(map[string]bool, len(preset))
	for i, p := range preset {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return newError(KindConstraint, fmt.Sprintf("apply category preset: row %d has an empty name", i))
		}
		key := strings.ToLower(name)
		if seen[key] {
			return newError(KindConstraint, fmt.Sprintf("apply category preset: duplicate name %q", name))
		}
		seen[key] = true
		if !isHexColor(p.ColorHex) {
			return newError(KindConstraint, fmt.Sprintf("apply category preset: row %d has an invalid color %q", i, p.ColorHex))
		}
		archetype, ok := domain.NormalizeCategoryArchetype(p.Archetype)
		if !ok {
			return newError(KindConstraint, fmt.Sprintf("apply category preset: row %d has an unknown archetype %q", i, p.Archetype))
		}
		rows[i] = domain.Category{Name: name, ColorHex: p.ColorHex, Details: p.Details, Archetype: archetype}
	}

	at := r.store.now().Unix()
	return r.store.Write(ctx, "categories apply preset", func(ctx context.Context, tx *sql.Tx) error {
		existing, err := loadCategories(ctx, tx)
		if err != nil {
			return err
		}
		var user []domain.Category
		for _, e := range existing {
			if e.IsSystem {
				if seen[strings.ToLower(e.Name)] {
					return newError(KindConstraint, fmt.Sprintf("apply category preset: %q is a built-in category", e.Name))
				}
				continue
			}
			user = append(user, e)
		}
		sortCategories(user)

		matched := make(map[string]bool, len(rows))
		byKey := make(map[string]domain.Category, len(user))
		for _, u := range user {
			byKey[strings.ToLower(strings.TrimSpace(u.Name))] = u
		}
		resolved := make([]domain.Category, len(rows))
		for i, p := range rows {
			if u, ok := byKey[strings.ToLower(p.Name)]; ok && !matched[u.ID] {
				matched[u.ID] = true
				u.ColorHex, u.Details, u.Archetype = p.ColorHex, p.Details, p.Archetype
				resolved[i] = u
				continue
			}
			id, err := newCategoryID()
			if err != nil {
				return err
			}
			p.ID = id
			resolved[i] = p
		}

		var next []domain.Category
		for i, p := range resolved {
			p.SortOrder = i + 1
			next = append(next, p)
		}
		order := len(resolved)
		for _, u := range user {
			if matched[u.ID] {
				continue
			}
			referenced, err := categoryReferenced(ctx, tx, u)
			if err != nil {
				return err
			}
			if referenced {
				order++
				u.SortOrder = order
				next = append(next, u)
			}
		}

		ids := make(map[string]bool, len(next))
		for _, c := range next {
			ids[c.ID] = true
		}
		return saveCategoriesTx(ctx, tx, existing, next, ids, at)
	})
}

// categoryReferenced reports whether any card (live or retired), plan block
// or day goal still points at c. Such a category is history, not clutter.
func categoryReferenced(ctx context.Context, tx *sql.Tx, c domain.Category) (bool, error) {
	var found int
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM timeline_cards WHERE category = ?)
		    OR EXISTS (SELECT 1 FROM plan_blocks WHERE category_id = ?)
		    OR EXISTS (SELECT 1 FROM day_goal_categories WHERE category_id = ?)`,
		c.Name, c.ID, c.ID).Scan(&found)
	if err != nil {
		return false, wrap("check category references "+c.Name, err)
	}
	return found == 1, nil
}

// sortCategories orders rows as List does: sort_order, then name.
func sortCategories(cats []domain.Category) {
	sort.SliceStable(cats, func(i, j int) bool {
		if cats[i].SortOrder != cats[j].SortOrder {
			return cats[i].SortOrder < cats[j].SortOrder
		}
		return cats[i].Name < cats[j].Name
	})
}

// isHexColor accepts the "#RRGGBB" form the categories table stores.
func isHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, ch := range value[1:] {
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F') {
			return false
		}
	}
	return true
}
