package storage

import (
	"sync/atomic"
	"time"
)

// ObservationCounts is a process-local snapshot. It contains no operation,
// error or breadcrumb text and is reset when the store's observer is recreated.
type ObservationCounts struct {
	SlowQueries       int64
	QueryErrors       int64
	BusyErrors        int64
	MaintenanceErrors int64
}

// CounterObserver records bounded, local diagnostics without blocking SQL or
// retaining content. The zero value is ready for concurrent use.
type CounterObserver struct {
	slowQueries       atomic.Int64
	queryErrors       atomic.Int64
	busyErrors        atomic.Int64
	maintenanceErrors atomic.Int64
}

func (o *CounterObserver) ObserveQuery(_ string, elapsed time.Duration, err error) {
	if elapsed >= slowQueryThreshold {
		o.slowQueries.Add(1)
	}
	if err != nil {
		o.queryErrors.Add(1)
	}
}

func (o *CounterObserver) ObserveBusy(string) { o.busyErrors.Add(1) }

func (o *CounterObserver) ObserveBreadcrumb(name string) {
	switch name {
	case "storage.checkpoint.failed", "storage.backup.failed", "storage.cleanup.failed":
		o.maintenanceErrors.Add(1)
	}
}

func (o *CounterObserver) Snapshot() ObservationCounts {
	return ObservationCounts{
		SlowQueries: o.slowQueries.Load(), QueryErrors: o.queryErrors.Load(),
		BusyErrors: o.busyErrors.Load(), MaintenanceErrors: o.maintenanceErrors.Load(),
	}
}

// ObservationCounts distinguishes measured zero from an observer that discards
// signals. Custom observers can implement Snapshot to expose the same counters.
func (s *Store) ObservationCounts() *ObservationCounts {
	if s == nil {
		return nil
	}
	observer, ok := s.observer.(interface{ Snapshot() ObservationCounts })
	if !ok {
		return nil
	}
	counts := observer.Snapshot()
	return &counts
}
