package fake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

const (
	eventsCapacity = 256
	statusCapacity = 16
	stateFileName  = ".daygo-fake-capture.json"
)

var (
	_               platform.Capture = (*Capture)(nil)
	errInvalidState                  = errors.New("invalid fake capture state")
	errStateIO                       = errors.New("fake capture state I/O failed")
	fakeEpoch                        = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
)

// Capture is a deterministic metadata-only implementation of platform.Capture.
type Capture struct {
	operations   sync.Mutex
	mu           sync.Mutex
	events       chan platform.CaptureEvent
	status       chan platform.CaptureStatus
	closed       bool
	running      bool
	permission   platform.PermissionState
	cfg          platform.CaptureConfig
	state        durableState
	statePath    string
	stopWorker   chan struct{}
	workerDone   chan struct{}
	current      *segmentState
	pending      []platform.CaptureEvent
	deliveryWake chan struct{}
	deliveryStop chan struct{}
	deliveryDone chan struct{}
}

type segmentState struct {
	path      string
	frames    int
	startedAt time.Time
}

type durableState struct {
	Version int            `json:"version"`
	Acked   uint64         `json:"acked"`
	MaxSeq  uint64         `json:"max_seq"`
	Events  []durableEvent `json:"events"`
}

type durableEvent struct {
	Seq     uint64                    `json:"seq"`
	Kind    platform.CaptureEventKind `json:"kind"`
	Frame   *durableFrame             `json:"frame,omitempty"`
	Segment *platform.SegmentClosed   `json:"segment,omitempty"`
}

