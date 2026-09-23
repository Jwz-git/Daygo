// Package recorder owns the Go recording lifecycle. It drives one-shot Capture
// calls and persists their intent before pixels are written.
package recorder

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/settings"
)

type State string

const (
	StateIdle      State = "idle"
	StateStarting  State = "starting"
	StateCapturing State = "capturing"
	StatePaused    State = "paused"
)

type CaptureStore interface {
	Begin(ctx context.Context, relativePath string, frameIndex int, capturedAt time.Time, idle *int, width, height int, redacted bool) (int64, error)
	Commit(context.Context, int64, int64) error
	MarkBlocked(context.Context, int64) error
	Abandon(context.Context, int64) error
	AmortizeSegment(ctx context.Context, segmentPath string, totalSize int64) error
}
type Clock interface{ Now() time.Time }
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type Event struct {
	State State
	At    time.Time
	Err   error
}

type Config struct {
	Capture     platform.Capture
	Store       CaptureStore
	Settings    settings.Snapshot
	Directory   string
	JPEGQuality int
	Clock       Clock
	OnEvent     func(Event)
}

type Recorder struct {
	mu               sync.Mutex
	cfg              Config
	state            State
	userPaused       bool
	cancel           context.CancelFunc
	done             chan struct{}
	settingsChanged  chan struct{}
	lastFrameAt      *time.Time
	lastError        error
	systemBlockers   map[platform.SystemEventKind]struct{}
	resumeGeneration uint64
	lastSegmentPath  string
	lastSegmentSize  int64
}

func New(cfg Config) (*Recorder, error) {
	if cfg.Capture == nil || cfg.Store == nil {
		return nil, fmt.Errorf("recorder: capture and store are required")
	}
	if cfg.Directory == "" || !filepath.IsAbs(cfg.Directory) {
		return nil, fmt.Errorf("recorder: directory must be absolute")
	}
	if cfg.JPEGQuality == 0 {
		cfg.JPEGQuality = 85
	}
	if cfg.Clock == nil {
		cfg.Clock = realClock{}
	}
	return &Recorder{cfg: cfg, state: StateIdle, settingsChanged: make(chan struct{}, 1), systemBlockers: make(map[platform.SystemEventKind]struct{})}, nil
}
func (r *Recorder) State() State { r.mu.Lock(); defer r.mu.Unlock(); return r.state }

// ActiveSegmentPath reports the segment the recorder is currently writing into,
// or "" when no segment is active (idle, paused, stopped, or legacy staging
// mode). A frame in the active segment has no finalized container yet, so it
// cannot be decoded until the segment rolls over or is finalized on pause/stop;
// the analysis scheduler reads this to defer a batch that still references it.
func (r *Recorder) ActiveSegmentPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastSegmentPath
}

// LastFrameAt reports the last frame that was fully written and committed to
// storage. A defensive copy keeps callers from sharing mutable state with the
// recorder goroutine.
func (r *Recorder) LastFrameAt() *time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastFrameAt == nil {
		return nil
	}
	value := *r.lastFrameAt
	return &value
}
func (r *Recorder) LastError() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastError
}
func (r *Recorder) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.state != StateIdle {
		return fmt.Errorf("recorder: cannot start from %s", r.state)
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.done = make(chan struct{})
	r.state = StateStarting
	r.userPaused = false
	r.lastError = nil
	r.resumeGeneration++
	r.mu.Unlock()
	r.emit(StateStarting, nil)
	go r.run(runCtx)
	return nil
}
func (r *Recorder) Stop() error {
	r.mu.Lock()
	if r.state == StateIdle {
		r.mu.Unlock()
		return nil
	}
	cancel := r.cancel
	done := r.done
	r.lastError = nil
	// Bump the resume generation so any pending user-pause or system-event
	// timer no-ops when it fires: the state guard alone loses the race where
	// the timer wins the lock before run()'s deferred teardown sets StateIdle.
	r.resumeGeneration++
	r.mu.Unlock()
	cancel()
	<-done
	return nil
}

