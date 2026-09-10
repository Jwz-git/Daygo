// Package storage owns every SQL statement and every connection to the Daygo
// business database. It is the single schema owner and the single writer; no
// other package opens SQLite or holds a *sql.DB. See docs/05 §5.6.2.
//
// The rules this package exists to enforce:
//
//   - Go Core stays buildable and testable with CGO_ENABLED=0 on Linux, so the
//     driver is pure Go (modernc.org/sqlite). Nothing here may import Wails, a
//     macOS framework, or an adapter implementation.
//   - The platform adapter never opens or writes SQLite. It reports frames and
//     closed segments; Go decides what is persisted (docs/06 §6.3).
//   - A second instance that cannot take the write lock downgrades to a
//     read-only connection. Read-only is enforced at the connection layer with
//     SQLITE_OPEN_READONLY plus PRAGMA query_only, never by caller discipline
//     (docs/07 §7.5).
//   - Schema evolves through a versioned migration chain starting at
//     user_version 1. Every new version needs an "old database to new database"
//     fixture test (docs/05 §5.6.2 rule 3).
//   - There is no global database singleton. The *Store returned by Open is the
//     single instance, injected by the app layer.
//
// The pragma set is fixed at journal_mode=WAL, synchronous=NORMAL and
// busy_timeout=5000 (docs/03 §3.3). Error classification is centralized here so
// the binding layer can map storage errors onto the closed application code set
// in one place (docs/05 §5.6.1).
package storage
