package app

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

type fixtureMedia struct {
	platform.Media
	requests []platform.DecodeRequest
	fail     bool
}

func (m *fixtureMedia) DecodeFrame(_ context.Context, req platform.DecodeRequest) ([]byte, error) {
	m.requests = append(m.requests, req)
	if m.fail {
		return nil, errors.New("anonymous missing segment")
	}
	width, height := 1920, 1080
	if req.MaxPixelSize == 256 {
		width, height = 256, 144
	}
	var out bytes.Buffer
	err := jpeg.Encode(&out, image.NewRGBA(image.Rect(0, 0, width, height)), nil)
	return out.Bytes(), err
}
func mediaFixtureBackend(t *testing.T) (*Backend, *fixtureMedia, int64) {
	t.Helper()
	b, _ := writerBackendWithStore(t, t.TempDir())
	capture := b.store().Captures()
	ctx := context.Background()
	now := time.Now()
	pending, err := capture.Begin(ctx, "anonymous/fixture.hevc.mp4", 2, now, nil, 1920, 1080, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := capture.Commit(ctx, pending, 123); err != nil {
		t.Fatal(err)
	}
	refs, err := capture.FramesInRange(ctx, now.Add(-time.Second).Unix(), now.Add(time.Second).Unix(), 1)
	if err != nil || len(refs) != 1 {
		t.Fatalf("capture fixture: %v %+v", err, refs)
	}
	media := &fixtureMedia{}
	b.media = media
	return b, media, refs[0].ID
}
func TestMediaFrameAndThumbnailContract(t *testing.T) {
	b, media, id := mediaFixtureBackend(t)
	for _, path := range []string{"/media/frame", "/media/thumbnail"} {
		response := httptest.NewRecorder()
		b.serveAsset(response, httptest.NewRequest("GET", path+"?id="+strconv.FormatInt(id, 10), nil))
		if response.Code != 200 {
			t.Fatalf("%s status=%d", path, response.Code)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(response.Body.Bytes()))
		width, height, maxPixel := 1920, 1080, 0
		if path == "/media/thumbnail" {
			width, height, maxPixel = 256, 144, 256
		}
		if err != nil || cfg.Width != width || cfg.Height != height {
			t.Fatalf("%s geometry %+v err=%v", path, cfg, err)
		}
		req := media.requests[len(media.requests)-1]
		if req.MaxPixelSize != maxPixel || req.FrameIndex != 2 || req.SegmentPath != "anonymous/fixture.hevc.mp4" {
			t.Fatalf("decode request %+v", req)
		}
		if response.Header().Get("Content-Type") != "image/jpeg" || response.Header().Get("Cache-Control") != "private, max-age=86400, immutable" {
			t.Fatal("resource headers changed")
		}
	}
}
func TestMediaResourcesRejectInvalidIDsAndMissingPixels(t *testing.T) {
	b, media, id := mediaFixtureBackend(t)
	for _, path := range []string{"/media/frame", "/media/thumbnail"} {
		for _, query := range []string{"", "id=0", "id=-1", "id=%2B1", "id=a", "id=1234567890123456789", "id=1&id=2", "id=1&path=../../secret"} {
			response := httptest.NewRecorder()
			b.serveAsset(response, httptest.NewRequest("GET", path+"?"+query, nil))
			if response.Code != 400 {
				t.Errorf("%s ?%s status=%d want 400", path, query, response.Code)
			}
		}
		for _, target := range []string{path + "?id=99999", path + "?id=" + strconv.FormatInt(id, 10)} {
			media.fail = true
			response := httptest.NewRecorder()
			b.serveAsset(response, httptest.NewRequest("GET", target, nil))
			if response.Code != 404 {
				t.Errorf("missing pixels status=%d", response.Code)
			}
		}
	}
}

func TestMediaReusesThumbnailAndChecksFrameBeforeReturningIt(t *testing.T) {
	b, media, id := mediaFixtureBackend(t)
	path := "/media/thumbnail?id=" + strconv.FormatInt(id, 10)
	for range 2 {
		response := httptest.NewRecorder()
		b.serveAsset(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 {
			t.Fatalf("thumbnail status=%d", response.Code)
		}
	}
	if len(media.requests) != 1 {
		t.Fatalf("thumbnail decoded %d times, want 1", len(media.requests))
	}

	store := b.store()
	if err := store.Captures().AmortizeSegment(t.Context(), "anonymous/fixture.hevc.mp4", 123); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CleanupRecordings(t.Context(), t.TempDir(), 100); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	b.serveAsset(response, httptest.NewRequest("GET", path, nil))
	if response.Code != 404 || len(media.requests) != 1 {
		t.Fatalf("deleted thumbnail response=%d decode calls=%d", response.Code, len(media.requests))
	}
}

type blockingMedia struct {
	platform.Media
	started chan struct{}
	release chan struct{}
}

func (m blockingMedia) DecodeFrame(ctx context.Context, _ platform.DecodeRequest) ([]byte, error) {
	m.started <- struct{}{}
	select {
	case <-m.release:
		return []byte("fixture frame"), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestMediaBoundsConcurrentDecodes(t *testing.T) {
	const requests = frameDecodeSlots + 2
	m := blockingMedia{started: make(chan struct{}, requests), release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(m.release) })
	t.Cleanup(release)
	d := newFrameDecoder()
	done := make(chan error, requests)
	for i := range requests {
		go func() {
			_, err := d.decode(t.Context(), t.Context(), int64(i), m, platform.DecodeRequest{})
			done <- err
		}()
	}
	for range frameDecodeSlots {
		select {
		case <-m.started:
		case <-time.After(time.Second):
			t.Fatal("decode slots did not fill")
		}
	}
	select {
	case <-m.started:
		t.Fatal("more decodes started than available slots")
	case <-time.After(30 * time.Millisecond):
	}
	release()
	for range requests {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}
