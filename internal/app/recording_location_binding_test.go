package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
	"github.com/Jwz-git/Daygo/internal/recorder"
	"github.com/Jwz-git/Daygo/internal/recordinglocation"
	"github.com/Jwz-git/Daygo/internal/storage"
)

func TestWindowsRecordingDirectoryMoveKeepsHistoryAndPersistsRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows directory picker and live migration")
	}
	dir := t.TempDir()
	store, err := storage.Open(context.Background(), storage.Options{Dir: dir, CaptureOwnerRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	source := filepath.Join(dir, "recordings")
	if err := os.MkdirAll(filepath.Join(source, "segments"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(source, "segments", "fixture.mp4")
	if err := os.WriteFile(file, []byte("anonymous frames"), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "moved")
	b := NewBackend(nil, store)
	if err := b.MoveRecordingDirectory(target); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetRecordingDirectory()
	if err != nil || got != target {
		t.Fatalf("directory = %q, %v", got, err)
	}
	data, err := os.ReadFile(filepath.Join(target, "segments", "fixture.mp4"))
	if err != nil || string(data) != "anonymous frames" {
		t.Fatalf("moved bytes = %q, %v", data, err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("old file still exists: %v", err)
	}
	stored, err := recordinglocation.Active(context.Background(), store.Settings(), source)
	if err != nil || stored != target {
		t.Fatalf("persisted root = %q, %v", stored, err)
	}
}

func TestWindowsRecordingDirectoryMoveRestoresActiveRecorder(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows live migration")
	}
	dir := t.TempDir()
	store, err := storage.Open(context.Background(), storage.Options{Dir: dir, CaptureOwnerRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	b := NewBackend(nil, store)
	b.setCapture(fake.NewCapture())
	if err := b.SetRecording(true); err != nil {
		t.Fatal(err)
	}
	defer b.shutdown()
	deadline := time.Now().Add(3 * time.Second)
	for b.recorderState() != recorder.StateCapturing {
		if time.Now().After(deadline) {
			t.Fatal("recorder did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	target := filepath.Join(t.TempDir(), "moved")
	if err := b.MoveRecordingDirectory(target); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for b.recorderState() != recorder.StateCapturing {
		if time.Now().After(deadline) {
			t.Fatal("recorder did not resume")
		}
		time.Sleep(5 * time.Millisecond)
	}
	root, err := b.GetRecordingDirectory()
	if err != nil || root != target {
		t.Fatalf("active root = %q, %v", root, err)
	}
}

func TestRecordingDirectoryMoveRequiresOwner(t *testing.T) {
	dir := t.TempDir()
	owner, err := storage.Open(context.Background(), storage.Options{Dir: dir, CaptureOwnerRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	reader, err := storage.Open(context.Background(), storage.Options{Dir: dir, CaptureOwnerRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := NewBackend(nil, reader).MoveRecordingDirectory(filepath.Join(t.TempDir(), "target")); err == nil {
		t.Fatal("read-only instance moved recording directory")
	}
}

func TestWindowsRecordingDirectoryMovePreservesSystemPause(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows live migration")
	}
	dir := t.TempDir()
	store, err := storage.Open(context.Background(), storage.Options{Dir: dir, CaptureOwnerRequested: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	b := NewBackend(nil, store)
	b.setCapture(fake.NewCapture())
	if err := b.SetRecording(true); err != nil {
		t.Fatal(err)
	}
	defer b.shutdown()
	deadline := time.Now().Add(3 * time.Second)
	for b.recorderState() != recorder.StateCapturing {
		if time.Now().After(deadline) {
			t.Fatal("recorder did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	b.recorder.HandleSystemEvent(platform.SystemEvent{Kind: platform.EventScreenLocked})
	if b.recorderState() != recorder.StatePaused {
		t.Fatal("screen lock did not pause recorder")
	}
	if err := b.MoveRecordingDirectory(filepath.Join(t.TempDir(), "moved")); err != nil {
		t.Fatal(err)
	}
	if b.recorder.UserPaused() {
		t.Fatal("system pause was converted into an indefinite user pause")
	}
	b.recorder.HandleSystemEvent(platform.SystemEvent{Kind: platform.EventScreenUnlocked})
	deadline = time.Now().Add(3 * time.Second)
	for b.recorderState() != recorder.StateCapturing {
		if time.Now().After(deadline) {
			t.Fatal("recorder did not resume after unlock")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
