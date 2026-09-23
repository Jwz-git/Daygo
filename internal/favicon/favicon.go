// Package favicon resolves a site's favicon from its hostname, on the Go side
// rather than inside the webview. Fetching here uses the process network stack
// (http.DefaultTransport honours HTTPS_PROXY), which is what lets the icon load
// in environments where the webview cannot reach Google directly.
//
// The design mirrors Dayflow's FaviconService: race Google's S2 aggregator
// against the site's own /favicon.ico, first decodable image wins. Results are
// cached on disk so a host is fetched once per machine, not once per render.
//
// The host string originates from LLM output (a card's appSites) and is treated
// as untrusted: it is validated as a plausible public hostname, and the dialer
// refuses to connect to private, loopback or link-local addresses (SSRF guard).
package favicon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	// fetchTimeout bounds one favicon lookup end to end. Icons are decorative;
	// a slow host must never hold a render.
	fetchTimeout = 4 * time.Second
	// directHeadStart delays the direct /favicon.ico attempt so the aggregators
	// (which return normalized icons and resolve HTML-declared favicons the
	// root path misses) get a lead, matching Dayflow's S2 head-start. First
	// decodable image wins regardless.
	directHeadStart = 150 * time.Millisecond
	// maxIconBytes caps a favicon response. Real icons are a few KiB; the cap
	// stops a hostile or broken host from ballooning memory or disk.
	maxIconBytes = 512 << 10
	// negativeTTL suppresses re-fetching a host that just failed, so a dead
	// host is not hammered on every card render.
	negativeTTL = 10 * time.Minute
	// memoryCacheCap bounds the in-process hot cache of resolved icons. The
	// disk cache is the source of truth, so evicting an entry only costs a
	// later disk read — this cap is what stops a resident agent from retaining
	// one icon (up to maxIconBytes) per distinct host seen over its whole
	// uptime, which is the leak this package used to have.
	memoryCacheCap = 512
	// negativeCacheCap bounds the negative cache the same way. Its entries are
	// only timestamps, but an always-on agent would still accumulate one per
	// host that ever failed without a cap.
	negativeCacheCap = 512
)

// Result is a resolved favicon: raw image bytes and the sniffed content type.
type Result struct {
	Data        []byte
	ContentType string
}

// Resolver fetches and caches favicons. It is safe for concurrent use.
type Resolver struct {
	client   *http.Client
	cacheDir string
	group    singleflight.Group

	mu       sync.Mutex
	memory   *lruCache[Result]    // host -> cached icon (bounded session hot cache)
	negative *lruCache[time.Time] // host -> earliest retry time (bounded)
}

// New builds a Resolver caching to dir (created on first successful write).
// The HTTP client uses an SSRF-guarded dialer and resolves a proxy from the
// environment first, then the macOS system proxy — the latter is what lets
// Google S2 work here the way Dayflow's native URLSession does, since Go does
// not read the system proxy on its own.
func New(cacheDir string) *Resolver {
	transport := &http.Transport{
		Proxy: proxyResolver(),
		DialContext: (&guardedDialer{
			base: &net.Dialer{Timeout: fetchTimeout, KeepAlive: 30 * time.Second},
		}).DialContext,
		TLSHandshakeTimeout:   fetchTimeout,
		ResponseHeaderTimeout: fetchTimeout,
		MaxIdleConns:          8,
		IdleConnTimeout:       90 * time.Second,
	}
	return &Resolver{
		client:   &http.Client{Transport: transport, Timeout: fetchTimeout},
		cacheDir: cacheDir,
		memory:   newLRUCache[Result](memoryCacheCap),
		negative: newLRUCache[time.Time](negativeCacheCap),
	}
}

// proxyResolver returns a Transport proxy function that prefers an
// environment proxy (HTTPS_PROXY/HTTP_PROXY) and falls back to the platform
// system proxy resolved once at startup. This mirrors Dayflow, whose URLSession
// honours the macOS system proxy transparently.
func proxyResolver() func(*http.Request) (*url.URL, error) {
	sys := systemProxyURL()
	return func(req *http.Request) (*url.URL, error) {
		if u, err := http.ProxyFromEnvironment(req); err == nil && u != nil {
			return u, nil
		}
		return sys, nil
	}
}

// Resolve returns the favicon for host, or an error when nothing decodable is
// found. Concurrent calls for one host share a single fetch. Successful results
// are cached in memory and on disk; failures are negatively cached in memory.
func (r *Resolver) Resolve(ctx context.Context, host string) (Result, error) {
	normalized, err := NormalizeHost(host)
	if err != nil {
		return Result{}, err
	}
	normalized = resolveAlias(normalized)

	if hit, ok := r.lookup(normalized); ok {
		return hit, nil
	}

	value, err, _ := r.group.Do(normalized, func() (any, error) {
		// Re-check after acquiring the flight: a racing caller may have filled
		// the caches while this goroutine waited.
		if hit, ok := r.lookup(normalized); ok {
			return hit, nil
		}
		if disk, ok := r.readDisk(normalized); ok {
			r.storeMemory(normalized, disk)
			return disk, nil
		}
		result, ferr := r.fetch(ctx, normalized)
		if ferr != nil {
			r.storeNegative(normalized)
			return Result{}, ferr
		}
		r.storeMemory(normalized, result)
		r.writeDisk(normalized, result)
		return result, nil
	})
	if err != nil {
		return Result{}, err
	}
	return value.(Result), nil
}

