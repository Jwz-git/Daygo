package app

import (
	"context"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/recorder"
	"os"
	"path/filepath"
	"time"
)

func (b *Backend) ensureRecorder() (*recorder.Recorder, error) {
	b.recorderMu.Lock()
	defer b.recorderMu.Unlock()
	if b.recorder != nil {
		return b.recorder, nil
	}
	if b.capture == nil || b.storage == nil {
		return nil, apperr.E(apperr.NativeUnavailable, "recording services are unavailable", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	snapshot, err := b.loadSettings(ctx)
	if err != nil {
		return nil, mapStorageError("load recording settings", err)
	}
	directory, err := b.recordingRoot(ctx)
	if err != nil {
		return nil, mapStorageError("resolve recording directory", err)
	}
	defaultRoot := filepath.Join(filepath.Dir(b.storage.Path()), "recordings")
	if directory != defaultRoot {
		if info, err := os.Stat(directory); err != nil || !info.IsDir() {
			return nil, apperr.E(apperr.NativeUnavailable, "recording directory is unavailable", err)
		}
	}
	r, err := recorder.New(recorder.Config{Capture: b.capture, Store: b.storage.Captures(), Settings: snapshot, Directory: directory, OnEvent: b.handleRecorderEvent})
	if err != nil {
		return nil, apperr.E(apperr.Internal, "create recorder", err)
	}
	b.recorder = r
	return r, nil
}

func (b *Backend) handleRecorderEvent(event recorder.Event) {
	if event.FrameAt == nil {
		b.emitter.Emit(EventRecordingState, map[string]any{"state": string(event.State)})
		b.updateStatus(event.State)
		return
	}
	if b.GetUIVisibility().Visible {
		b.emitter.Emit(EventRecordingState, map[string]any{"state": string(event.State), "lastFrameAtTs": event.FrameAt.Unix()})
	}
	// A healthy committed frame does not change native labels or controls.
	// Retry an unapplied snapshot to recover a failed native update.
	b.statusPaintMu.Lock()
	retry := b.paintedStatus == nil && b.statusUpdater != nil
	b.statusPaintMu.Unlock()
	if retry {
		b.updateStatus(event.State)
	}
}

func (b *Backend) SetRecording(enabled bool) error {
	b.moveMu.RLock()
	defer b.moveMu.RUnlock()
	if enabled && b.updatePrepared.Load() {
		return apperr.E(apperr.Conflict, "recording is paused for update installation", nil)
	}
	if !enabled && b.updatePrepared.Load() {
		b.updateWasRecording.Store(false)
	}
	_, owner := b.instanceOwnership()
	if !owner {
		return apperr.E(apperr.NotCaptureOwner, "this instance is not the capture owner", nil)
	}
	r, err := b.ensureRecorder()
	if err != nil {
		return err
	}
	if enabled {
		if root, err := b.recordingRoot(context.Background()); err != nil {
			return mapStorageError("resolve recording directory", err)
		} else if root != filepath.Join(filepath.Dir(b.storage.Path()), "recordings") {
			if info, err := os.Stat(root); err != nil || !info.IsDir() {
				return apperr.E(apperr.NativeUnavailable, "recording directory is unavailable", err)
			}
		}
		if err := r.Start(context.Background()); err != nil {
			return apperr.E(apperr.Conflict, "start recording failed", err)
		}
		return nil
	}
	return r.Stop()
}
func (b *Backend) PauseRecording(minutes int) error {
	b.moveMu.RLock()
	defer b.moveMu.RUnlock()
	if minutes != 0 && minutes != 15 && minutes != 30 && minutes != 60 {
		return apperr.E(apperr.InvalidArgument, "pause duration is invalid", nil)
	}
	_, owner := b.instanceOwnership()
	if !owner {
		return apperr.E(apperr.NotCaptureOwner, "this instance is not the capture owner", nil)
	}
	r, err := b.ensureRecorder()
	if err != nil {
		return err
	}
	if err := r.Pause(time.Duration(minutes) * time.Minute); err != nil {
		return apperr.E(apperr.Conflict, "pause recording failed", err)
	}
	return nil
}
func (b *Backend) ResumeRecording() error {
	b.moveMu.RLock()
	defer b.moveMu.RUnlock()
	_, owner := b.instanceOwnership()
	if !owner {
		return apperr.E(apperr.NotCaptureOwner, "this instance is not the capture owner", nil)
	}
	r, err := b.ensureRecorder()
	if err != nil {
		return err
	}
	if err := r.Resume(); err != nil {
		return apperr.E(apperr.Conflict, "resume recording failed", err)
	}
	return nil
}
func (b *Backend) recorderState() recorder.State {
	b.recorderMu.Lock()
	defer b.recorderMu.Unlock()
	if b.recorder == nil {
		return recorder.StateIdle
	}
	return b.recorder.State()
}

// activeSegmentPath reports the segment the recorder is currently writing, or
// "" when no recorder exists yet or none is active. The analysis scheduler reads
// it to defer a batch whose frames still live in that unfinalized segment.
func (b *Backend) activeSegmentPath() string {
	b.recorderMu.Lock()
	defer b.recorderMu.Unlock()
	if b.recorder == nil {
		return ""
	}
	return b.recorder.ActiveSegmentPath()
}
func (b *Backend) GetRecordingDirectory() (string, error) {
	if b.storage == nil {
		return "", apperr.E(apperr.DatabaseError, "recording directory unavailable", nil)
	}
	root, err := b.recordingRoot(context.Background())
	if err != nil {
		return "", mapStorageError("recording directory", err)
	}
	return root, nil
}
