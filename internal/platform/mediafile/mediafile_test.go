package mediafile

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func writeTestJPEG(t *testing.T, dir, name string, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 200, A: 255})
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(file, img, nil); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestDecodeFrameReadsInsideRoot(t *testing.T) {
	dir := t.TempDir()
	name := writeTestJPEG(t, dir, "staging/frame-1.jpg", 8, 6)
	media := New(dir)

	data, err := media.DecodeFrame(context.Background(), platform.DecodeRequest{SegmentPath: name, FrameIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty frame")
	}
}

func TestDecodeFrameRejectsPathEscape(t *testing.T) {
	dir := t.TempDir()
	writeTestJPEG(t, dir, "staging/ok.jpg", 4, 4)
	media := New(dir)

	for _, attempt := range []string{
		"../../etc/passwd",
		"/etc/passwd",
		"staging/../../../secret.jpg",
		"..\\windows\\secret.jpg",
		"",
	} {
		if _, err := media.DecodeFrame(context.Background(), platform.DecodeRequest{SegmentPath: attempt}); err == nil {
			t.Fatalf("segment path %q escaped the root", attempt)
		}
	}
}

func TestDecodeFrameMissingAndUnrooted(t *testing.T) {
	media := New(t.TempDir())
	if _, err := media.DecodeFrame(context.Background(), platform.DecodeRequest{SegmentPath: "staging/absent.jpg"}); err == nil {
		t.Fatal("missing frame decoded")
	}
	unrooted := New("")
	if _, err := unrooted.DecodeFrame(context.Background(), platform.DecodeRequest{SegmentPath: "x.jpg"}); err == nil {
		t.Fatal("unrooted media decoded")
	}
}

func TestProbeSegmentReportsGeometryAndEncodeIsUnsupported(t *testing.T) {
	dir := t.TempDir()
	name := writeTestJPEG(t, dir, "staging/frame-2.jpg", 32, 18)
	media := New(dir)

	info, err := media.ProbeSegment(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Readable || info.FrameCount != 1 || info.Width != 32 || info.Height != 18 {
		t.Fatalf("probe=%+v", info)
	}

	if _, err := media.EncodeVideo(context.Background(), platform.EncodeRequest{}); err == nil {
		t.Fatal("EncodeVideo succeeded; encoding is a pending decision")
	}
}
