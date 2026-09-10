package platformtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/platform"
)

const (
	timeout = 3 * time.Second
	settle  = 50 * time.Millisecond
)

// Suite exercises behavior shared by every normal-path Capture implementation.
func Suite(t *testing.T, newCapture func(t *testing.T) platform.Capture) {
	t.Helper()
	t.Run("lifecycle configuration and streams", func(t *testing.T) {
		capture := newCapture(t)
		if cap(capture.Events()) < 64 || cap(capture.Status()) < 8 {
			t.Fatalf("channel capacities = events:%d status:%d", cap(capture.Events()), cap(capture.Status()))
		}
		display := "display-contract"
		cfg := config(t, &display)
		equivalent := cfg
		equivalent.BlockedApplicationIDs = []string{"app.a", "app.b"}
		equivalent.PreferredDisplayID = pointer("display-contract")
		mustStart(t, capture, cfg)
		cfg.BlockedApplicationIDs[0] = "mutated"
		display = "mutated"
		if err := capture.Start(context.Background(), equivalent); err != nil {
			t.Fatalf("same canonical config Start: %v", err)
		}
		different := equivalent
		different.CaptureHeight++
		if err := capture.Start(context.Background(), different); err == nil {
			t.Fatal("different config Start succeeded while running")
		}
		frame := receiveEvent(t, capture.Events())
		assertFrame(t, frame, 0)
		if err := capture.Stop(context.Background()); err != nil {
			t.Fatalf("Stop: %v", err)
		}
		closed := receiveKind(t, capture.Events(), platform.CaptureEventSegmentClosed)
		assertClosed(t, closed, frame)
		if err := capture.Stop(context.Background()); err != nil {
			t.Fatalf("idempotent Stop: %v", err)
		}
		mustStart(t, capture, equivalent)
		restarted := receiveKind(t, capture.Events(), platform.CaptureEventFrame)
		assertFrame(t, restarted, 0)
		if restarted.Seq <= closed.Seq {
			t.Fatalf("restarted seq %d <= %d", restarted.Seq, closed.Seq)
		}
		if err := capture.Ack(context.Background(), restarted.Seq); err != nil {
			t.Fatalf("Ack before Close: %v", err)
		}
		if err := capture.Close(context.Background()); err != nil {
			t.Fatalf("Close: %v", err)
		}
		assertClosedChannel(t, capture.Events())
		assertClosedChannel(t, capture.Status())
	})

	t.Run("status merges rather than blocking", func(t *testing.T) {
		capture := newCapture(t)
		bounded := cap(capture.Status())
		mustStart(t, capture, config(t, nil))
		for index := 0; index < bounded*3+4; index++ {
			receiveKind(t, capture.Events(), platform.CaptureEventFrame)
		}
		if err := capture.Stop(context.Background()); err != nil {
			t.Fatalf("Stop after status churn: %v", err)
		}
		if err := capture.Close(context.Background()); err != nil {
			t.Fatalf("Close after status churn: %v", err)
		}
		statuses := drainStatus(t, capture.Status())
		if len(statuses) == 0 || len(statuses) > bounded {
			t.Fatalf("drained %d statuses, want 1..%d", len(statuses), bounded)
		}
		latest := statuses[len(statuses)-1]
		if latest.Phase != platform.CaptureIdle || latest.Permission != platform.PermissionGranted || latest.Fault != nil {
			t.Fatalf("latest status = %#v, want idle and permission granted", latest)
		}
	})

	t.Run("cumulative ack and replay", func(t *testing.T) {
		cfg := config(t, nil)
		first := newCapture(t)
		mustStart(t, first, cfg)
		frame := receiveKind(t, first.Events(), platform.CaptureEventFrame)
		if err := first.Stop(context.Background()); err != nil {
			t.Fatalf("Stop: %v", err)
		}
		segment := receiveKind(t, first.Events(), platform.CaptureEventSegmentClosed)
		if err := first.Ack(context.Background(), frame.Seq); err != nil {
			t.Fatalf("Ack: %v", err)
		}
		if err := first.Close(context.Background()); err != nil {
			t.Fatalf("Close: %v", err)
		}
		second := newCapture(t)
		mustStart(t, second, cfg)
		replayed := receiveEvent(t, second.Events())
		if replayed.Seq != segment.Seq || replayed.Kind != segment.Kind || replayed.Segment == nil || *replayed.Segment != *segment.Segment {
			t.Fatalf("replayed event = %#v, want %#v", replayed, segment)
		}
		fresh := receiveKind(t, second.Events(), platform.CaptureEventFrame)
		if fresh.Seq <= segment.Seq {
			t.Fatalf("fresh seq %d <= historical max %d", fresh.Seq, segment.Seq)
		}
		if err := second.Ack(context.Background(), fresh.Seq); err != nil {
			t.Fatalf("Ack replay capture: %v", err)
		}
		if err := second.Close(context.Background()); err != nil {
			t.Fatalf("Close replay capture: %v", err)
		}
	})

	t.Run("pre-canceled commands have no side effects", func(t *testing.T) {
		capture := newCapture(t)
		cfg := config(t, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := capture.Start(ctx, cfg); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled Start = %v", err)
		}
		select {
		case event := <-capture.Events():
			t.Fatalf("canceled Start emitted %#v", event)
		default:
		}
		mustStart(t, capture, cfg)
		frame := receiveKind(t, capture.Events(), platform.CaptureEventFrame)
		if err := capture.Ack(ctx, frame.Seq); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled Ack = %v", err)
		}
		if err := capture.Stop(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled Stop = %v", err)
		}
		if err := capture.Close(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled Close = %v", err)
		}
		if err := capture.Stop(context.Background()); err != nil {
			t.Fatalf("Stop after canceled commands: %v", err)
		}
		closed := receiveKind(t, capture.Events(), platform.CaptureEventSegmentClosed)
		if err := capture.Ack(context.Background(), closed.Seq); err != nil {
			t.Fatalf("Ack before final Close: %v", err)
		}
		if err := capture.Close(context.Background()); err != nil {
			t.Fatalf("final Close: %v", err)
		}
	})
}