// Pause stops capture at the user's request. A positive duration schedules an
// automatic resume once it elapses; a zero duration pauses indefinitely until
// the user resumes. The auto-resume is guarded by resumeGeneration, so a later
// Resume, Stop, or system event cancels a pending timer just as it does for the
// system-event resume path.
func (r *Recorder) Pause(duration time.Duration) error {
	r.mu.Lock()
	if r.state != StateCapturing {
		r.mu.Unlock()
		return fmt.Errorf("recorder: cannot pause from %s", r.state)
	}
	r.userPaused = true
	r.state = StatePaused
	r.resumeGeneration++
	generation := r.resumeGeneration
	r.mu.Unlock()
	if closer, ok := r.cfg.Capture.(platform.SegmentCloser); ok {
		_ = closer.CloseActiveSegment(context.Background())
		r.mu.Lock()
		activePath := r.lastSegmentPath
		activeSize := r.lastSegmentSize
		r.lastSegmentPath = ""
		r.lastSegmentSize = 0
		r.mu.Unlock()
		if activePath != "" {
			_ = r.cfg.Store.AmortizeSegment(context.Background(), activePath, activeSize)
		}
	}
	if duration > 0 {
		time.AfterFunc(duration, func() { r.resumeAfterUserPause(generation) })
	}
	r.emit(StatePaused, nil)
	return nil
}

// resumeAfterUserPause ends a timed user pause. It drops the user hold in every
// case the generation still matches, but only returns to capturing when no
// system blocker (sleep, lock, screensaver) is active — a timed pause that
// expires while the screen is locked must not start capturing an unavailable
// display; capture resumes when the system unblocks instead.
func (r *Recorder) resumeAfterUserPause(generation uint64) {
	r.mu.Lock()
	if generation != r.resumeGeneration || r.state != StatePaused {
		r.mu.Unlock()
		return
	}
	r.userPaused = false
	if len(r.systemBlockers) != 0 {
		r.mu.Unlock()
		return
	}
	r.state = StateCapturing
	r.mu.Unlock()
	r.emit(StateCapturing, nil)
}
func (r *Recorder) Resume() error {
	r.mu.Lock()
	if r.state != StatePaused {
		r.mu.Unlock()
		return fmt.Errorf("recorder: cannot resume from %s", r.state)
	}
	if len(r.systemBlockers) != 0 {
		r.mu.Unlock()
		return fmt.Errorf("recorder: cannot resume while system capture is blocked")
	}
	r.userPaused = false
	r.state = StateCapturing
	r.resumeGeneration++
	r.mu.Unlock()
	r.emit(StateCapturing, nil)
	return nil
}

// UpdateSettings applies capture settings to subsequent frames without
// restarting the recorder or changing its lifecycle state.
func (r *Recorder) UpdateSettings(next settings.Snapshot) {
	next.BlockedApplicationIDs = append([]string(nil), next.BlockedApplicationIDs...)
	r.mu.Lock()
	r.cfg.Settings = next
	r.mu.Unlock()
	select {
	case r.settingsChanged <- struct{}{}:
	default:
	}
}

func (r *Recorder) captureSettings() settings.Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.cfg.Settings
	current.BlockedApplicationIDs = append([]string(nil), current.BlockedApplicationIDs...)
	return current
}

// HandleSystemEvent pauses for sleep/lock/screensaver and resumes after the
// documented wake/unlock grace period. User pause is never auto-resumed.
func (r *Recorder) HandleSystemEvent(event platform.SystemEvent) {
	switch event.Kind {
	case platform.EventSleep, platform.EventScreenLocked, platform.EventScreensaverStart:
		r.mu.Lock()
		r.systemBlockers[event.Kind] = struct{}{}
		// A blocker does NOT bump resumeGeneration: it is not a resume decision,
		// only a hold. A stale system-event resume timer already no-ops here via
		// its len(systemBlockers) != 0 guard, and bumping would instead orphan a
		// pending timed user-pause timer — its resumeAfterUserPause would then
		// find a stale generation, never clear userPaused, and the wake branch
		// below would refuse to re-arm, freezing a timed pause forever.
		if r.state == StateCapturing {
			r.state = StatePaused
			r.mu.Unlock()
			r.emit(StatePaused, nil)
		} else {
			r.mu.Unlock()
		}
	case platform.EventWake, platform.EventScreenUnlocked, platform.EventScreensaverStop:
		r.mu.Lock()
		delete(r.systemBlockers, blockingEventFor(event.Kind))
		if r.userPaused || r.state != StatePaused || len(r.systemBlockers) != 0 {
			r.mu.Unlock()
			return
		}
		delay := 5 * time.Second
		if event.Kind != platform.EventWake {
			delay = 500 * time.Millisecond
		}
		r.resumeGeneration++
		generation := r.resumeGeneration
		r.mu.Unlock()
		time.AfterFunc(delay, func() { r.resumeAfterSystemEvent(generation) })
	}
}

