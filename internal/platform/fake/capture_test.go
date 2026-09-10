package fake_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
	"github.com/Jwz-git/Daygo/internal/platform/platformtest"
)

func TestCaptureContract(t *testing.T) {
	platformtest.Suite(t, func(t *testing.T) platform.Capture {
		t.Helper()
		return fake.NewCapture()
	})
}

func TestCapturePermissionContract(t *testing.T) {
	platformtest.SuitePermission(t, func(t *testing.T) platformtest.AuthorizedCapture {
		t.Helper()
		return fake.NewCapture()
	})
}

func TestCaptureRejectsCorruptStateWithoutLeakingPath(t *testing.T) {
	directory := t.TempDir()
	statePath := filepath.Join(directory, ".daygo-fake-capture.json")
	if err := os.WriteFile(statePath, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt state: %v", err)
	}
	capture := fake.NewCapture()
	cfg := platform.CaptureConfig{Interval: time.Second, CaptureHeight: 720, SegmentDirectory: directory}
	err := capture.Start(context.Background(), cfg)
	if err == nil {
		t.Fatal("Start accepted corrupt state")
	}
	if strings.Contains(err.Error(), directory) || strings.Contains(err.Error(), statePath) {
		t.Fatalf("error leaked state path: %v", err)
	}
	if closeErr := capture.Close(context.Background()); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}
}

func TestCaptureRejectsIncompleteDurableSequence(t *testing.T) {
	tests := map[string]string{
		"missing all events":    `{"version":1,"acked":0,"max_seq":1,"events":[]}`,
		"missing first event":   `{"version":1,"acked":0,"max_seq":2,"events":[{"seq":2,"kind":"segment_closed","segment":{"SegmentPath":"fake/segment-1","TotalBytes":0,"FrameCount":1,"Succeeded":true}}]}`,
		"missing middle event":  `{"version":1,"acked":0,"max_seq":3,"events":[{"seq":1,"kind":"frame","frame":{"segment_path":"fake/segment-1","frame_index":0,"captured_at":"2026-01-01T00:00:00Z","display_id":"fake-display-1","width":1280,"height":720,"redacted":false}},{"seq":3,"kind":"segment_closed","segment":{"SegmentPath":"fake/segment-1","TotalBytes":0,"FrameCount":1,"Succeeded":true}}]}`,
		"kind payload mismatch": `{"version":1,"acked":0,"max_seq":1,"events":[{"seq":1,"kind":"frame","segment":{"SegmentPath":"fake/segment-1","TotalBytes":0,"FrameCount":1,"Succeeded":true}}]}`,
	}
	for name, state := range tests {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, ".daygo-fake-capture.json"), []byte(state), 0o600); err != nil {
				t.Fatalf("write state: %v", err)
			}
			capture := fake.NewCapture()
			err := capture.Start(context.Background(), platform.CaptureConfig{Interval: time.Second, CaptureHeight: 720, SegmentDirectory: directory})
			if err == nil {
				t.Fatal("Start accepted incomplete durable sequence")
			}
			if strings.Contains(err.Error(), directory) {
				t.Fatalf("error leaked state directory: %v", err)
			}
			if closeErr := capture.Close(context.Background()); closeErr != nil {
				t.Fatalf("Close: %v", closeErr)
			}
		})
	}
}
