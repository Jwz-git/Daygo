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
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
	"golang.org/x/sync/singleflight"
)

const (
	// fetchTimeout bounds one favicon lookup end to end. Icons are decorative;
	// a slow host must never hold a render.
	fetchTimeout = 4 * time.Second
	// raceSettleWindow gives the origin /favicon.ico and higher-fidelity sources
	// a brief window to finish when an aggregator replies first, so official
	// origin icons win over lower-confidence or redirected aggregator images.
	raceSettleWindow = 350 * time.Millisecond
	// maxIconBytes caps a favicon response. Real icons are a few KiB; the cap
	// stops a hostile or broken host from ballooning memory or disk.
	maxIconBytes = 512 << 10
	// maxHTMLBytes bounds the HTML head snippet read during fallback link discovery.
	maxHTMLBytes = 64 << 10
	// diskCacheTTL bounds how long a cached icon stays fresh on disk before
	// re-validation, preventing stale or degraded icons from sticking forever.
	diskCacheTTL = 14 * 24 * time.Hour
	// diskCacheVersion namespaces cache keys so quality-gate upgrades bypass
	// pre-existing low-quality or fallback icons cached by earlier builds.
	diskCacheVersion = "v2:"
	// Each hostname can race multiple sources. Bound the actual HTTP requests
	// across all hosts, including slower losers while cancellation settles.
	maxConcurrentRequests = 12
	// negativeTTL suppresses re-fetching a host that just failed, so a dead
	// host is not hammered on every card render.
	negativeTTL = 10 * time.Minute
	// memoryCacheCap bounds the in-process hot cache of resolved icons. The
	// disk cache is the source of truth, so evicting an entry only costs a
	// later disk read — this cap is what stops a resident agent from retaining
	// one icon (up to maxIconBytes) per distinct host seen over its whole
	// uptime, which is the leak this package used to have.
	memoryCacheCap   = 512
	memoryCacheBytes = 8 << 20
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
	client       *http.Client
	cacheDir     string
	settleWindow time.Duration
	group        singleflight.Group
	requests     *semaphore.Weighted
	disk         diskIconCache

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
		requests: semaphore.NewWeighted(maxConcurrentRequests),
		memory:   newWeightedLRUCache[Result](memoryCacheCap, memoryCacheBytes, func(result Result) int { return len(result.Data) }),
		negative: newLRUCache[time.Time](negativeCacheCap),
	}
}

// proxyResolver returns a Transport proxy function that prefers an
// environment proxy (HTTPS_PROXY/HTTP_PROXY) and falls back to the platform
// system proxy resolved once on the first fallback. This mirrors Dayflow, whose URLSession
// honours the macOS system proxy transparently.
func proxyResolver() func(*http.Request) (*url.URL, error) {
	systemProxy := sync.OnceValue(systemProxyURL)
	return func(req *http.Request) (*url.URL, error) {
		if u, err := http.ProxyFromEnvironment(req); err == nil && u != nil {
			return u, nil
		}
		return systemProxy(), nil
	}
}