// lookup consults the in-memory hot cache and negative cache. The bool reports
// a hot-cache hit; a live negative entry returns a sentinel error via ok=false
// path handled by the caller's fetch (it will be re-negatived cheaply).
func (r *Resolver) lookup(host string) (Result, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if hit, ok := r.memory.get(host); ok {
		return hit, true
	}
	if until, ok := r.negative.get(host); ok {
		if time.Now().Before(until) {
			// Represented as a hot "empty" result so callers skip the network
			// without a disk hit; Resolve treats empty data as not-found below.
			return Result{}, false
		}
		// Expired: drop it so the cache does not retain dead hosts forever.
		r.negative.delete(host)
	}
	return Result{}, false
}

func (r *Resolver) storeMemory(host string, result Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memory.put(host, result)
	r.negative.delete(host)
}

func (r *Resolver) storeNegative(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.negative.put(host, time.Now().Add(negativeTTL))
}

// negativeActive reports whether host is inside its negative-cache window.
func (r *Resolver) negativeActive(host string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	until, ok := r.negative.get(host)
	if !ok {
		return false
	}
	if time.Now().Before(until) {
		return true
	}
	r.negative.delete(host)
	return false
}

// fetch races several favicon sources; the first decodable image wins and the
// losers are cancelled. Following Dayflow, the aggregators (Google S2 +
// icon.horse) fire immediately because they return normalized icons and
// resolve HTML-declared favicons that the root /favicon.ico misses; the direct
// site hit starts a short head-start later as the aggregator-free fallback.
func (r *Resolver) fetch(ctx context.Context, host string) (Result, error) {
	if r.negativeActive(host) {
		return Result{}, errNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	type attempt struct {
		result Result
		err    error
	}
	sources := []struct {
		url   string
		delay time.Duration
	}{
		{s2URL(host), 0},
		{aggregatorURL(host), 0},
		{directURL(host), directHeadStart},
	}
	results := make(chan attempt, len(sources))
	for _, src := range sources {
		go func(url string, delay time.Duration) {
			if delay > 0 {
				select {
				case <-ctx.Done():
					results <- attempt{Result{}, ctx.Err()}
					return
				case <-time.After(delay):
				}
			}
			res, err := r.request(ctx, url)
			results <- attempt{res, err}
		}(src.url, src.delay)
	}

	var lastErr error
	for range sources {
		got := <-results
		if got.err == nil && len(got.result.Data) > 0 {
			return got.result, nil
		}
		if got.err != nil {
			lastErr = got.err
		}
	}
	if lastErr == nil {
		lastErr = errNotFound
	}
	return Result{}, lastErr
}

// request fetches one URL and validates the payload is a non-empty image.
func (r *Resolver) request(ctx context.Context, target string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "image/*")
	// No cookies, no referrer: only the hostname may leave the device.
	req.Header.Set("Referer", "")

	resp, err := r.client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("favicon: status %d", resp.StatusCode)
	}
	data, err := readCapped(resp.Body, maxIconBytes)
	if err != nil {
		return Result{}, err
	}
	if len(data) == 0 {
		return Result{}, errNotFound
	}
	ct := http.DetectContentType(data)
	if !strings.HasPrefix(ct, "image/") {
		return Result{}, fmt.Errorf("favicon: not an image (%s)", ct)
	}
	return Result{Data: data, ContentType: ct}, nil
}

// --- disk cache ---

func (r *Resolver) diskPath(host string) string {
	sum := sha256.Sum256([]byte(host))
	return filepath.Join(r.cacheDir, hex.EncodeToString(sum[:])+".ico")
}

func (r *Resolver) readDisk(host string) (Result, bool) {
	if r.cacheDir == "" {
		return Result{}, false
	}
	data, err := os.ReadFile(r.diskPath(host))
	if err != nil || len(data) == 0 {
		return Result{}, false
	}
	ct := http.DetectContentType(data)
	if !strings.HasPrefix(ct, "image/") {
		return Result{}, false
	}
	return Result{Data: data, ContentType: ct}, true
}

func (r *Resolver) writeDisk(host string, result Result) {
	if r.cacheDir == "" || len(result.Data) == 0 {
		return
	}
	if err := os.MkdirAll(r.cacheDir, 0o755); err != nil {
		return
	}
	path := r.diskPath(host)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, result.Data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// --- helpers ---

var errNotFound = errors.New("favicon: not found")

// hostAliases redirects known renamed/sub-brand hosts to the domain that
// actually serves the icon, matching Dayflow's FaviconService.hostAliases.
var hostAliases = map[string]string{
	"codex.com": "chatgpt.com",
	"codex.so":  "chatgpt.com",
}

func resolveAlias(host string) string {
	if aliased, ok := hostAliases[host]; ok {
		return aliased
	}
	return host
}

func directURL(host string) string {
	return "https://" + host + "/favicon.ico"
}

// s2URL is Google's favicon aggregator (Dayflow's default). It resolves the
// site's HTML-declared favicon server-side and returns a normalized 64px icon.
// Only the hostname is sent. Reachable via the proxy resolved in New.
func s2URL(host string) string {
	return "https://www.google.com/s2/favicons?sz=64&domain=" + url.QueryEscape(host)
}

// aggregatorURL is a second aggregator that stays reachable when Google is
// blocked and no proxy is configured (docs/decisions/timeline-favicon-fetch.md);
// only the hostname is sent.
func aggregatorURL(host string) string {
	return "https://icon.horse/icon/" + url.PathEscape(host)
}
