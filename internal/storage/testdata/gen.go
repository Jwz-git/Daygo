//go:build ignore

// Command gen writes the anonymous binary fixtures used by the storage tests.
//
// Run from the repository root:
//
//	go run ./internal/storage/testdata/gen.go
//
// The generator is committed so the fixtures are reproducible; the .db files it
// writes are committed too, because DB-2 needs a database written by a previous
// version and that cannot be recreated by this build's own DDL.
//
// Every value here is invented. No fixture may contain real user data, and none
// does: docs/modules/data.md requires isolated, anonymous fixtures.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	outDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	// Allow running from the repository root or from this directory.
	if _, err := os.Stat(filepath.Join(outDir, "gen.go")); err != nil {
		outDir = filepath.Join(outDir, "internal", "storage", "testdata")
	}

	if err := writeV0WithSettings(filepath.Join(outDir, "v0-with-settings.db")); err != nil {
		log.Fatalf("v0-with-settings.db: %v", err)
	}
	if err := writeTruncated(filepath.Join(outDir, "truncated.db")); err != nil {
		log.Fatalf("truncated.db: %v", err)
	}
	if err := writeNotADatabase(filepath.Join(outDir, "notadb.db")); err != nil {
		log.Fatalf("notadb.db: %v", err)
	}
	fmt.Println("fixtures written to", outDir)
}

// writeV0WithSettings builds a version-0 database holding a table this build
// does not create, so a migration test can prove the upgrade does not disturb
// pre-existing data.
func writeV0WithSettings(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE legacy_marker (
			id   INTEGER PRIMARY KEY,
			note TEXT NOT NULL
		)`,
		`INSERT INTO legacy_marker (id, note) VALUES
			(1, 'anonymous-fixture-alpha'),
			(2, 'anonymous-fixture-beta')`,
		// user_version stays 0: this database predates the migration chain.
		`PRAGMA user_version = 0`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// writeTruncated builds a database that starts life valid and is then cut short,
// which is how real torn writes present themselves.
func writeTruncated(path string) error {
	if err := writeV0WithSettings(path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	// Keep the header so the file is still recognizably a database, but drop
	// enough that the page tree cannot be read.
	if err := os.Truncate(path, info.Size()/3); err != nil {
		return err
	}
	return nil
}

// writeNotADatabase writes a file with no SQLite header at all.
func writeNotADatabase(path string) error {
	return os.WriteFile(path, []byte("this is not a sqlite database\n"), 0o600)
}
