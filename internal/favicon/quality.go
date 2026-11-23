package favicon

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type sourceKind uint8

const (
	sourceDirect sourceKind = iota
	sourceGoogleS2
	sourceIconHorse
	sourceHTML
)

const (
	// Direct origin icons of at least 32px win immediately without waiting on
	// aggregators, since the site itself is authoritative and crisp at UI sizes.
	directImmediateScore = 116

	// Verified SHA-256 of Google S2's 726-byte 16x16 default globe PNG. S2
	// normally returns HTTP 404 for it, so this is defense-in-depth in case a
	// proxy or upstream serves it with HTTP 200.
	s2DefaultGlobeSHA256 = "59bfe9bc385ad69f50793ce4a53397316d7a875a7148a63c16df9b674c6cda64"

	// Verified SHA-256 of Google's generic accounts.google.com 32x32 icon,
	// which S2 returns when a product host (e.g. docs.google.com) redirects
	// unauthenticated requests to Google sign-in.
	s2GoogleAccountsSHA256 = "63efcfe9eeda4cc58965c7587a485886612cbf878ecd8fc3e4dd594db31a67fd"
)

// loginRedirectHosts are generic identity-provider hosts that aggregators can
// accidentally scrape when a product subdomain redirects to sign-in.
var loginRedirectHosts = map[string]struct{}{
	"accounts.google.com":       {},
	"login.microsoftonline.com": {},
	"login.live.com":            {},
	"appleid.apple.com":         {},
}

var errPlaceholderIcon = errors.New("favicon: rejected placeholder or mismatched redirect icon")

// validateAndScore checks source-specific placeholder signals and computes a
// quality score so higher-fidelity official icons beat low-res or fallback ones.
func validateAndScore(source sourceKind, requestedHost string, resp *http.Response, data []byte) (int, error) {
	switch source {
	case sourceIconHorse:
		if strings.EqualFold(strings.TrimSpace(resp.Header.Get("X-Icon-Fallback")), "true") {
			return 0, errPlaceholderIcon
		}
		if rawConf := strings.TrimSpace(resp.Header.Get("X-Icon-Confidence")); rawConf != "" {
			if conf, err := strconv.ParseFloat(rawConf, 64); err == nil && conf <= 0 {
				return 0, errPlaceholderIcon
			}
		}
		if rawSource := strings.TrimSpace(resp.Header.Get("X-Icon-Source")); rawSource != "" {
			if u, err := url.Parse(rawSource); err == nil {
				srcHost := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
				if _, isLogin := loginRedirectHosts[srcHost]; isLogin && srcHost != requestedHost {
					return 0, errPlaceholderIcon
				}
			}
		}
	case sourceGoogleS2:
		sum := sha256.Sum256(data)
		digest := hex.EncodeToString(sum[:])
		if digest == s2DefaultGlobeSHA256 {
			return 0, errPlaceholderIcon
		}
		if digest == s2GoogleAccountsSHA256 && requestedHost != "accounts.google.com" && requestedHost != "google.com" {
			return 0, errPlaceholderIcon
		}
	}

	base := 80
	switch source {
	case sourceDirect:
		base = 100
	case sourceHTML:
		base = 95
	case sourceIconHorse:
		base = 85
		if rawConf := strings.TrimSpace(resp.Header.Get("X-Icon-Confidence")); rawConf != "" {
			if conf, err := strconv.ParseFloat(rawConf, 64); err == nil && conf < 0.75 {
				base = 55
			}
		}
	case sourceGoogleS2:
		base = 80
	}

	w, h := imageDimensions(data)
	side := max(w, h)
	bonus := 0
	switch {
	case side >= 64:
		bonus = 24
	case side >= 32:
		bonus = 16
	case side >= 24:
		bonus = 10
	case side >= 16:
		bonus = 4
	}
	return base + bonus, nil
}

