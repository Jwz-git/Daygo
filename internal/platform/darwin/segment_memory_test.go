//go:build darwin && cgo

package darwin

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"os"
	"os/exec"
	"regexp"
	"runtime/debug"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func synthetic1080pSegment(t *testing.T) (platform.Media, string) {
	t.Helper()
	dir := t.TempDir()
	if err := segmentCloseActive(); err != nil {
		t.Fatal(err)
	}
	var rel string
	for i := 0; i < 8; i++ {
		result, err := testFrameAppendSynthetic(dir, 1920, 1080, i)
		if err != nil {
			t.Fatal(err)
		}
		rel = result.SegmentPath
	}
	if err := segmentCloseActive(); err != nil {
		t.Fatal(err)
	}
	return NewMedia(dir), rel
}

func TestNative1080pDecodeOwnershipAndConcurrentSegments(t *testing.T) {
	type segment struct {
		media platform.Media
		rel   string
	}
	segments := make([]segment, 3)
	for i := range segments {
		segments[i].media, segments[i].rel = synthetic1080pSegment(t)
	}
	ctx := context.Background()
	first, err := segments[0].media.DecodeFrame(ctx, platform.DecodeRequest{SegmentPath: segments[0].rel})
	if err != nil {
		t.Fatal(err)
	}
	preserved := bytes.Clone(first)
	errors := make(chan error, 4)
	var workers sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for i := 0; i < 12; i++ {
				s := segments[(worker+i)%len(segments)]
				maxPixel := 0
				if i%2 == 1 {
					maxPixel = 256
				}
				data, err := s.media.DecodeFrame(ctx, platform.DecodeRequest{SegmentPath: s.rel, FrameIndex: i % 8, MaxPixelSize: maxPixel})
				if err != nil {
					errors <- err
					return
				}
				cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
				width, height := 1920, 1080
				if maxPixel != 0 {
					width, height = 256, 144
				}
				if err != nil || cfg.Width != width || cfg.Height != height {
					errors <- fmt.Errorf("JPEG geometry: %+v err=%v want=%dx%d", cfg, err, width, height)
					return
				}
			}
		}(worker)
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if !bytes.Equal(first, preserved) {
		t.Fatal("later decodes changed an earlier caller-owned buffer")
	}
	s := segments[0]
	if _, err := s.media.DecodeFrame(ctx, platform.DecodeRequest{SegmentPath: s.rel, FrameIndex: 999}); err == nil {
		t.Fatal("invalid frame succeeded")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.media.DecodeFrame(canceled, platform.DecodeRequest{SegmentPath: s.rel}); err == nil {
		t.Fatal("canceled decode succeeded")
	}
	if _, err := s.media.DecodeFrame(ctx, platform.DecodeRequest{SegmentPath: s.rel}); err != nil {
		t.Fatalf("reader did not recover after errors: %v", err)
	}
}

// Opt-in physical-memory gate. Each run is a fresh process; Go GC is a test
// control only, so native retention cannot be hidden by a growing Go heap.
func TestNativeDecodeMemoryPlateau(t *testing.T) {
	if os.Getenv("DAYGO_NATIVE_MEMORY_CHILD") != "1" {
		if os.Getenv("DAYGO_NATIVE_MEMORY") != "1" {
			t.Skip("set DAYGO_NATIVE_MEMORY=1 for the three-process footprint gate")
		}
		for run := 1; run <= 3; run++ {
			cmd := exec.Command(os.Args[0], "-test.run=^TestNativeDecodeMemoryPlateau$", "-test.v")
			cmd.Env = append(os.Environ(), "DAYGO_NATIVE_MEMORY_CHILD=1")
			out, err := cmd.CombinedOutput()
			t.Logf("run %d:\n%s", run, out)
			if err != nil {
				t.Fatalf("memory run %d: %v", run, err)
			}
		}
		return
	}
	media, rel := synthetic1080pSegment(t)
	var at300 float64
	for i := 0; i < 600; i++ {
		data, err := media.DecodeFrame(context.Background(), platform.DecodeRequest{SegmentPath: rel, FrameIndex: i % 8})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if _, err := jpeg.DecodeConfig(bytes.NewReader(data)); err != nil {
				t.Fatal(err)
			}
		}
		if i == 299 || i == 599 {
			time.Sleep(3 * time.Second)
			debug.FreeOSMemory()
			raw, err := exec.Command("vmmap", "-summary", strconv.Itoa(os.Getpid())).Output()
			if err != nil {
				t.Fatal(err)
			}
			match := regexp.MustCompile(`(?m)^Physical footprint:\s+([0-9.]+)([KMG])`).FindStringSubmatch(string(raw))
			if len(match) != 3 {
				t.Fatal("vmmap omitted physical footprint")
			}
			value, err := strconv.ParseFloat(match[1], 64)
			if err != nil {
				t.Fatal(err)
			}
			switch match[2] {
			case "K":
				value /= 1024
			case "G":
				value *= 1024
			}
			t.Logf("decode=%d settled footprint=%.2f MiB", i+1, value)
			if i == 299 {
				at300 = value
			} else if value-at300 > 5 {
				t.Fatalf("300→600 retained %.2f MiB, limit 5 MiB", value-at300)
			}
		}
	}
}