type durableFrame struct {
	SegmentPath string    `json:"segment_path"`
	FrameIndex  int       `json:"frame_index"`
	CapturedAt  time.Time `json:"captured_at"`
	IdleSeconds *int      `json:"idle_seconds,omitempty"`
	DisplayID   string    `json:"display_id"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	Redacted    bool      `json:"redacted"`
}

// NewCapture constructs an idle fake capture adapter.
func NewCapture() *Capture {
	capture := &Capture{
		events:       make(chan platform.CaptureEvent, eventsCapacity),
		status:       make(chan platform.CaptureStatus, statusCapacity),
		permission:   platform.PermissionGranted,
		deliveryWake: make(chan struct{}, 1),
		deliveryStop: make(chan struct{}),
		deliveryDone: make(chan struct{}),
	}
	capture.publishStatusLocked(capture.statusSnapshotLocked(platform.CaptureIdle))
	go capture.deliverEvents()
	return capture
}

// SetPermission drives the screen-recording authorization this fake observes; a
// real adapter can only learn it from the OS. Losing the grant while capturing
// stops the timer and finalizes the active segment without touching the frame
// backlog, mirroring docs/04 §4.1.3.
func (c *Capture) SetPermission(state platform.PermissionState) {
	c.operations.Lock()
	defer c.operations.Unlock()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	suspend := state != c.permission && state != platform.PermissionGranted && c.running
	c.permission = state
	if suspend {
		c.mu.Unlock()
		_ = c.stop(context.Background())
		return
	}
	if c.running {
		c.publishStatusLocked(c.statusSnapshotLocked(platform.CaptureCapturing))
	} else {
		c.publishStatusLocked(platform.CaptureStatus{Phase: platform.CaptureIdle, Permission: c.permission})
	}
	c.mu.Unlock()
}

func (c *Capture) Events() <-chan platform.CaptureEvent  { return c.events }
func (c *Capture) Status() <-chan platform.CaptureStatus { return c.status }

func (c *Capture) Start(ctx context.Context, cfg platform.CaptureConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.operations.Lock()
	defer c.operations.Unlock()
	cfg = platform.CanonicalizeCaptureConfig(cfg)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("fake capture is closed")
	}
	if c.running {
		if platform.CaptureConfigEqual(c.cfg, cfg) {
			return nil
		}
		return errors.New("fake capture is already running with a different configuration")
	}
	if c.permission != platform.PermissionGranted {
		c.publishStatusLocked(platform.CaptureStatus{Phase: platform.CaptureIdle, Permission: c.permission})
		return nil
	}
	statePath := filepath.Join(cfg.SegmentDirectory, stateFileName)
	state, err := loadState(statePath)
	if err != nil {
		return fmt.Errorf("load fake capture state: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.cfg = cfg
	replay := c.statePath != statePath
	if replay {
		c.statePath = statePath
		c.state = state
	} else if state.MaxSeq > c.state.MaxSeq {
		c.state = state
	}
	c.running = true
	c.current = &segmentState{path: fmt.Sprintf("fake/segment-%020d", c.state.MaxSeq+1), startedAt: fakeEpoch.Add(time.Duration(c.state.MaxSeq) * normalizedInterval(cfg.Interval))}
	c.stopWorker = make(chan struct{})
	c.workerDone = make(chan struct{})
	c.publishStatusLocked(c.statusSnapshotLocked(platform.CaptureStarting))
	if replay {
		for _, event := range c.state.Events {
			if event.Seq > c.state.Acked {
				c.enqueueLocked(event.platformEvent())
			}
		}
	}
	c.publishStatusLocked(c.statusSnapshotLocked(platform.CaptureCapturing))
	go c.run(c.stopWorker, c.workerDone, cfg.Interval)
	return nil
}

func (c *Capture) Stop(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.operations.Lock()
	defer c.operations.Unlock()
	return c.stop(ctx)
}

func (c *Capture) stop(ctx context.Context) error {
	c.mu.Lock()
	if c.closed || (!c.running && c.current == nil) {
		c.mu.Unlock()
		return nil
	}
	stop, done := c.stopWorker, c.workerDone
	if c.running {
		c.running = false
		select {
		case <-done:
		default:
			close(stop)
		}
	}
	c.mu.Unlock()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current != nil && c.current.frames > 0 {
		event := platform.CaptureEvent{Kind: platform.CaptureEventSegmentClosed, Segment: &platform.SegmentClosed{SegmentPath: c.current.path, TotalBytes: 0, FrameCount: c.current.frames, Succeeded: true}}
		if err := c.appendLocked(event); err != nil {
			c.current = nil
			c.publishStatusLocked(platform.CaptureStatus{Phase: platform.CaptureIdle, Permission: c.permission, Fault: &platform.CaptureFault{Code: "fake_state_write", Retryable: true, Message: "fake capture state could not be persisted"}})
			return fmt.Errorf("finalize fake segment: %w", err)
		}
	}
	c.current = nil
	c.publishStatusLocked(platform.CaptureStatus{Phase: platform.CaptureIdle, Permission: c.permission})
	return nil
}

func (c *Capture) Ack(ctx context.Context, seq uint64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.operations.Lock()
	defer c.operations.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("fake capture is closed")
	}
	if c.statePath == "" || seq <= c.state.Acked {
		return nil
	}
	if seq > c.state.MaxSeq {
		seq = c.state.MaxSeq
	}
	previous := c.state
	c.state.Acked = seq
	kept := make([]durableEvent, 0, len(c.state.Events))
	for _, event := range c.state.Events {
		if event.Seq > seq {
			kept = append(kept, event)
		}
	}
	c.state.Events = kept
	if err := writeState(c.statePath, c.state); err != nil {
		c.state = previous
		return fmt.Errorf("persist fake capture acknowledgement: %w", err)
	}
	return nil
}

func (c *Capture) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.operations.Lock()
	defer c.operations.Unlock()
	if err := c.stop(ctx); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.deliveryStop)
	c.mu.Unlock()
	<-c.deliveryDone
	c.mu.Lock()
	close(c.events)
	close(c.status)
	return nil
}

func (c *Capture) run(stop <-chan struct{}, done chan<- struct{}, interval time.Duration) {
	defer close(done)
	if interval <= 0 {
		interval = time.Millisecond
	}
	if !c.captureFrame() {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if !c.captureFrame() {
				return
			}
		}
	}
}

func (c *Capture) captureFrame() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running || c.closed || c.current == nil {
		return false
	}
	if c.segmentLimitReachedLocked() {
		if err := c.closeSegmentLocked(); err != nil {
			c.failStateWriteLocked()
			return false
		}
		c.current = c.newSegmentLocked()
	}
	index := c.current.frames
	display := "fake-display-1"
	if c.cfg.PreferredDisplayID != nil {
		display = *c.cfg.PreferredDisplayID
	}
	height := c.cfg.CaptureHeight
	if height <= 0 {
		height = 720
	}
	frame := &platform.CapturedFrame{SegmentPath: c.current.path, FrameIndex: index, CapturedAt: c.current.startedAt.Add(time.Duration(index) * normalizedInterval(c.cfg.Interval)), DisplayID: display, Width: height * 16 / 9, Height: height}
	if err := c.appendLocked(platform.CaptureEvent{Kind: platform.CaptureEventFrame, Frame: frame}); err != nil {
		c.failStateWriteLocked()
		return false
	}
	c.current.frames++
	c.publishStatusLocked(c.statusSnapshotLocked(platform.CaptureCapturing))
	return true
}

func (c *Capture) segmentLimitReachedLocked() bool {
	if c.current.frames == 0 {
		return false
	}
	if c.cfg.SegmentMaxFrames > 0 && c.current.frames >= c.cfg.SegmentMaxFrames {
		return true
	}
	return c.cfg.SegmentMaxDuration > 0 &&
		time.Duration(c.current.frames)*normalizedInterval(c.cfg.Interval) >= c.cfg.SegmentMaxDuration
}

func (c *Capture) closeSegmentLocked() error {
	return c.appendLocked(platform.CaptureEvent{
		Kind: platform.CaptureEventSegmentClosed,
		Segment: &platform.SegmentClosed{
			SegmentPath: c.current.path,
			FrameCount:  c.current.frames,
			Succeeded:   true,
		},
	})
}

func (c *Capture) newSegmentLocked() *segmentState {
	return &segmentState{
		path:      fmt.Sprintf("fake/segment-%020d", c.state.MaxSeq+1),
		startedAt: fakeEpoch.Add(time.Duration(c.state.MaxSeq) * normalizedInterval(c.cfg.Interval)),
	}
}

func (c *Capture) failStateWriteLocked() {
	c.running = false
	c.publishStatusLocked(platform.CaptureStatus{Phase: platform.CaptureIdle, Permission: c.permission, Fault: &platform.CaptureFault{Code: "fake_state_write", Retryable: true, Message: "fake capture state could not be persisted"}})
}

func normalizedInterval(value time.Duration) time.Duration {
	if value <= 0 {
		return time.Millisecond
	}
	return value
}

func (c *Capture) appendLocked(event platform.CaptureEvent) error {
	event.Seq = c.state.MaxSeq + 1
	durable := durableFromPlatform(event)
	previousMaxSeq := c.state.MaxSeq
	previousVersion := c.state.Version
	c.state.MaxSeq = event.Seq
	c.state.Version = 1
	c.state.Events = append(c.state.Events, durable)
	if err := writeState(c.statePath, c.state); err != nil {
		c.state.MaxSeq = previousMaxSeq
		c.state.Version = previousVersion
		c.state.Events = c.state.Events[:len(c.state.Events)-1]
		return err
	}
	c.enqueueLocked(event)
	return nil
}

func (c *Capture) enqueueLocked(event platform.CaptureEvent) {
	c.pending = append(c.pending, event)
	select {
	case c.deliveryWake <- struct{}{}:
	default:
	}
}

func (c *Capture) deliverEvents() {
	defer close(c.deliveryDone)
	for {
		c.mu.Lock()
		if len(c.pending) == 0 {
			c.mu.Unlock()
			select {
			case <-c.deliveryWake:
				continue
			case <-c.deliveryStop:
				return
			}
		}
		event := c.pending[0]
		c.mu.Unlock()
		select {
		case c.events <- event:
			c.mu.Lock()
			if len(c.pending) > 0 && c.pending[0].Seq == event.Seq {
				c.pending = c.pending[1:]
			}
			c.mu.Unlock()
		case <-c.deliveryStop:
			return
		}
	}
}

func (c *Capture) statusSnapshotLocked(phase platform.CapturePhase) platform.CaptureStatus {
	display := "fake-display-1"
	if c.cfg.PreferredDisplayID != nil {
		display = *c.cfg.PreferredDisplayID
	}
	status := platform.CaptureStatus{Phase: phase, Permission: c.permission, ActiveDisplayID: &display}
	if c.current != nil && c.current.frames > 0 {
		at := c.current.startedAt.Add(time.Duration(c.current.frames-1) * normalizedInterval(c.cfg.Interval))
		status.LastFrameAt = &at
	}
	return status
}

func (c *Capture) publishStatusLocked(status platform.CaptureStatus) {
	select {
	case c.status <- status:
	default:
		select {
		case <-c.status:
		default:
		}
		c.status <- status
	}
}

func durableFromPlatform(event platform.CaptureEvent) durableEvent {
	result := durableEvent{Seq: event.Seq, Kind: event.Kind, Segment: event.Segment}
	if event.Frame != nil {
		result.Frame = &durableFrame{SegmentPath: event.Frame.SegmentPath, FrameIndex: event.Frame.FrameIndex, CapturedAt: event.Frame.CapturedAt, IdleSeconds: event.Frame.IdleSeconds, DisplayID: event.Frame.DisplayID, Width: event.Frame.Width, Height: event.Frame.Height, Redacted: event.Frame.Redacted}
	}
	return result
}

func (event durableEvent) platformEvent() platform.CaptureEvent {
	result := platform.CaptureEvent{Seq: event.Seq, Kind: event.Kind, Segment: event.Segment}
	if event.Frame != nil {
		result.Frame = &platform.CapturedFrame{SegmentPath: event.Frame.SegmentPath, FrameIndex: event.Frame.FrameIndex, CapturedAt: event.Frame.CapturedAt, IdleSeconds: event.Frame.IdleSeconds, DisplayID: event.Frame.DisplayID, Width: event.Frame.Width, Height: event.Frame.Height, Redacted: event.Frame.Redacted}
	}
	return result
}

func loadState(fileName string) (durableState, error) {
	data, err := os.ReadFile(fileName)
	if errors.Is(err, os.ErrNotExist) {
		return durableState{Version: 1}, nil
	}
	if err != nil {
		return durableState{}, errStateIO
	}
	var state durableState
	if err := json.Unmarshal(data, &state); err != nil {
		return durableState{}, errInvalidState
	}
	if state.Version != 1 || state.Acked > state.MaxSeq {
		return durableState{}, errInvalidState
	}
	previous := state.Acked
	for _, event := range state.Events {
		if event.Seq != previous+1 || event.Seq > state.MaxSeq || !event.Kind.Valid() || (event.Frame == nil) == (event.Segment == nil) {
			return durableState{}, errInvalidState
		}
		if (event.Kind == platform.CaptureEventFrame) != (event.Frame != nil) {
			return durableState{}, errInvalidState
		}
		if event.Frame != nil && !platform.ValidSegmentPath(event.Frame.SegmentPath) {
			return durableState{}, errInvalidState
		}
		if event.Segment != nil && !platform.ValidSegmentPath(event.Segment.SegmentPath) {
			return durableState{}, errInvalidState
		}
		previous = event.Seq
	}
	if previous != state.MaxSeq {
		return durableState{}, errInvalidState
	}
	return state, nil
}

func writeState(fileName string, state durableState) error {
	if err := writeStateFile(fileName, state); err != nil {
		return errStateIO
	}
	return nil
}

func writeStateFile(fileName string, state durableState) error {
	if err := os.MkdirAll(filepath.Dir(fileName), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(fileName), ".daygo-fake-capture-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, fileName); err != nil {
		return err
	}
	remove = false
	return nil
}
