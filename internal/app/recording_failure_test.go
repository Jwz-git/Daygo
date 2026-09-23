package app

import (
	"errors"
	"os"
	"testing"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/storage"
)

func TestRecordingFailureReasonRedactsDetails(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"capture timeout", &platform.CaptureError{Code: platform.CaptureTimeout, NativeCode: 0x887a0027}, "capture_timeout:0x887a0027"},
		{"storage busy", &storage.Error{Kind: storage.KindBusy, Op: "private path"}, "storage_busy"},
		{"file path", &os.PathError{Op: "write", Path: "C:\\private\\screen.jpg", Err: errors.New("denied")}, "filesystem"},
		{"unknown", errors.New("private screen content"), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := recordingFailureReason(tt.err); got != tt.want {
				t.Fatalf("reason = %q, want %q", got, tt.want)
			}
		})
	}
}
