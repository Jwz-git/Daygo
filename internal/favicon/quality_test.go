package favicon

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func makePNGWithSize(w, h uint32) []byte {
	buf := bytes.Clone(pngPixel)
	binary.BigEndian.PutUint32(buf[16:20], w)
	binary.BigEndian.PutUint32(buf[20:24], h)
	return buf
}

func makeICOWithSizes(sizes ...int) []byte {
	buf := make([]byte, 6+len(sizes)*16)
	buf[2] = 1 // ICO type
	binary.LittleEndian.PutUint16(buf[4:6], uint16(len(sizes)))
	for i, s := range sizes {
		off := 6 + i*16
		b := byte(s)
		if s >= 256 {
			b = 0
		}
		buf[off] = b
		buf[off+1] = b
	}
	return buf
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func httpResponse(req *http.Request, status int, body []byte, headers map[string]string) *http.Response {
	h := make(http.Header)
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Response{
		StatusCode: status,
		Header:     h,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}
}

func TestImageDimensions(t *testing.T) {
	w, h := imageDimensions(makePNGWithSize(64, 64))
	if w != 64 || h != 64 {
		t.Fatalf("PNG dimensions = %dx%d, want 64x64", w, h)
	}
	w, h = imageDimensions(makeICOWithSizes(16, 32, 256))
	if w != 256 || h != 256 {
		t.Fatalf("ICO dimensions = %dx%d, want 256x256", w, h)
	}
	w, h = imageDimensions(makeICOWithSizes(16, 48))
	if w != 48 || h != 48 {
		t.Fatalf("ICO dimensions = %dx%d, want 48x48", w, h)
	}
}

func TestRejectsIconHorseFallbackAndLoginRedirect(t *testing.T) {
	directIcon := makePNGWithSize(48, 48)
	fallbackIcon := makePNGWithSize(128, 128)

	t.Run("fallback header rejected", func(t *testing.T) {
		r := New("")
		r.settleWindow = 50 * time.Millisecond
		r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "icon.horse" {
				return httpResponse(req, 200, fallbackIcon, map[string]string{
					"X-Icon-Fallback":   "true",
					"X-Icon-Confidence": "0",
				}), nil
			}
			return httpResponse(req, 404, nil, nil), nil
		})
		if _, err := r.Resolve(context.Background(), "no-icon.example"); err == nil {
			t.Fatal("expected icon.horse X-Icon-Fallback=true to be rejected")
		}
	})

	t.Run("accounts.google.com redirect rejected in favor of direct origin", func(t *testing.T) {
		r := New("")
		r.settleWindow = 100 * time.Millisecond
		r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Host {
			case "icon.horse":
				return httpResponse(req, 200, fallbackIcon, map[string]string{
					"X-Icon-Fallback":   "false",
					"X-Icon-Confidence": "0.6",
					"X-Icon-Source":     "https://accounts.google.com/favicon.ico",
				}), nil
			case "docs.google.com":
				time.Sleep(15 * time.Millisecond)
				return httpResponse(req, 200, directIcon, nil), nil
			default:
				return httpResponse(req, 404, nil, nil), nil
			}
		})
		got, err := r.Resolve(context.Background(), "docs.google.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(got.Data, directIcon) {
			t.Fatal("expected direct origin icon to win over mismatched accounts.google.com redirect")
		}
	})
}

func TestDirectOriginBeatsFasterAggregatorWithinSettleWindow(t *testing.T) {
	directIcon := makePNGWithSize(32, 32)
	s2Icon := makePNGWithSize(64, 64)
	r := New("")
	r.settleWindow = 120 * time.Millisecond
	r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "www.google.com":
			return httpResponse(req, 200, s2Icon, nil), nil
		case "origin.example":
			time.Sleep(20 * time.Millisecond)
			return httpResponse(req, 200, directIcon, nil), nil
		default:
			return httpResponse(req, 404, nil, nil), nil
		}
	})
	got, err := r.Resolve(context.Background(), "origin.example")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got.Data, directIcon) {
		t.Fatal("expected crisp direct origin icon to beat faster S2 aggregator")
	}
}

func TestHighResAggregatorBeatsTinyDirectICO(t *testing.T) {
	tinyDirect := makeICOWithSizes(16)
	crispHorse := makePNGWithSize(256, 256)
	r := New("")
	r.settleWindow = 100 * time.Millisecond
	r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "tiny.example":
			return httpResponse(req, 200, tinyDirect, nil), nil
		case "icon.horse":
			time.Sleep(15 * time.Millisecond)
			return httpResponse(req, 200, crispHorse, map[string]string{
				"X-Icon-Fallback":   "false",
				"X-Icon-Confidence": "0.9",
			}), nil
		default:
			return httpResponse(req, 404, nil, nil), nil
		}
	})
	got, err := r.Resolve(context.Background(), "tiny.example")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got.Data, crispHorse) {
		t.Fatal("expected high-confidence 256px icon to beat tiny 16px direct ICO")
	}
}