// AuthorizedCapture is a Capture whose screen-recording authorization a test can
// drive directly. The fake implements it. A real adapter learns authorization
// from the OS, so it can only run this suite by hand with the grant withheld.
type AuthorizedCapture interface {
	platform.Capture
	SetPermission(platform.PermissionState)
}

// SuitePermission exercises the authorization path. Without a grant, Start
// configures nothing, emits no frames and reports the real permission; once the
// grant returns, the next Start records normally (docs/04 §4.1.3, MC-1/MC-2).
func SuitePermission(t *testing.T, newCapture func(t *testing.T) AuthorizedCapture) {
	t.Helper()
	t.Run("unauthorized start records nothing and recovers", func(t *testing.T) {
		capture := newCapture(t)
		capture.SetPermission(platform.PermissionNotDetermined)
		cfg := config(t, nil)
		if err := capture.Start(context.Background(), cfg); err != nil {
			t.Fatalf("Start without permission: %v", err)
		}
		awaitStatus(t, capture.Status(), platform.CaptureIdle, platform.PermissionNotDetermined)
		assertNoFrames(t, capture.Events())

		capture.SetPermission(platform.PermissionGranted)
		mustStart(t, capture, cfg)
		frame := receiveKind(t, capture.Events(), platform.CaptureEventFrame)
		assertFrame(t, frame, 0)
		awaitStatus(t, capture.Status(), platform.CaptureCapturing, platform.PermissionGranted)

		if err := capture.Stop(context.Background()); err != nil {
			t.Fatalf("Stop: %v", err)
		}
		awaitStatus(t, capture.Status(), platform.CaptureIdle, platform.PermissionGranted)
		if err := capture.Close(context.Background()); err != nil {
			t.Fatalf("Close: %v", err)
		}
		assertClosedChannel(t, capture.Events())
		assertClosedChannel(t, capture.Status())
	})

	t.Run("losing the grant suspends capture and seq continues", func(t *testing.T) {
		capture := newCapture(t)
		cfg := config(t, nil)
		mustStart(t, capture, cfg)
		frame := receiveKind(t, capture.Events(), platform.CaptureEventFrame)
		assertFrame(t, frame, 0)

		capture.SetPermission(platform.PermissionDenied)
		awaitStatus(t, capture.Status(), platform.CaptureIdle, platform.PermissionDenied)
		assertNoFrames(t, capture.Events())

		capture.SetPermission(platform.PermissionGranted)
		mustStart(t, capture, cfg)
		restarted := receiveKind(t, capture.Events(), platform.CaptureEventFrame)
		if restarted.Seq <= frame.Seq {
			t.Fatalf("restarted seq %d <= %d", restarted.Seq, frame.Seq)
		}
		if err := capture.Stop(context.Background()); err != nil {
			t.Fatalf("Stop: %v", err)
		}
		if err := capture.Close(context.Background()); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
}

func config(t *testing.T, display *string) platform.CaptureConfig {
	t.Helper()
	return platform.CaptureConfig{Interval: 10 * time.Millisecond, CaptureHeight: 720, BlockedApplicationIDs: []string{"app.b", "app.a"}, SegmentDirectory: t.TempDir(), PreferredDisplayID: display, SegmentMaxFrames: 100, SegmentMaxDuration: time.Hour}
}

func mustStart(t *testing.T, capture platform.Capture, cfg platform.CaptureConfig) {
	t.Helper()
	if err := capture.Start(context.Background(), cfg); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

func receiveEvent(t *testing.T, events <-chan platform.CaptureEvent) platform.CaptureEvent {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("events closed early")
		}
		return event
	case <-time.After(timeout):
		t.Fatal("timed out waiting for event")
		return platform.CaptureEvent{}
	}
}

func receiveKind(t *testing.T, events <-chan platform.CaptureEvent, kind platform.CaptureEventKind) platform.CaptureEvent {
	t.Helper()
	for {
		event := receiveEvent(t, events)
		if event.Kind == kind {
			return event
		}
	}
}

func assertFrame(t *testing.T, event platform.CaptureEvent, index int) {
	t.Helper()
	if event.Seq == 0 || event.Kind != platform.CaptureEventFrame || event.Frame == nil || event.Segment != nil || event.Frame.FrameIndex != index || !platform.ValidSegmentPath(event.Frame.SegmentPath) {
		t.Fatalf("invalid frame event: %#v", event)
	}
}

func assertClosed(t *testing.T, event, frame platform.CaptureEvent) {
	t.Helper()
	if event.Seq <= frame.Seq || event.Kind != platform.CaptureEventSegmentClosed || event.Frame != nil || event.Segment == nil || event.Segment.SegmentPath != frame.Frame.SegmentPath || event.Segment.FrameCount < 1 {
		t.Fatalf("invalid segment event: %#v after %#v", event, frame)
	}
}

func assertClosedChannel[T any](t *testing.T, channel <-chan T) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case _, ok := <-channel:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("channel did not close")
		}
	}
}