func blockingEventFor(event platform.SystemEventKind) platform.SystemEventKind {
	switch event {
	case platform.EventWake:
		return platform.EventSleep
	case platform.EventScreenUnlocked:
		return platform.EventScreenLocked
	case platform.EventScreensaverStop:
		return platform.EventScreensaverStart
	default:
		return ""
	}
}

func (r *Recorder) resumeAfterSystemEvent(generation uint64) {
	r.mu.Lock()
	if generation != r.resumeGeneration || r.userPaused || r.state != StatePaused || len(r.systemBlockers) != 0 {
		r.mu.Unlock()
		return
	}
	r.state = StateCapturing
	r.mu.Unlock()
	r.emit(StateCapturing, nil)
}
func (r *Recorder) emit(s State, e error) {
	if r.cfg.OnEvent != nil {
		r.cfg.OnEvent(Event{State: s, At: r.cfg.Clock.Now(), Err: e})
	}
}
func (r *Recorder) run(ctx context.Context) {
	defer close(r.done)
	defer func() {
		if closer, ok := r.cfg.Capture.(platform.SegmentCloser); ok {
			_ = closer.CloseActiveSegment(context.Background())
			r.mu.Lock()
			activePath := r.lastSegmentPath
			activeSize := r.lastSegmentSize
			r.lastSegmentPath = ""
			r.lastSegmentSize = 0
			r.mu.Unlock()
			if activePath != "" {
				_ = r.cfg.Store.AmortizeSegment(context.Background(), activePath, activeSize)
			}
		}
		r.mu.Lock()
		r.cancel = nil
		r.state = StateIdle
		r.mu.Unlock()
		r.emit(StateIdle, nil)
	}()
	if err := os.MkdirAll(r.cfg.Directory, 0700); err != nil {
		r.fail(err)
		return
	}
	r.mu.Lock()
	blocked := len(r.systemBlockers) != 0
	if blocked {
		r.state = StatePaused
	} else {
		r.state = StateCapturing
	}
	state := r.state
	r.mu.Unlock()
	r.emit(state, nil)
	current := r.captureSettings()
	ticker := time.NewTicker(time.Duration(current.CaptureIntervalSeconds) * time.Second)
	defer ticker.Stop()
	if !blocked {
		if err := r.capture(ctx); err != nil && ctx.Err() == nil {
			r.fail(err)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.settingsChanged:
			current = r.captureSettings()
			ticker.Reset(time.Duration(current.CaptureIntervalSeconds) * time.Second)
		case <-ticker.C:
			r.mu.Lock()
			paused := r.state == StatePaused
			r.mu.Unlock()
			if paused {
				continue
			}
			if err := r.capture(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				// Keep the resident recorder alive through sustained errors. An
				// error event and LastError expose the failure until a frame lands.
				r.fail(err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(captureRetryDelay):
				}
				continue
			}
		}
	}
}

// captureRetryDelay is the pause after a failed capture before the next
// ticker-driven attempt (the ticker itself stays on its interval).
const captureRetryDelay = 2 * time.Second

