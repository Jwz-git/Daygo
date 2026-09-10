package fake

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

func TestStopResumesFinalizationAfterCanceledAttempt(t *testing.T) {
	capture := NewCapture()
	directory := t.TempDir()
	done := make(chan struct{})
	close(done)

	capture.mu.Lock()
	capture.running = false
	capture.cfg = platform.CaptureConfig{Interval: time.Second, SegmentDirectory: directory}
	capture.statePath = filepath.Join(directory, stateFileName)
	capture.state = durableState{Version: 1, MaxSeq: 1, Events: []durableEvent{{
		Seq:  1,
		Kind: platform.CaptureEventFrame,
		Frame: &durableFrame{
			SegmentPath: "fake/segment-1",
			FrameIndex:  0,
			CapturedAt:  fakeEpoch,
			DisplayID:   "fake-display-1",
			Width:       1280,
			Height:      720,
		},
	}}}
	capture.current = &segmentState{path: "fake/segment-1", frames: 1, startedAt: fakeEpoch}
	capture.workerDone = done
	capture.mu.Unlock()

	if err := capture.Stop(context.Background()); err != nil {
		t.Fatalf("resume Stop: %v", err)
	}
	closed := receiveFakeEvent(t, capture.Events())
	if closed.Kind != platform.CaptureEventSegmentClosed || closed.Seq != 2 || closed.Segment == nil {
		t.Fatalf("final event = %#v, want segment_closed seq 2", closed)
	}
	if err := capture.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func receiveFakeEvent(t *testing.T, events <-chan platform.CaptureEvent) platform.CaptureEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for fake capture event")
		return platform.CaptureEvent{}
	}
}
