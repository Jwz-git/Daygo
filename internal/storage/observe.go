package storage

import "time"

// Observer receives slow-query, contention and breadcrumb signals from the
// read/write wrappers. It exists so those signals are recorded where the SQL
// runs, while staying out of repository signatures (docs/05 §5.6.2 rule 6).
//
// Implementations must be safe for concurrent use and must not block: a slow
// observer would distort the measurement it exists to report.
//
// The statement text passed here is compile-time constant SQL. Bound arguments
// are never included, because they can carry user content (docs/07).
type Observer interface {
	// ObserveQuery reports a completed statement.
	ObserveQuery(op string, d time.Duration, err error)
	// ObserveBusy reports a lock-contention failure, which is the signal DB-8
	// asserts stays free of storms.
	ObserveBusy(op string)
	// ObserveBreadcrumb reports a low-cardinality diagnostic marker, for
	// example a retry or a degraded open.
	ObserveBreadcrumb(name string)
}

// NopObserver discards every signal. It is the default when Options.Observer is
// nil, so callers that do not care about diagnostics pay nothing.
type NopObserver struct{}

func (NopObserver) ObserveQuery(string, time.Duration, error) {}
func (NopObserver) ObserveBusy(string)                        {}
func (NopObserver) ObserveBreadcrumb(string)                  {}

// Statement timeout ceilings from docs/05 §5.6.1. Callers may pass a shorter
// deadline through ctx; these are the maxima the wrapper enforces when it has
// to originate one itself.
const (
	readTimeout  = 5 * time.Second
	writeTimeout = 10 * time.Second
)

// slowQueryThreshold marks a statement slow enough to report. It is well below
// the timeouts above so a reported slow query is visible before it becomes a
// failure.
const slowQueryThreshold = 250 * time.Millisecond

// breadcrumb names emitted by this package.
const (
	breadcrumbLockBusy     = "storage.lock.busy"
	breadcrumbDegradedOpen = "storage.open.degraded_readonly"
	breadcrumbMigrated     = "storage.migrate.applied"
	// breadcrumbRecovered marks a database replaced from backup after
	// corruption. It is the only signal that the user is looking at older data
	// than they had, so it must not be silent.
	breadcrumbRecovered = "storage.open.recovered_from_backup"
)
