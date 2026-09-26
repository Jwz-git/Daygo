package app

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"net/http/httptest"
	"strconv"
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