// Resolve returns the favicon for host, or an error when nothing decodable is
// found. Concurrent calls for one host share a single fetch. Successful results
// are cached in memory and on disk; failures are negatively cached in memory.
func (r *Resolver) Resolve(ctx context.Context, host string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	normalized, err := NormalizeHost(host)
	if err != nil {
		return Result{}, err
	}
	normalized = resolveAlias(normalized)

	if hit, ok := r.lookup(normalized); ok {
		return hit, nil
	}
	if r.negativeActive(normalized) {
		return Result{}, errNotFound
	}

	value, err, _ := r.group.Do(normalized, func() (any, error) {
		// Re-check after acquiring the flight: a racing caller may have filled
		// the caches while this goroutine waited.
		if hit, ok := r.lookup(normalized); ok {
			return hit, nil
		}
		if r.negativeActive(normalized) {
			return Result{}, errNotFound
		}
		if disk, ok := r.readDisk(normalized); ok {
			r.storeMemory(normalized, disk)
			return disk, nil
		}
		result, ferr := r.fetch(ctx, normalized)
		if ferr != nil {
			if ctx.Err() == nil && !errors.Is(ferr, context.Canceled) && !errors.Is(ferr, errFetchQueued) {
				r.storeNegative(normalized)
			}
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

// lookup consults the in-memory cache of successful results.
func (r *Resolver) lookup(host string) (Result, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.memory.get(host)
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

func (r *Resolver) raceSettleWindow() time.Duration {
	if r.settleWindow > 0 {
		return r.settleWindow
	}
	return raceSettleWindow
}

// fetch resolves the highest-quality official favicon for host. It starts the
// site's own /favicon.ico alongside Google S2 and icon.horse with no handicap,
// rejects synthetic fallback / login-redirect placeholders, and prefers crisp
// origin icons over aggregator responses. When all three fail, it falls back
// to discovering raster <link rel="icon"> tags in the homepage HTML head and
// finally to the parent registrable domain for non-multi-tenant subdomains.
func (r *Resolver) fetch(ctx context.Context, host string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	res, queued, err := r.fetchPrimary(ctx, host)
	if err == nil && len(res.Data) > 0 {
		return res, nil
	}
	if ctx.Err() != nil || queued {
		if queued {
			return Result{}, errFetchQueued
		}
		return Result{}, err
	}

	if htmlRes, htmlErr := r.fetchHTMLIcon(ctx, host); htmlErr == nil && len(htmlRes.Data) > 0 {
		return htmlRes, nil
	}
	if ctx.Err() != nil {
		return Result{}, err
	}

	if parent, ok := parentDomainFallback(host); ok {
		if parentRes, _, parentErr := r.fetchPrimary(ctx, parent); parentErr == nil && len(parentRes.Data) > 0 {
			return parentRes, nil
		}
	}
	if err == nil {
		err = errNotFound
	}
	return Result{}, err
}

func (r *Resolver) fetchPrimary(ctx context.Context, host string) (Result, bool, error) {
	raceCtx, cancelRace := context.WithCancel(ctx)
	defer cancelRace()

	type attempt struct {
		kind   sourceKind
		result Result
		score  int
		err    error
	}
	sources := []struct {
		kind sourceKind
		url  string
	}{
		{sourceDirect, directURL(host)},
		{sourceGoogleS2, s2URL(host)},
		{sourceIconHorse, aggregatorURL(host)},
	}
	results := make(chan attempt, len(sources))
	for _, src := range sources {
		go func(kind sourceKind, target string) {
			res, score, err := r.request(raceCtx, kind, host, target)
			results <- attempt{kind: kind, result: res, score: score, err: err}
		}(src.kind, src.url)
	}

	var best attempt
	var hasBest bool
	var lastErr error
	var queued bool
	var settleC <-chan time.Time
	var settleTimer *time.Timer
	defer func() {
		if settleTimer != nil {
			settleTimer.Stop()
		}
	}()

	received := 0
	for received < len(sources) {
		select {
		case got := <-results:
			received++
			if got.err == nil && len(got.result.Data) > 0 {
				if !hasBest || got.score > best.score {
					best = got
					hasBest = true
				}
				if got.kind == sourceDirect && got.score >= directImmediateScore {
					return got.result, false, nil
				}
				if settleTimer == nil && received < len(sources) {
					settleTimer = time.NewTimer(r.raceSettleWindow())
					settleC = settleTimer.C
				}
			} else if got.err != nil {
				lastErr = got.err
				queued = queued || errors.Is(got.err, errFetchQueued)
			}
		case <-settleC:
			if hasBest {
				return best.result, false, nil
			}
			settleC = nil
		case <-raceCtx.Done():
			if hasBest {
				return best.result, false, nil
			}
			if queued {
				return Result{}, true, errFetchQueued
			}
			return Result{}, false, raceCtx.Err()
		}
	}
	if hasBest {
		return best.result, false, nil
	}
	if queued {
		return Result{}, true, errFetchQueued
	}
	if lastErr == nil {
		lastErr = errNotFound
	}
	return Result{}, false, lastErr
}

// fetchHTMLIcon reads a bounded head prefix from https://<host>/ and attempts
// up to two raster <link rel="icon" / "apple-touch-icon"> URLs.
func (r *Resolver) fetchHTMLIcon(ctx context.Context, host string) (Result, error) {
	iconURLs, err := r.discoverHTMLIcons(ctx, host)
	if err != nil || len(iconURLs) == 0 {
		return Result{}, errNotFound
	}
	limit := min(len(iconURLs), 2)
	for i := range limit {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		res, _, reqErr := r.request(ctx, sourceHTML, host, iconURLs[i])
		if reqErr == nil && len(res.Data) > 0 {
			return res, nil
		}
	}
	return Result{}, errNotFound
}

func (r *Resolver) discoverHTMLIcons(ctx context.Context, host string) ([]string, error) {
	if err := r.requests.Acquire(ctx, 1); err != nil {
		return nil, errFetchQueued
	}
	defer r.requests.Release(1)

	target := "https://" + host + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Referer", "")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("favicon: html status %d", resp.StatusCode)
	}
	if ct := strings.ToLower(resp.Header.Get("Content-Type")); ct != "" &&
		!strings.Contains(ct, "html") && !strings.Contains(ct, "xhtml") {
		return nil, fmt.Errorf("favicon: non-html content type %q", ct)
	}
	snippet, err := io.ReadAll(io.LimitReader(resp.Body, maxHTMLBytes))
	if err != nil || len(snippet) == 0 {
		return nil, errNotFound
	}
	baseURL := req.URL
	if resp.Request != nil && resp.Request.URL != nil {
		baseURL = resp.Request.URL
	}
	return extractHTMLIconURLs(baseURL, snippet), nil
}

// request fetches one URL, validates it is a non-placeholder image, and scores it.
func (r *Resolver) request(ctx context.Context, source sourceKind, requestedHost, target string) (Result, int, error) {
	if err := r.requests.Acquire(ctx, 1); err != nil {
		return Result{}, 0, errFetchQueued
	}
	defer r.requests.Release(1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Result{}, 0, err
	}
	req.Header.Set("Accept", "image/*")
	// No cookies, no referrer: only the hostname may leave the device.
	req.Header.Set("Referer", "")

	resp, err := r.client.Do(req)
	if err != nil {
		return Result{}, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, 0, fmt.Errorf("favicon: status %d", resp.StatusCode)
	}
	data, err := readCapped(resp.Body, maxIconBytes)
	if err != nil {
		return Result{}, 0, err
	}
	if len(data) == 0 {
		return Result{}, 0, errNotFound
	}
	ct := http.DetectContentType(data)
	if !strings.HasPrefix(ct, "image/") {
		return Result{}, 0, fmt.Errorf("favicon: not an image (%s)", ct)
	}
	score, err := validateAndScore(source, requestedHost, resp, data)
	if err != nil {
		return Result{}, 0, err
	}
	return Result{Data: data, ContentType: ct}, score, nil
}

// --- disk cache ---

func (r *Resolver) diskPath(host string) string {
	sum := sha256.Sum256([]byte(diskCacheVersion + host))
	return filepath.Join(r.cacheDir, hex.EncodeToString(sum[:])+".ico")
}

func (r *Resolver) readDisk(host string) (Result, bool) {
	if r.cacheDir == "" {
		return Result{}, false
	}
	r.disk.initialize(r.cacheDir)
	file, err := os.Open(r.diskPath(host))
	if err != nil {
		return Result{}, false
	}
	defer file.Close()
	if info, statErr := file.Stat(); statErr == nil && time.Since(info.ModTime()) > diskCacheTTL {
		return Result{}, false
	}
	data, err := readCapped(file, maxIconBytes)
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
	r.disk.write(r.cacheDir, r.diskPath(host), result.Data)
}

// --- helpers ---

var errNotFound = errors.New("favicon: not found")
var errFetchQueued = errors.New("favicon: canceled while waiting for a request slot")

// hostAliases redirects known renamed, short-link, or sub-brand hosts to the
// canonical domain that serves the official icon.
var hostAliases = map[string]string{
	"codex.com":             "chatgpt.com",
	"codex.so":              "chatgpt.com",
	"chat.openai.com":       "chatgpt.com",
	"twitter.com":           "x.com",
	"t.co":                  "x.com",
	"youtu.be":              "youtube.com",
	"b23.tv":                "bilibili.com",
	"notion.site":           "notion.so",
	"app.slack.com":         "slack.com",
	"makersuite.google.com": "aistudio.google.com",
	"larkoffice.com":        "feishu.cn",
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

// s2URL is Google's favicon aggregator. Using domain_url=https://<host> instead
// of bare domain=<host> preserves product-specific icons for Google subdomains
// such as mail.google.com, drive.google.com, and calendar.google.com.
func s2URL(host string) string {
	return "https://www.google.com/s2/favicons?sz=64&domain_url=" + url.QueryEscape("https://"+host)
}

// aggregatorURL is a second aggregator that stays reachable when Google is
// blocked and no proxy is configured (docs/decisions/timeline-favicon-fetch.md);
// only the hostname is sent.
func aggregatorURL(host string) string {
	return "https://icon.horse/icon/" + url.PathEscape(host)
}
