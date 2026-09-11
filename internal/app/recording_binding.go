package app

import (
	"context"
	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/recorder"
	"github.com/Jwz-git/Daygo/internal/settings"
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
	snapshot, err := settings.New(b.storage.Settings()).Load(ctx)
	if err != nil {
		return nil, mapStorageError("load recording settings", err)
	}
	r, err := recorder.New(recorder.Config{Capture: b.capture, Store: b.storage.Captures(), Settings: snapshot, Directory: filepath.Join(filepath.Dir(b.storage.Path()), "recordings"), OnEvent: func(e recorder.Event) {
		b.emitter.Emit(EventRecordingState, map[string]string{"state": string(e.State)})
		b.updateStatus(e.State)
	}})
	if err != nil {
		return nil, apperr.E(apperr.Internal, "create recorder", err)
	}
	b.recorder = r
	return r, nil
}

func (b *Backend) SetRecording(enabled bool) error {
	_, owner := b.instanceOwnership()
	if !owner {
		return apperr.E(apperr.NotCaptureOwner, "this instance is not the capture owner", nil)
	}
	r, err := b.ensureRecorder()
	if err != nil {
		return err
	}
	if enabled {
		if err := r.Start(context.Background()); err != nil {
			return apperr.E(apperr.Conflict, "start recording failed", err)
		}
		return nil
	}
	return r.Stop()
}
func (b *Backend) PauseRecording(minutes int) error {
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
	if err := r.Pause(); err != nil {
		return apperr.E(apperr.Conflict, "pause recording failed", err)
	}
	return nil
}
func (b *Backend) ResumeRecording() error {
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
func (b *Backend) GetRecordingDirectory() (string, error) {
	if b.storage == nil {
		return "", apperr.E(apperr.DatabaseError, "recording directory unavailable", nil)
	}
	return filepath.Join(filepath.Dir(b.storage.Path()), "recordings"), nil
}
