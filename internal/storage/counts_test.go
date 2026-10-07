package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCounterObserverCountsWithoutRetainingContent(t *testing.T) {
	var observer CounterObserver
	var workers sync.WaitGroup
	for i := 0; i < 20; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			observer.ObserveQuery("anonymous-private-operation", 250*time.Millisecond, errors.New("anonymous-private-error"))
			observer.ObserveQuery("fast", time.Millisecond, nil)
			observer.ObserveBusy("anonymous-private-operation")
			observer.ObserveBreadcrumb("storage.backup.failed")
			observer.ObserveBreadcrumb("storage.cleanup.ok")
			observer.ObserveBreadcrumb("anonymous-private-breadcrumb")
		}()
	}
	workers.Wait()
	got := observer.Snapshot()
	if got != (ObservationCounts{SlowQueries: 20, QueryErrors: 20, BusyErrors: 20, MaintenanceErrors: 20}) {
		t.Fatalf("counts = %+v", got)
	}
	payload, err := json.Marshal(got)
	if err != nil || strings.Contains(string(payload), "private") {
		t.Fatalf("snapshot leaked content: %s, %v", payload, err)
	}
}

func TestDefaultObserverAndMaintenanceShareCounters(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	before := store.ObservationCounts()
	if before == nil {
		t.Fatal("default observer is not measurable")
	}
	// A real SQLite failure must reach the same snapshot as maintenance failures.
	err = store.Write(ctx, "anonymous fixture", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO missing_anonymous_table VALUES (1)")
		return err
	})
	if err == nil {
		t.Fatal("missing-table write succeeded")
	}
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("anonymous"), 0o600); err != nil {
		t.Fatal(err)
	}
	NewMaintainer(store, MaintainerOptions{BackupDir: blocker}).runBackup(ctx)
	after := store.ObservationCounts()
	if after.QueryErrors <= before.QueryErrors || after.MaintenanceErrors != before.MaintenanceErrors+1 {
		t.Fatalf("before=%+v after=%+v", before, after)
	}
}

func TestUnmeasuredObserverIsNotReportedAsZero(t *testing.T) {
	store, err := Open(context.Background(), Options{Dir: t.TempDir(), Observer: NopObserver{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if store.ObservationCounts() != nil {
		t.Fatal("discarded signals reported as measured zero")
	}
}