func drainStatus(t *testing.T, status <-chan platform.CaptureStatus) []platform.CaptureStatus {
	t.Helper()
	var drained []platform.CaptureStatus
	for {
		select {
		case value, ok := <-status:
			if !ok {
				return drained
			}
			drained = append(drained, value)
		case <-time.After(settle):
			return drained
		}
	}
}

// awaitStatus consumes the mergeable status stream until it reports the expected
// current state. Both arguments are asserted, not just waited out: an
// implementation that never reaches the state fails on the deadline.
func awaitStatus(t *testing.T, status <-chan platform.CaptureStatus, phase platform.CapturePhase, permission platform.PermissionState) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case value, ok := <-status:
			if !ok {
				t.Fatal("status channel closed early")
			}
			if value.Phase == phase && value.Permission == permission {
				return
			}
		case <-deadline:
			t.Fatalf("no status reached phase %q permission %q", phase, permission)
		}
	}
}

// assertNoFrames tolerates segment bookkeeping, which a suspension may still
// finalize, but fails on any frame the implementation should not have taken.
func assertNoFrames(t *testing.T, events <-chan platform.CaptureEvent) {
	t.Helper()
	deadline := time.After(settle)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatal("events closed early")
			}
			if event.Kind == platform.CaptureEventFrame {
				t.Fatalf("frame recorded without authorization: %#v", event)
			}
		case <-deadline:
			return
		}
	}
}

func pointer(value string) *string { return &value }
