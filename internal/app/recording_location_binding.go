package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/recorder"
	"github.com/Jwz-git/Daygo/internal/recordinglocation"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// PickRecordingDirectory opens the native directory picker. An empty result
// means the user canceled the picker and makes no change.
func (b *Backend) PickRecordingDirectory() (string, error) {
	if runtime.GOOS != "windows" {
		return "", apperr.E(apperr.NativeUnavailable, "recording directory migration is Windows-only", nil)
	}
	b.windowCtxMu.Lock()
	ctx := b.windowCtx
	b.windowCtxMu.Unlock()
	if ctx == nil {
		return "", apperr.E(apperr.NativeUnavailable, "directory picker unavailable", nil)
	}
	return wailsruntime.OpenDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{})
}

// MoveRecordingDirectory relocates the entire recordings tree. The destination
// must be empty; its old files stay authoritative until the commit.
func (b *Backend) MoveRecordingDirectory(target string) error {
	if runtime.GOOS != "windows" {
		return apperr.E(apperr.NativeUnavailable, "recording directory migration is Windows-only", nil)
	}
	canWrite, owner := b.instanceOwnership()
	if !canWrite || !owner {
		return apperr.E(apperr.NotCaptureOwner, "capture owner required", nil)
	}
	if !filepath.IsAbs(target) {
		return apperr.E(apperr.InvalidArgument, "recording directory must be absolute", nil)
	}
	b.moveControlMu.Lock()
	if b.moveCancel != nil {
		b.moveControlMu.Unlock()
		return apperr.E(apperr.Conflict, "recording directory migration already running", nil)
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.moveCancel = cancel
	b.moveControlMu.Unlock()
	defer func() { b.moveControlMu.Lock(); b.moveCancel = nil; b.moveControlMu.Unlock(); cancel() }()
	b.moveMu.Lock()
	defer b.moveMu.Unlock()
	store := b.store()
	if store == nil {
		return apperr.E(apperr.DatabaseError, "database unavailable", nil)
	}
	source, err := b.recordingRoot(ctx)
	if err != nil {
		return mapStorageError("read recording directory", err)
	}
	defaultRoot := filepath.Join(filepath.Dir(store.Path()), "recordings")
	if source == defaultRoot {
		if err := os.MkdirAll(source, 0700); err != nil {
			return apperr.E(apperr.InvalidArgument, "recording directory unavailable", err)
		}
	} else if _, err := os.Stat(source); err != nil {
		return apperr.E(apperr.NativeUnavailable, "recording directory unavailable", err)
	}
	b.recorderMu.Lock()
	oldRecorder := b.recorder
	b.recorderMu.Unlock()
	state := recorder.StateIdle
	userPaused := false
	pauseDeadline := time.Time{}
	if oldRecorder != nil {
		state = oldRecorder.State()
		userPaused = oldRecorder.UserPaused()
		pauseDeadline = oldRecorder.PauseUntil()
		if err := oldRecorder.StopWithCause(recorder.StopMigration); err != nil {
			return apperr.E(apperr.Conflict, "could not finish active recording segment", err)
		}
	}
	restart := func() error {
		if oldRecorder == nil || state == recorder.StateIdle {
			return nil
		}
		if state == recorder.StatePaused && userPaused {
			if !pauseDeadline.IsZero() {
				if remaining := time.Until(pauseDeadline); remaining > 0 {
					return oldRecorder.StartPaused(context.Background(), remaining)
				}
				return oldRecorder.Start(context.Background())
			}
			return oldRecorder.StartPaused(context.Background(), 0)
		}
		return oldRecorder.Start(context.Background())
	}
	committed := false
	defer func() {
		if !committed {
			_ = restart()
		}
	}()
	if err := recordinglocation.Move(ctx, store.Settings(), defaultRoot, target); err != nil {
		if errors.Is(err, context.Canceled) {
			// Explicit cancellation abandons the copy intent. Source remains
			// authoritative; partial target files are left for the user to inspect.
			_ = store.Settings().Delete(context.Background(), recordinglocation.MigrationKey)
			return apperr.E(apperr.Canceled, "recording directory migration canceled", err)
		}
		return apperr.E(apperr.Conflict, "recording directory migration failed", err)
	}
	committed = true
	if oldRecorder != nil {
		if err := oldRecorder.SetDirectory(target); err != nil {
			return apperr.E(apperr.Internal, "switch recorder directory failed", err)
		}
	}
	if err := restart(); err != nil {
		return apperr.E(apperr.Conflict, "resume recording after migration failed", err)
	}
	b.emitSettingsChanged([]string{recordinglocation.DirectoryKey})
	_ = recordinglocation.Finish(context.Background(), store.Settings())
	return nil
}

func (b *Backend) CancelRecordingDirectoryMove() error {
	b.moveControlMu.Lock()
	cancel := b.moveCancel
	b.moveControlMu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

type RecordingDirectoryMigrationDTO struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Phase     string `json:"phase"`
	Available bool   `json:"available"`
}

func (b *Backend) GetRecordingDirectoryMigration() (RecordingDirectoryMigrationDTO, error) {
	if runtime.GOOS != "windows" {
		return RecordingDirectoryMigrationDTO{}, nil
	}
	store := b.store()
	if store == nil {
		return RecordingDirectoryMigrationDTO{}, apperr.E(apperr.DatabaseError, "database unavailable", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	source, target, phase, err := recordinglocation.Pending(ctx, store.Settings())
	if err != nil {
		return RecordingDirectoryMigrationDTO{}, mapStorageError("read recording migration", err)
	}
	root, err := b.recordingRoot(ctx)
	if err != nil {
		return RecordingDirectoryMigrationDTO{}, mapStorageError("read recording directory", err)
	}
	available := true
	if root != filepath.Join(filepath.Dir(store.Path()), "recordings") {
		info, statErr := os.Stat(root)
		available = statErr == nil && info.IsDir()
	}
	return RecordingDirectoryMigrationDTO{Source: source, Target: target, Phase: phase, Available: available}, nil
}