func TestExtractHTMLIconURLs(t *testing.T) {
	base, _ := url.Parse("https://site.example/app/")
	html := []byte(`<!doctype html><html><head>
		<link rel="icon" type="image/svg+xml" href="/logo.svg">
		<link rel="mask-icon" href="/mask.png">
		<link rel="icon" href="data:image/png;base64,AAAA">
		<link rel="icon" href="https://127.0.0.1/evil.png">
		<link rel="shortcut icon" sizes="16x16" href="/small.ico">
		<link rel="icon" type="image/png" sizes="64x64" href="/icons/icon-64.png">
		<link rel="apple-touch-icon" sizes="180x180" href="https://cdn.site.example/touch.png">
	</head><body><link rel="icon" href="/ignored-in-body.png"></body></html>`)

	urls := extractHTMLIconURLs(base, html)
	if len(urls) != 3 {
		t.Fatalf("got %d urls (%v), want 3", len(urls), urls)
	}
	if urls[0] != "https://cdn.site.example/touch.png" {
		t.Fatalf("first candidate = %q, want apple-touch-icon", urls[0])
	}
	if urls[1] != "https://site.example/icons/icon-64.png" {
		t.Fatalf("second candidate = %q, want 64x64 png", urls[1])
	}
	if urls[2] != "https://site.example/small.ico" {
		t.Fatalf("third candidate = %q, want small.ico", urls[2])
	}
}

func TestHTMLFallbackWhenPrimarySourcesFail(t *testing.T) {
	htmlIcon := makePNGWithSize(96, 96)
	r := New("")
	r.settleWindow = 30 * time.Millisecond
	r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://custom.example/":
			return httpResponse(req, 200, []byte(`<html><head><link rel="apple-touch-icon" href="/assets/brand.png"></head></html>`), map[string]string{
				"Content-Type": "text/html; charset=utf-8",
			}), nil
		case "https://custom.example/assets/brand.png":
			return httpResponse(req, 200, htmlIcon, nil), nil
		default:
			return httpResponse(req, 404, nil, nil), nil
		}
	})
	got, err := r.Resolve(context.Background(), "custom.example")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got.Data, htmlIcon) {
		t.Fatal("expected HTML <link rel=apple-touch-icon> fallback icon")
	}
}

func TestParentDomainFallback(t *testing.T) {
	cases := []struct {
		host   string
		parent string
		ok     bool
	}{
		{"docs.example.com", "example.com", true},
		{"a.b.example.com", "example.com", true},
		{"course.hdu.edu.cn", "hdu.edu.cn", true},
		{"hdu.edu.cn", "", false},
		{"example.com", "", false},
		{"user.github.io", "", false},
		{"docs.google.com", "", false},
		{"my-app.vercel.app", "", false},
	}
	for _, tc := range cases {
		got, ok := parentDomainFallback(tc.host)
		if ok != tc.ok || got != tc.parent {
			t.Errorf("parentDomainFallback(%q) = (%q, %v), want (%q, %v)", tc.host, got, ok, tc.parent, tc.ok)
		}
	}
}

func TestDiskCacheTTLExpiresOldIcons(t *testing.T) {
	r := New(t.TempDir())
	host := "stale.example"
	r.writeDisk(host, Result{Data: pngPixel, ContentType: "image/png"})
	expired := time.Now().Add(-(diskCacheTTL + time.Hour))
	if err := os.Chtimes(r.diskPath(host), expired, expired); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.readDisk(host); ok {
		t.Fatal("expected disk cache entry older than TTL to miss")
	}
}

func TestS2URLUsesDomainURLAndRejectsNonDomainExtensions(t *testing.T) {
	if got := s2URL("mail.google.com"); !strings.Contains(got, "domain_url=https%3A%2F%2Fmail.google.com") {
		t.Fatalf("s2URL = %q, want domain_url parameter", got)
	}
	for _, bad := range []string{"WeChat.exe", "installer.dmg", "service.local"} {
		if got, err := NormalizeHost(bad); err == nil {
			t.Fatalf("NormalizeHost(%q) = %q, want error", bad, got)
		}
	}
}

func TestExpandedHostAliases(t *testing.T) {
	cases := map[string]string{
		"twitter.com":           "x.com",
		"youtu.be":              "youtube.com",
		"b23.tv":                "bilibili.com",
		"notion.site":           "notion.so",
		"app.slack.com":         "slack.com",
		"chat.openai.com":       "chatgpt.com",
		"makersuite.google.com": "aistudio.google.com",
		"larkoffice.com":        "feishu.cn",
	}
	for in, want := range cases {
		if got := resolveAlias(in); got != want {
			t.Errorf("resolveAlias(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParentDomainEndToEndFallback(t *testing.T) {
	parentIcon := makePNGWithSize(64, 64)
	r := New("")
	r.settleWindow = 30 * time.Millisecond
	r.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == "https://acme.example/favicon.ico" {
			return httpResponse(req, 200, parentIcon, nil), nil
		}
		return httpResponse(req, 404, nil, nil), nil
	})
	got, err := r.Resolve(context.Background(), "docs.acme.example")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got.Data, parentIcon) {
		t.Fatal("expected parent domain direct icon as fallback for docs.acme.example")
	}
}