// imageDimensions extracts pixel dimensions from PNG, ICO, GIF, WebP, or JPEG
// headers without external dependencies. Returns (0, 0) if unrecognized.
func imageDimensions(data []byte) (int, int) {
	// PNG: 8-byte signature + 4-byte len + "IHDR" + 4-byte width + 4-byte height.
	if len(data) >= 24 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) &&
		bytes.Equal(data[12:16], []byte("IHDR")) {
		return int(binary.BigEndian.Uint32(data[16:20])), int(binary.BigEndian.Uint32(data[20:24]))
	}
	// ICO: 00 00 01 00 + uint16 count + 16-byte directory entries (0 means 256).
	if len(data) >= 6 && data[0] == 0 && data[1] == 0 && data[2] == 1 && data[3] == 0 {
		count := int(binary.LittleEndian.Uint16(data[4:6]))
		bestW, bestH := 0, 0
		for i := range min(count, 32) {
			off := 6 + i*16
			if off+16 > len(data) {
				break
			}
			w := int(data[off])
			if w == 0 {
				w = 256
			}
			h := int(data[off+1])
			if h == 0 {
				h = 256
			}
			if max(w, h) > max(bestW, bestH) {
				bestW, bestH = w, h
			}
		}
		if bestW > 0 && bestH > 0 {
			return bestW, bestH
		}
	}
	// WebP: RIFF <size> WEBP <chunk>
	if len(data) >= 30 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		chunk := string(data[12:16])
		switch chunk {
		case "VP8X":
			w := 1 + (int(data[24]) | int(data[25])<<8 | int(data[26])<<16)
			h := 1 + (int(data[27]) | int(data[28])<<8 | int(data[29])<<16)
			return w, h
		case "VP8 ":
			if bytes.Equal(data[23:26], []byte{0x9d, 0x01, 0x2a}) {
				w := int(binary.LittleEndian.Uint16(data[26:28])) & 0x3fff
				h := int(binary.LittleEndian.Uint16(data[28:30])) & 0x3fff
				return w, h
			}
		case "VP8L":
			if data[20] == 0x2f {
				bits := binary.LittleEndian.Uint32(data[21:25])
				w := int(bits&0x3fff) + 1
				h := int((bits>>14)&0x3fff) + 1
				return w, h
			}
		}
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err == nil && cfg.Width > 0 && cfg.Height > 0 {
		return cfg.Width, cfg.Height
	}
	return 0, 0
}

type htmlIconCandidate struct {
	url   string
	score int
}

// extractHTMLIconURLs scans <link> tags in an HTML head snippet for raster
// icon URLs (apple-touch-icon, icon, shortcut icon), ordered by quality.
func extractHTMLIconURLs(baseURL *url.URL, html []byte) []string {
	text := string(html)
	lower := strings.ToLower(text)
	if endHead := strings.Index(lower, "</head>"); endHead >= 0 {
		text = text[:endHead]
		lower = lower[:endHead]
	}

	var candidates []htmlIconCandidate
	seen := make(map[string]struct{})
	pos := 0
	for {
		idx := strings.Index(lower[pos:], "<link")
		if idx < 0 {
			break
		}
		start := pos + idx
		// Ensure tag name boundary ("<link " / "<link\t" / "<link\n").
		after := start + len("<link")
		if after >= len(text) || (text[after] != ' ' && text[after] != '\t' && text[after] != '\n' && text[after] != '\r') {
			pos = after
			continue
		}
		end := strings.IndexByte(text[after:], '>')
		if end < 0 {
			break
		}
		tag := text[after : after+end]
		pos = after + end + 1

		attrs := parseHTMLAttributes(tag)
		rel := strings.ToLower(attrs["rel"])
		href := strings.TrimSpace(attrs["href"])
		if href == "" || rel == "" {
			continue
		}
		iconRelScore := scoreIconRel(rel)
		if iconRelScore == 0 {
			continue
		}
		// Skip SVG and data URIs: Go's http.DetectContentType only permits
		// raster image/* responses, avoiding active SVG payloads.
		if strings.HasPrefix(strings.ToLower(href), "data:") {
			continue
		}
		typ := strings.ToLower(attrs["type"])
		if strings.Contains(typ, "svg") {
			continue
		}
		resolved, err := resolveSafeIconURL(baseURL, href)
		if err != nil {
			continue
		}
		if strings.HasSuffix(strings.ToLower(resolved.Path), ".svg") {
			continue
		}
		target := resolved.String()
		if _, dup := seen[target]; dup {
			continue
		}
		seen[target] = struct{}{}

		score := iconRelScore + scoreIconSizes(attrs["sizes"])
		if strings.Contains(typ, "png") || strings.HasSuffix(strings.ToLower(resolved.Path), ".png") {
			score += 10
		}
		candidates = append(candidates, htmlIconCandidate{url: target, score: score})
	}

	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	limit := min(len(candidates), 3)
	out := make([]string, 0, limit)
	for i := range limit {
		out = append(out, candidates[i].url)
	}
	return out
}

func scoreIconRel(rel string) int {
	tokens := strings.Fields(rel)
	hasIcon := false
	hasTouch := false
	for _, tok := range tokens {
		switch tok {
		case "apple-touch-icon", "apple-touch-icon-precomposed":
			hasTouch = true
		case "icon":
			hasIcon = true
		case "mask-icon":
			return 0
		}
	}
	if hasTouch {
		return 120
	}
	if hasIcon {
		return 100
	}
	return 0
}

