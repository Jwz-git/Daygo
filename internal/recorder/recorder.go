// Package recorder owns the Go recording lifecycle. It drives one-shot Capture
// calls and persists their intent before pixels are written.
package recorder

import (
	"context"
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
	Begin(context.Context, string, time.Time, *int, int, int, bool) (int64, error)
	Commit(context.Context, int64, int64) error
	MarkBlocked(context.Context, int64) error
	Abandon(context.Context, int64) error
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
	systemBlockers   map[platform.SystemEventKind]struct{}
	resumeGeneration uint64
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
	r.resumeGeneration++
	r.mu.Unlock()
	r.emit(StateStarting, nil)
	go r.run(runCtx)
	return nil
}
func (r *Recorder) Stop() error {
	r.mu.Lock()
	if r.state == StateIdle {
		return nil
	}
	cancel := r.cancel
	done := r.done
	r.mu.Unlock()
	cancel()
	<-done
	return nil
}
func (r *Recorder) Pause() error {
	r.mu.Lock()
	if r.state != StateCapturing {
		r.mu.Unlock()
		return fmt.Errorf("recorder: cannot pause from %s", r.state)
	}
	r.userPaused = true
	r.state = StatePaused
	r.resumeGeneration++
	r.mu.Unlock()
	r.emit(StatePaused, nil)
	return nil
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
		r.resumeGeneration++
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
	defer func() { r.mu.Lock(); r.cancel = nil; r.state = StateIdle; r.mu.Unlock(); r.emit(StateIdle, nil) }()
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
			// The initial capture gets the same tolerance as the loop: emit
			// and continue rather than aborting a resident recorder.
			r.fail(err)
			consecutiveFailures := 1
			for consecutiveFailures < captureFailureLimit {
				select {
				case <-ctx.Done():
					return
				case <-time.After(captureRetryDelay):
				}
				if err := r.capture(ctx); err != nil {
					if ctx.Err() != nil {
						return
					}
					r.fail(err)
					consecutiveFailures++
					continue
				}
				break
			}
			if consecutiveFailures >= captureFailureLimit {
				return
			}
		}
	}
	consecutiveFailures := 0
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
				// One failed frame is not a reason to stop a resident
				// recorder: emit the error and keep the loop alive. Only
				// captureFailureLimit consecutive failures give up.
				r.fail(err)
				consecutiveFailures++
				if consecutiveFailures >= captureFailureLimit {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(captureRetryDelay):
				}
				continue
			}
			consecutiveFailures = 0
		}
	}
}

// captureFailureLimit is how many consecutive single-capture failures the
// loop tolerates before giving up. A display switch, a transient permission
// hiccup, or a failed placeholder write fails one frame; stopping the whole
// recorder on the first of those would silently end recording for a resident
// agent. Three in a row with no success between them means something real.
const captureFailureLimit = 3

// captureRetryDelay is the pause after a failed capture before the next
// ticker-driven attempt (the ticker itself stays on its interval).
const captureRetryDelay = 2 * time.Second

func (r *Recorder) fail(err error) { r.emit(r.State(), err) }
func (r *Recorder) capture(ctx context.Context) error {
	now := r.cfg.Clock.Now()
	current := r.captureSettings()
	name := now.UTC().Format("20060102-150405.000000000") + ".jpg"
	// Persist slash-separated segment paths on every OS. Convert to the host
	// filesystem form only when resolving the actual output file.
	rel := path.Join("staging", name)
	abs := filepath.Join(r.cfg.Directory, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return err
	}
	id, err := r.cfg.Store.Begin(ctx, rel, now, nil, current.CaptureHeightPixels*16/9, current.CaptureHeightPixels, false)
	if err != nil {
		return err
	}
	result, err := r.cfg.Capture.Capture(ctx, platform.CaptureRequest{OutputPath: abs, ImageFormat: platform.CaptureImageJPEG, TargetHeight: current.CaptureHeightPixels, JPEGQuality: r.cfg.JPEGQuality, BlockedApplicationIDs: current.BlockedApplicationIDs})
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
