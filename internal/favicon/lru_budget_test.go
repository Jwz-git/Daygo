package favicon

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

func TestWeightedLRUAccountingAndOversizedReplacement(t *testing.T) {
	c := newWeightedLRUCache[string](10, 8, func(s string) int { return len(s) })
	c.put("a", "1234")
	c.put("b", "1234")
	c.get("a")
	c.put("c", "12")
	if _, ok := c.get("b"); ok {
		t.Fatal("byte overflow did not evict LRU")
	}
	if c.weight() != 6 {
		t.Fatalf("weight=%d want 6", c.weight())
	}
	c.put("a", "1")
	if c.weight() != 3 {
		t.Fatalf("replacement weight=%d want 3", c.weight())
	}
	c.delete("c")
	c.delete("missing")
	if c.weight() != 1 {
		t.Fatalf("delete weight=%d want 1", c.weight())
	}
	c.put("a", "123456789")
	if c.len() != 0 || c.weight() != 0 {
		t.Fatal("oversized replacement retained old/new value")
	}
	c.put("empty", "")
	if _, ok := c.get("empty"); !ok {
		t.Fatal("empty value was treated as a miss")
	}
}

func TestFaviconByteBudgetAndEvictedDiskResolution(t *testing.T) {
	r := New(t.TempDir())
	// Anonymous image payloads exercise the budget before the entry cap.
	data := append(bytes.Clone(pngPixel), make([]byte, 256<<10)...)
	for i := 0; i < memoryCacheCap+50; i++ {
		host := fmt.Sprintf("anonymous%d.example", i)
		result := Result{Data: bytes.Clone(data), ContentType: "image/png"}
		r.writeDisk(host, result)
		r.storeMemory(host, result)
		if r.memory.weight() > memoryCacheBytes || r.memory.len() > memoryCacheCap {
			t.Fatal("cache budget exceeded")
		}
	}
	if _, ok := r.lookup("anonymous0.example"); ok {
		t.Fatal("oldest icon not evicted")
	}
	got, err := r.Resolve(context.Background(), "anonymous0.example")
	if err != nil || !bytes.Equal(got.Data, data) {
		t.Fatalf("evicted disk icon could not resolve: %v", err)
	}
	oversized := Result{Data: make([]byte, memoryCacheBytes+1), ContentType: "image/png"}
	r.storeMemory("oversized.example", oversized)
	if _, ok := r.lookup("oversized.example"); ok {
		t.Fatal("oversized result cached")
	}
}

type anonymousIconTransport struct{ calls atomic.Int32 }

func (r *anonymousIconTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.calls.Add(1)
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(pngPixel)), Request: req}, nil
}
func TestBudgetResolverConcurrentDeduplication(t *testing.T) {
	r := New("")
	transport := &anonymousIconTransport{}
	r.client.Transport = transport
	var workers sync.WaitGroup
	errors := make(chan error, 24)
	for i := 0; i < 24; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			res, err := r.Resolve(context.Background(), "anonymous-concurrent.example")
			if err != nil {
				errors <- err
			} else if !bytes.Equal(res.Data, pngPixel) {
				errors <- fmt.Errorf("unexpected icon")
			}
		}()
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if calls := transport.calls.Load(); calls > 3 {
		t.Fatalf("concurrent callers launched %d requests, want at most one three-source race", calls)
	}
}