func scoreIconSizes(sizes string) int {
	best := 0
	for _, token := range strings.Fields(strings.ToLower(sizes)) {
		wStr, hStr, ok := strings.Cut(token, "x")
		if !ok {
			continue
		}
		w, errW := strconv.Atoi(wStr)
		h, errH := strconv.Atoi(hStr)
		if errW != nil || errH != nil || w <= 0 || h <= 0 {
			continue
		}
		side := min(max(w, h), 256)
		if side > best {
			best = side
		}
	}
	switch {
	case best >= 128:
		return 30
	case best >= 64:
		return 25
	case best >= 32:
		return 15
	case best >= 16:
		return 5
	default:
		return 0
	}
}

func resolveSafeIconURL(base *url.URL, rawHref string) (*url.URL, error) {
	ref, err := url.Parse(rawHref)
	if err != nil {
		return nil, err
	}
	resolved := base.ResolveReference(ref)
	if resolved.Scheme != "https" && resolved.Scheme != "http" {
		return nil, fmt.Errorf("favicon: unsupported icon scheme %q", resolved.Scheme)
	}
	host := strings.ToLower(resolved.Hostname())
	if host == "" || len(host) > maxHostLen || net.ParseIP(host) != nil || !isPlausibleHost(host) {
		return nil, fmt.Errorf("favicon: implausible icon host %q", host)
	}
	return resolved, nil
}

func parseHTMLAttributes(tag string) map[string]string {
	attrs := make(map[string]string)
	i := 0
	n := len(tag)
	for i < n {
		for i < n && (tag[i] == ' ' || tag[i] == '\t' || tag[i] == '\n' || tag[i] == '\r' || tag[i] == '/') {
			i++
		}
		if i >= n {
			break
		}
		nameStart := i
		for i < n && tag[i] != '=' && tag[i] != ' ' && tag[i] != '\t' && tag[i] != '\n' && tag[i] != '\r' && tag[i] != '/' {
			i++
		}
		name := strings.ToLower(tag[nameStart:i])
		for i < n && (tag[i] == ' ' || tag[i] == '\t' || tag[i] == '\n' || tag[i] == '\r') {
			i++
		}
		if i >= n || tag[i] != '=' {
			if name != "" {
				attrs[name] = ""
			}
			continue
		}
		i++ // skip '='
		for i < n && (tag[i] == ' ' || tag[i] == '\t' || tag[i] == '\n' || tag[i] == '\r') {
			i++
		}
		if i >= n {
			break
		}
		var val string
		if tag[i] == '"' || tag[i] == '\'' {
			quote := tag[i]
			i++
			valStart := i
			for i < n && tag[i] != quote {
				i++
			}
			val = tag[valStart:i]
			if i < n {
				i++
			}
		} else {
			valStart := i
			for i < n && tag[i] != ' ' && tag[i] != '\t' && tag[i] != '\n' && tag[i] != '\r' {
				i++
			}
			val = tag[valStart:i]
		}
		if name != "" {
			attrs[name] = val
		}
	}
	return attrs
}

// secondLevelPublicSuffixes are common multi-part registries where stripping
// down to two labels would yield the registry itself instead of a site domain.
var secondLevelPublicSuffixes = map[string]struct{}{
	"co.uk": {}, "org.uk": {}, "ac.uk": {},
	"com.cn": {}, "edu.cn": {}, "org.cn": {}, "gov.cn": {}, "net.cn": {}, "ac.cn": {},
	"co.jp": {}, "ne.jp": {}, "or.jp": {}, "ac.jp": {},
	"com.au": {}, "com.br": {}, "com.tw": {}, "com.hk": {}, "com.sg": {}, "co.kr": {},
}

// multiTenantParentDomains host unrelated user/product subdomains whose icons
// should not collapse onto the parent platform icon.
var multiTenantParentDomains = map[string]struct{}{
	"github.io": {}, "gitlab.io": {}, "vercel.app": {}, "netlify.app": {},
	"pages.dev": {}, "web.app": {}, "firebaseapp.com": {}, "herokuapp.com": {},
	"blogspot.com": {}, "wordpress.com": {}, "notion.site": {}, "google.com": {},
	"amazonaws.com": {}, "cloudfront.net": {}, "azurewebsites.net": {}, "workers.dev": {},
}

// parentDomainFallback derives the registrable parent domain for a subdomain
// when the subdomain itself serves no icon (e.g. "docs.example.com" -> "example.com").
func parentDomainFallback(host string) (string, bool) {
	labels := strings.Split(host, ".")
	if len(labels) < 3 {
		return "", false
	}
	lastTwo := labels[len(labels)-2] + "." + labels[len(labels)-1]
	minLabels := 2
	if _, isTwoPart := secondLevelPublicSuffixes[lastTwo]; isTwoPart {
		minLabels = 3
	}
	if len(labels) <= minLabels {
		return "", false
	}
	parent := strings.Join(labels[len(labels)-minLabels:], ".")
	if parent == host {
		return "", false
	}
	if _, multiTenant := multiTenantParentDomains[parent]; multiTenant {
		return "", false
	}
	return parent, true
}
