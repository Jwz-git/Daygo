package storage

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"
)

// recordingObserver captures signals so a test can assert they were emitted.
type recordingObserver struct {
	mu          sync.Mutex
	queries     []string
	busy        []string
	breadcrumbs []string
}

func (o *recordingObserver) ObserveQuery(op string, _ time.Duration, _ error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.queries = append(o.queries, op)
}

func (o *recordingObserver) ObserveBusy(op string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.busy = append(o.busy, op)
}

func (o *recordingObserver) ObserveBreadcrumb(name string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.breadcrumbs = append(o.breadcrumbs, name)
}

func (o *recordingObserver) hasBreadcrumb(name string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, b := range o.breadcrumbs {
		if b == name {
			return true
		}
	}
	return false
}

func (o *recordingObserver) queryCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.queries)
}

// Observability is attached to the store, not to repository signatures
// (docs/05 §5.6.2 rule 6). A statement must be reported without the caller
// passing anything.
func TestObserverReceivesStatements(t *testing.T) {
	dir := newDir(t)
	observer := &recordingObserver{}

	store, err := Open(context.Background(), Options{Dir: dir, Observer: observer})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Write(context.Background(), "insert setting", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			"INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)",
			"appearance.theme", `"dark"`, 0)
		return err
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	if observer.queryCount() == 0 {
		t.Fatal("observer received no statements")
	}
}

// A degraded open is a diagnostic event: the user's second instance is running
// read-only, which explains why its writes are refused.
func TestDegradedOpenEmitsBreadcrumb(t *testing.T) {
	dir := newDir(t)
	openWriter(t, dir)

	observer := &recordingObserver{}
	second, err := Open(context.Background(), Options{Dir: dir, Observer: observer})
	if err != nil {
		t.Fatalf("Open second: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	if second.Mode() != ModeReadOnly {
		t.Fatalf("Mode() = %q, want %q", second.Mode(), ModeReadOnly)
	}
	if !observer.hasBreadcrumb(breadcrumbDegradedOpen) {
		t.Fatalf("degraded open did not emit %q; got %v", breadcrumbDegradedOpen, observer.breadcrumbs)
	}
}

// A nil observer must be replaced by the no-op implementation rather than
// leaving a nil to be dereferenced on the first statement.
func TestNilObserverIsSafe(t *testing.T) {
	store, err := Open(context.Background(), Options{Dir: newDir(t)})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Write(context.Background(), "noop", func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			"INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)", "k", `1`, 0)
		return err
	}); err != nil {
		t.Fatalf("write with nil observer: %v", err)
	}
}

func TestNopObserverAcceptsEverything(t *testing.T) {
	var n NopObserver
	n.ObserveQuery("op", time.Millisecond, nil)
	n.ObserveBusy("op")
	n.ObserveBreadcrumb("name")
}
