package favicon

import (
	"bytes"
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
