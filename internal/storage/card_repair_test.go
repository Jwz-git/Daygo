package storage

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateV27IndexRepairsOnlyCorruptCards(t *testing.T) {
	dir := newDir(t)
	copyFile(t, filepath.Join("testdata", "v27-card-repair-source.db"), filepath.Join(dir, DatabaseFileName))
	store := openWriter(t, dir)
	if got := userVersionOf(t, store); got != schemaVersion() {
		t.Fatalf("user_version = %d, want %d", got, schemaVersion())
	}
	for _, fixture := range []struct {
		id      int
		deleted bool
	}{
		{2001, false}, {2002, true}, {2003, true},
	} {
		var deleted bool
		if err := store.db.QueryRow(`SELECT is_deleted FROM timeline_cards WHERE id = ?`, fixture.id).Scan(&deleted); err != nil {
			t.Fatal(err)
		}
		if deleted != fixture.deleted {
			t.Fatalf("card %d deleted = %v, want %v", fixture.id, deleted, fixture.deleted)
		}
	}
	rows, err := store.db.Query(`EXPLAIN QUERY PLAN `+sanitizeCorruptCardsSQL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(plan, " "), "idx_timeline_cards_invalid_live") {
		t.Fatalf("startup repair scans outside the invalid-card index: %q", plan)
	}
}
