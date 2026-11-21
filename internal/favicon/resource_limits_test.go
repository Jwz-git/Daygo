package favicon

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type waitingTransport struct {
	active atomic.Int32
	peak   atomic.Int32
	enter  chan struct{}
}

func (r *waitingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	n := r.active.Add(1)
	defer r.active.Add(-1)
	for peak := r.peak.Load(); n > peak && !r.peak.CompareAndSwap(peak, n); peak = r.peak.Load() {
	}
	r.enter <- struct{}{}
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestFaviconHTTPConcurrencyAcrossHosts(t *testing.T) {
	r := New("")
	transport := &waitingTransport{enter: make(chan struct{}, 128)}
	r.client.Transport = transport
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var workers sync.WaitGroup
	for i := range 24 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, _ = r.Resolve(ctx, fmt.Sprintf("anonymous-%d.example", i))
		}()
	}
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for range maxConcurrentRequests {
		select {
		case <-transport.enter:
		case <-deadline.C:
			t.Error("requests did not fill the available slots")
			cancel()
			workers.Wait()
			return
		}
	}
	select {
	case <-transport.enter:
		t.Error("another request started with all slots occupied")
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	workers.Wait()
	if got := transport.peak.Load(); got != maxConcurrentRequests {
		t.Fatalf("requests peaked at %d, want %d", got, maxConcurrentRequests)
	}
}

func TestPruneDiskKeepsOnlyRecentCacheEntries(t *testing.T) {
	r := New(t.TempDir())
	old, recent := r.diskPath("old.example"), r.diskPath("recent.example")
	unrelated := filepath.Join(r.cacheDir, "keep.ico")
	for _, name := range []string{old, recent, unrelated} {
		if err := os.WriteFile(name, pngPixel, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	then := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, then, then); err != nil {
		t.Fatal(err)
	}
	pruneDiskIcons(r.cacheDir, 100, int64(len(pngPixel)))
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old cached icon exists or failed to stat: %v", err)
	}
	for _, name := range []string{recent, unrelated} {
		if _, err := os.Stat(name); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiskFaviconBudgetPrunesDuringWrites(t *testing.T) {
	r := New(t.TempDir())
	r.disk.initialize(r.cacheDir)
	old := r.diskPath("first.example")
	if err := os.WriteFile(old, pngPixel, 0o600); err != nil {
		t.Fatal(err)
	}
	then := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, then, then); err != nil {
		t.Fatal(err)
	}
	r.disk.bytes = diskIconByteLimit
	r.writeDisk("second.example", Result{Data: pngPixel})
	if want := int64(2 * len(pngPixel)); r.disk.bytes != want {
		t.Fatalf("write did not recount the disk cache, got %d bytes, want %d", r.disk.bytes, want)
	}
	if _, exists := r.readDisk("second.example"); !exists {
		t.Fatal("new icon was not retained")
	}
}
