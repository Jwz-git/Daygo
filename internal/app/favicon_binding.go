package app

import (
	"context"
	"net/http"
	"time"

	"github.com/Jwz-git/Daygo/internal/favicon"
)

/*
 * Site favicons for timeline cards. Resolution runs in Go, not the webview, so
 * the request uses the process network stack (proxy-aware) and results are
 * cached on disk. The host comes from a card's appSites (LLM output) and is
 * treated as untrusted: internal/favicon validates the name and refuses to
 * connect to non-public addresses (docs/decisions/timeline-favicon-fetch.md).
 */

// attachFavicons wires the resolver with a disk cache under the support dir.
// It runs during startup before any request can arrive, so no lock guards it.
func (b *Backend) attachFavicons(cacheDir string) {
	b.favicons = favicon.New(cacheDir)
}

// serveFavicon is the asset-server handler for GET /favicon?host=<host>. It
// streams the resolved icon, or 404 when the host yields nothing. Only the
// hostname reaches the network; failures are silent so the frontend falls back
// to its monogram.
func (b *Backend) serveFavicon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if b.favicons == nil {
		http.NotFound(w, r)
		return
	}
	host := r.URL.Query().Get("host")
	if host == "" {
		http.Error(w, "missing host", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()

	result, err := b.favicons.Resolve(ctx, host)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", result.ContentType)
	// Icons for a host are stable; let the webview cache aggressively.
	w.Header().Set("Cache-Control", "private, max-age=604800")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(result.Data)
}
