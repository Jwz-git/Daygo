package favicon

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// a 1x1 PNG, enough for http.DetectContentType to report image/png.
var pngPixel = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89,
}

func TestReadCapped(t *testing.T) {
	data, err := readCapped(bytes.NewReader([]byte("abc")), 10)
	if err != nil || string(data) != "abc" {
		t.Fatalf("readCapped small: %q err=%v", data, err)
	}
	if _, err := readCapped(bytes.NewReader(bytes.Repeat([]byte("x"), 20)), 10); err == nil {
		t.Fatal("readCapped over limit: expected error")
	}
}

func TestDiskCacheRoundTrip(t *testing.T) {
	r := New(t.TempDir())
	host := "example.com"
	want := Result{Data: pngPixel, ContentType: "image/png"}

	if _, ok := r.readDisk(host); ok {
		t.Fatal("readDisk before write: expected miss")
	}
	r.writeDisk(host, want)
	got, ok := r.readDisk(host)
	if !ok {
		t.Fatal("readDisk after write: expected hit")
	}
	if !bytes.Equal(got.Data, want.Data) {
		t.Fatalf("readDisk data mismatch")
	}
	if !strings.HasPrefix(got.ContentType, "image/") {
		t.Fatalf("readDisk content type = %q, want image/*", got.ContentType)
	}
}

func TestMemoryCacheHit(t *testing.T) {
	r := New(t.TempDir())
	host := "cached.com"
	r.storeMemory(host, Result{Data: pngPixel, ContentType: "image/png"})
	got, ok := r.lookup(host)
	if !ok || len(got.Data) == 0 {
		t.Fatalf("lookup after storeMemory: ok=%v len=%d", ok, len(got.Data))
	}
}

func TestResolveAlias(t *testing.T) {
	if got := resolveAlias("codex.com"); got != "chatgpt.com" {
		t.Fatalf("resolveAlias(codex.com) = %q, want chatgpt.com", got)
	}
	if got := resolveAlias("example.com"); got != "example.com" {
		t.Fatalf("resolveAlias(example.com) = %q, want unchanged", got)
	}
}

func TestNegativeCacheWindow(t *testing.T) {
	r := New(t.TempDir())
	host := "dead.example"
	if r.negativeActive(host) {
		t.Fatal("fresh host should not be negative")
	}
	r.storeNegative(host)
	if !r.negativeActive(host) {
		t.Fatal("host should be negative after storeNegative")
	}
}

// TestMemoryCacheBounded is the regression guard for the leak this package had:
// the in-memory hot cache must not grow without bound as distinct hosts arrive.
func TestMemoryCacheBounded(t *testing.T) {
	r := New(t.TempDir())
	for i := range memoryCacheCap + 50 {
		r.storeMemory(fmt.Sprintf("host%d.example", i), Result{Data: pngPixel, ContentType: "image/png"})
	}
	if n := r.memory.len(); n > memoryCacheCap {
		t.Fatalf("memory cache len = %d, want <= %d", n, memoryCacheCap)
	}
	if _, ok := r.lookup("host0.example"); ok {
		t.Fatal("oldest entry should have been evicted once over capacity")
	}
	newest := fmt.Sprintf("host%d.example", memoryCacheCap+49)
	if _, ok := r.lookup(newest); !ok {
		t.Fatal("newest entry should still be cached")
	}
}

// TestMemoryCacheKeepsRecentlyUsed proves eviction is LRU, not FIFO: a host
// touched right before the cache overflows must survive.
func TestMemoryCacheKeepsRecentlyUsed(t *testing.T) {
	r := New(t.TempDir())
	for i := range memoryCacheCap {
		r.storeMemory(fmt.Sprintf("host%d.example", i), Result{Data: pngPixel, ContentType: "image/png"})
	}
	if _, ok := r.lookup("host0.example"); !ok { // touch the oldest, promoting it
		t.Fatal("host0 should be present before overflow")
	}
	r.storeMemory("overflow.example", Result{Data: pngPixel, ContentType: "image/png"})
	if _, ok := r.lookup("host0.example"); !ok {
		t.Fatal("recently-used host0 must survive; host1 should have been evicted")
	}
	if _, ok := r.lookup("host1.example"); ok {
		t.Fatal("least-recently-used host1 should have been evicted")
	}
}

func TestNegativeCacheBounded(t *testing.T) {
	r := New(t.TempDir())
	for i := range negativeCacheCap + 50 {
		r.storeNegative(fmt.Sprintf("dead%d.example", i))
	}
	if n := r.negative.len(); n > negativeCacheCap {
		t.Fatalf("negative cache len = %d, want <= %d", n, negativeCacheCap)
	}
}
