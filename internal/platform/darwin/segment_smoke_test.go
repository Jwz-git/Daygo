//go:build darwin && cgo

package darwin

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

func TestNativeSegmentAppendDecodeAndLegacyFallback(t *testing.T) {
	dir := t.TempDir()

	// Close any active segment from prior runs
	if err := segmentCloseActive(); err != nil {
		t.Fatalf("segmentCloseActive: %v", err)
	}

	var segRelPath string
	// Append 3 synthetic frames
	for i := 0; i < 3; i++ {
		result, err := testFrameAppendSynthetic(dir, 64, 48, i)
		if err != nil {
			t.Fatalf("testFrameAppendSynthetic frame %d failed: %v", i, err)
		}
		if result.FrameIndex != i {
			t.Fatalf("frame %d got frame_index %d", i, result.FrameIndex)
		}
		if result.Width != 64 || result.Height != 48 {
			t.Fatalf("frame %d got geometry %dx%d, want 64x48", i, result.Width, result.Height)
		}
		segRelPath = result.SegmentPath
		if !platform.ValidSegmentPath(segRelPath) {
			t.Fatalf("invalid segment relative path: %q", segRelPath)
		}
	}

	media := NewMedia(dir)
	ctx := context.Background()

	// Finalize active segment
	if err := segmentCloseActive(); err != nil {
		t.Fatalf("segmentCloseActive: %v", err)
	}

	// Probe the finalized segment
	info, err := media.ProbeSegment(ctx, segRelPath)
	if err != nil {
		t.Fatalf("ProbeSegment: %v", err)
	}
	if !info.Readable {
		t.Fatalf("finalized segment is not readable")
	}
	if info.FrameCount < 3 {
		t.Fatalf("expected at least 3 frames, got %d", info.FrameCount)
	}
	if info.Width != 64 || info.Height != 48 {
		t.Fatalf("expected 64x48, got %dx%d", info.Width, info.Height)
	}

	// Decode all 3 frames
	for i := 0; i < 3; i++ {
		data, err := media.DecodeFrame(ctx, platform.DecodeRequest{
			SegmentPath: segRelPath,
			FrameIndex:  i,
		})
		if err != nil {
			t.Fatalf("DecodeFrame(%d): %v", i, err)
		}
		if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
			t.Fatalf("frame %d did not return valid JPEG data", i)
		}
	}

	// Test thumbnail decode with MaxPixelSize
	thumbData, err := media.DecodeFrame(ctx, platform.DecodeRequest{
		SegmentPath:  segRelPath,
		FrameIndex:   0,
		MaxPixelSize: 32,
	})
	if err != nil {
		t.Fatalf("DecodeFrame with MaxPixelSize: %v", err)
	}
	if len(thumbData) < 4 || thumbData[0] != 0xFF || thumbData[1] != 0xD8 {
		t.Fatalf("thumbnail did not return valid JPEG data")
	}

	// Test legacy JPEG direct read
	legacyRel := "legacy/legacy_frame.jpg"
	legacyAbs := filepath.Join(dir, filepath.FromSlash(legacyRel))
	if err := os.MkdirAll(filepath.Dir(legacyAbs), 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(legacyAbs)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 80}); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	// Probe legacy JPEG
	legacyInfo, err := media.ProbeSegment(ctx, legacyRel)
	if err != nil {
		t.Fatalf("ProbeSegment(legacy): %v", err)
	}
	if !legacyInfo.Readable || legacyInfo.FrameCount != 1 {
		t.Fatalf("legacy probe got readable=%v frameCount=%d", legacyInfo.Readable, legacyInfo.FrameCount)
	}

	// Decode legacy JPEG
	legacyData, err := media.DecodeFrame(ctx, platform.DecodeRequest{
		SegmentPath: legacyRel,
		FrameIndex:  0,
	})
	if err != nil {
		t.Fatalf("DecodeFrame(legacy): %v", err)
	}
	if len(legacyData) < 4 || legacyData[0] != 0xFF || legacyData[1] != 0xD8 {
		t.Fatalf("legacy JPEG decode did not return valid JPEG")
	}

	// Out of bounds frame index
	_, err = media.DecodeFrame(ctx, platform.DecodeRequest{
		SegmentPath: segRelPath,
		FrameIndex:  999,
	})
	if err == nil {
		t.Fatalf("expected error decoding frame 999, got nil")
	}
}

func TestNativeSegmentRolloverOnGeometryChange(t *testing.T) {
	dir := t.TempDir()

	if err := segmentCloseActive(); err != nil {
		t.Fatalf("segmentCloseActive: %v", err)
	}

	res1, err := testFrameAppendSynthetic(dir, 64, 48, 0)
	if err != nil {
		t.Fatalf("append 1: %v", err)
	}

	// Change dimensions -> should close old segment and start new segment
	res2, err := testFrameAppendSynthetic(dir, 32, 24, 0)
	if err != nil {
		t.Fatalf("append 2: %v", err)
	}

	if res1.SegmentPath == res2.SegmentPath {
		t.Fatalf("expected new segment path on resolution change, got same %s", res1.SegmentPath)
	}
	if res2.FrameIndex != 0 {
		t.Fatalf("expected frame_index 0 in new segment, got %d", res2.FrameIndex)
	}

	if err := segmentCloseActive(); err != nil {
		t.Fatalf("segmentCloseActive: %v", err)
	}

	media := NewMedia(dir)
	ctx := context.Background()

	info1, err := media.ProbeSegment(ctx, res1.SegmentPath)
	if err != nil || !info1.Readable || info1.Width != 64 || info1.Height != 48 {
		t.Fatalf("segment 1 probe failed: info=%+v err=%v", info1, err)
	}

	info2, err := media.ProbeSegment(ctx, res2.SegmentPath)
	if err != nil || !info2.Readable || info2.Width != 32 || info2.Height != 24 {
		t.Fatalf("segment 2 probe failed: info=%+v err=%v", info2, err)
	}
}

func TestNativeSegmentRepeatedSingleFrameDecode(t *testing.T) {
	dir := t.TempDir()
	if err := segmentCloseActive(); err != nil {
		t.Fatal(err)
	}
	frame, err := testFrameAppendSynthetic(dir, 64, 48, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := segmentCloseActive(); err != nil {
		t.Fatal(err)
	}

	media := NewMedia(dir)
	for i := 0; i < 200; i++ {
		data, err := media.DecodeFrame(context.Background(), platform.DecodeRequest{
			SegmentPath: frame.SegmentPath,
			FrameIndex:  0,
		})
		if err != nil {
			t.Fatalf("decode %d: %v", i, err)
		}
		if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
			t.Fatalf("decode %d returned invalid JPEG", i)
		}
	}
}