func (r *Recorder) fail(err error) {
	r.mu.Lock()
	r.lastError = err
	state := r.state
	r.mu.Unlock()
	r.emit(state, err)
}
func (r *Recorder) capture(ctx context.Context) error {
	now := r.cfg.Clock.Now()
	current := r.captureSettings()

	if _, ok := r.cfg.Capture.(platform.SegmentCloser); ok {
		req := platform.CaptureRequest{
			SegmentDirectory:      r.cfg.Directory,
			ImageFormat:           platform.CaptureImageJPEG,
			TargetHeight:          current.CaptureHeightPixels,
			JPEGQuality:           r.cfg.JPEGQuality,
			ShowsCursor:           true,
			BlockedApplicationIDs: current.BlockedApplicationIDs,
		}
		result, err := r.cfg.Capture.Capture(ctx, req)
		if err != nil {
			var capErr *platform.CaptureError
			if errors.As(err, &capErr) && capErr.Code == platform.CaptureUnsupported {
				return r.captureLegacy(ctx, now, current)
			}
			return err
		}

		r.mu.Lock()
		paused := r.state == StatePaused
		r.mu.Unlock()
		if paused {
			return nil
		}

		redacted := result.Outcome == platform.CaptureBlocked
		id, err := r.cfg.Store.Begin(ctx, result.SegmentPath, result.FrameIndex, result.CapturedAt, nil, result.Width, result.Height, redacted)
		if err != nil {
			return err
		}
		if redacted {
			if err := r.cfg.Store.MarkBlocked(ctx, id); err != nil {
				return err
			}
		}
		r.mu.Lock()
		var rolledPath string
		var rolledSize int64
		if r.lastSegmentPath != "" && r.lastSegmentPath != result.SegmentPath {
			rolledPath = r.lastSegmentPath
			rolledSize = r.lastSegmentSize
			r.lastSegmentSize = 0
		}
		r.lastSegmentPath = result.SegmentPath

		frameDelta := result.FileSize - r.lastSegmentSize
		if frameDelta <= 0 {
			frameDelta = 1
		}
		r.lastSegmentSize = result.FileSize
		r.mu.Unlock()

		if rolledPath != "" {
			_ = r.cfg.Store.AmortizeSegment(ctx, rolledPath, rolledSize)
		}

		if err := r.cfg.Store.Commit(ctx, id, frameDelta); err != nil {
			return err
		}

		r.mu.Lock()
		r.lastFrameAt = &now
		r.lastError = nil
		state := r.state
		r.mu.Unlock()
		r.emit(state, nil)
		return nil
	}

	return r.captureLegacy(ctx, now, current)
}

func (r *Recorder) captureLegacy(ctx context.Context, now time.Time, current settings.Snapshot) error {
	name := now.UTC().Format("20060102-150405.000000000") + ".jpg"
	// Persist slash-separated segment paths on every OS. Convert to the host
	// filesystem form only when resolving the actual output file.
	rel := path.Join("staging", name)
	abs := filepath.Join(r.cfg.Directory, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return err
	}
	id, err := r.cfg.Store.Begin(ctx, rel, 0, now, nil, current.CaptureHeightPixels*16/9, current.CaptureHeightPixels, false)
	if err != nil {
		return err
	}
	result, err := r.cfg.Capture.Capture(ctx, platform.CaptureRequest{
		OutputPath:            abs,
		ImageFormat:           platform.CaptureImageJPEG,
		TargetHeight:          current.CaptureHeightPixels,
		JPEGQuality:           r.cfg.JPEGQuality,
		BlockedApplicationIDs: current.BlockedApplicationIDs,
	})
	if err != nil {
		// The adapter failed before writing a usable file; whether the file
		// exists is unknown, so the intent is abandoned rather than left for
		// a Reconcile that might commit a half-written image.
		_ = r.cfg.Store.Abandon(ctx, id)
		return err
	}
	r.mu.Lock()
	paused := r.state == StatePaused
	r.mu.Unlock()
	if paused {
		_ = os.Remove(abs)
		// The pending row must go with the file: Reconcile only settles
		// intents whose files exist or stat cleanly, so a row left here
		// would leak forever. Abandoning the just-begun intent is safe —
		// nothing committed it, so no screenshots row references it.
		if err := r.cfg.Store.Abandon(ctx, id); err != nil {
			return err
		}
		return nil
	}
	if result.Outcome == platform.CaptureBlocked {
		if err := writePlaceholder(abs, current.CaptureHeightPixels, r.cfg.JPEGQuality); err != nil {
			_ = r.cfg.Store.Abandon(ctx, id)
			return err
		}
		if err := r.cfg.Store.MarkBlocked(ctx, id); err != nil {
			return err
		}
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if err := r.cfg.Store.Commit(ctx, id, info.Size()); err != nil {
		return err
	}
	r.mu.Lock()
	r.lastFrameAt = &now
	r.lastError = nil
	state := r.state
	r.mu.Unlock()
	// The externally visible recording snapshot changed even though the
	// lifecycle state did not. Reusing the state event lets Wails clients
	// invalidate GetRecordingState without introducing a test-only event.
	r.emit(state, nil)
	return nil
}
func writePlaceholder(path string, height, quality int) error {
	_ = height
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, image.NewUniform(color.RGBA{R: 32, G: 32, B: 32, A: 255}), &jpeg.Options{Quality: quality})
}
