//go:build darwin && cgo

package darwin

import (
	"context"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/recorder"
	"github.com/Jwz-git/Daygo/internal/settings"
	"github.com/Jwz-git/Daygo/internal/storage"
)

// The real native writer receives synthetic pixels, never a screen capture.
type syntheticRecorderCapture struct{}

func (syntheticRecorderCapture) Capture(_ context.Context, req platform.CaptureRequest) (platform.CaptureResult, error) {
	return testFrameAppendSynthetic(req.SegmentDirectory, 64, 48, 0)
}
func (syntheticRecorderCapture) CloseActiveSegment(context.Context) error {
	return segmentCloseActive()
}

func TestNativeRecorderSystemPauseFinalizesReadableSegment(t *testing.T) {
	for _, kind := range []platform.SystemEventKind{platform.EventSleep, platform.EventScreenLocked, platform.EventScreensaverStart} {
		t.Run(string(kind), func(t *testing.T) {
			if err := segmentCloseActive(); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			store, err := storage.Open(t.Context(), storage.Options{Dir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			frames := make(chan struct{}, 4)
			r, err := recorder.New(recorder.Config{
				Capture: syntheticRecorderCapture{}, Store: store.Captures(), Directory: root,
				Settings: settings.Snapshot{CaptureIntervalSeconds: 60, CaptureHeightPixels: 48},
				OnEvent: func(event recorder.Event) {
					if event.State == recorder.StateCapturing {
						frames <- struct{}{}
					}
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Start(t.Context()); err != nil {
				t.Fatal(err)
			}
			defer r.Stop()
			deadline := time.After(10 * time.Second)
			for r.ActiveSegmentPath() == "" {
				select {
				case <-frames:
				case <-deadline:
					t.Fatal("synthetic capture did not commit")
				}
			}
			segment := r.ActiveSegmentPath()
			r.HandleSystemEvent(platform.SystemEvent{Kind: kind})
			if r.State() != recorder.StatePaused || r.ActiveSegmentPath() != "" {
				t.Fatal("segment remained active after pause")
			}
			info, err := NewMedia(root).ProbeSegment(t.Context(), segment)
			if err != nil || !info.Readable || info.FrameCount < 1 {
				t.Fatalf("closed native segment=%+v err=%v", info, err)
			}
			if _, err := NewMedia(root).DecodeFrame(t.Context(), platform.DecodeRequest{SegmentPath: segment, FrameIndex: 0}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
