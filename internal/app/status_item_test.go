package app

import (
	"errors"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
	"github.com/Jwz-git/Daygo/internal/recorder"
)

func TestStatusItemPresentationFixtures(t *testing.T) {
	labels := defaultStatusItemLabels()
	until := time.Date(2026, 9, 26, 15, 30, 0, 0, time.FixedZone("fixture", 8*3600))
	tests := []struct {
		name               string
		view               statusItemView
		title              string
		icon               platform.StatusItemIcon
		primary, durations bool
	}{
		{"idle", statusItemView{state: recorder.StateIdle, owner: true, available: true}, labels.TitleIdle, platform.StatusIconInactive, true, false},
		{"starting", statusItemView{state: recorder.StateStarting, owner: true, available: true}, labels.TitleStarting, platform.StatusIconBusy, false, false},
		{"recording", statusItemView{state: recorder.StateCapturing, owner: true, available: true}, labels.TitleRecording, platform.StatusIconActive, false, true},
		{"read only", statusItemView{state: recorder.StateIdle, available: true}, labels.TitleReadOnly, platform.StatusIconInactive, false, false},
		{"unavailable", statusItemView{state: recorder.StateIdle, owner: true}, labels.TitleUnavailable, platform.StatusIconWarning, false, false},
		{"failure", statusItemView{state: recorder.StateCapturing, owner: true, available: true, failed: true}, labels.TitleError, platform.StatusIconWarning, false, true},
		{"system hold", statusItemView{state: recorder.StatePaused, owner: true, available: true, systemBlocked: true}, labels.TitleSystemPaused, platform.StatusIconPaused, false, false},
		{"timed pause", statusItemView{state: recorder.StatePaused, owner: true, available: true, userPaused: true, until: &until}, "已暂停 · 15:30 恢复", platform.StatusIconPaused, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := presentStatusItem(tt.view, labels)
			if got.Title != tt.title || got.Icon != tt.icon || got.PrimaryActionEnabled != tt.primary || got.PauseDurationsEnabled != tt.durations {
				t.Fatalf("got=%+v", got)
			}
		})
	}
}

func TestNativeActionErrorsUseSanitizedLocalizedCopy(t *testing.T) {
	labels := defaultStatusItemLabels()
	for _, tt := range []struct {
		err  error
		want string
	}{
		{apperr.E(apperr.NotCaptureOwner, "private path", nil), labels.ErrorOwner},
		{apperr.E(apperr.PermissionDenied, "private screen", nil), labels.ErrorPermission},
		{apperr.E(apperr.NativeUnavailable, "private key", nil), labels.ErrorUnavailable},
		{errors.New("private payload"), labels.ErrorFailed},
	} {
		if got := nativeActionErrorMessage(tt.err, labels); got != tt.want {
			t.Fatalf("message=%q want=%q", got, tt.want)
		}
	}
}

func TestStatusLabelsRejectIncompleteOrAmbiguousQuitCopy(t *testing.T) {
	b := NewBackend(nil, nil)
	want := defaultStatusItemLabels()
	for _, bad := range []StatusItemLabelsDTO{{}, func() StatusItemLabelsDTO { value := want; value.QuitAnyway = value.KeepOpen; return value }()} {
		if err := b.SetStatusItemLabels(bad); err == nil {
			t.Fatal("unsafe label bundle was accepted")
		}
		if b.statusLabels.get() != want {
			t.Fatal("invalid bundle replaced safe copy")
		}
	}
}

func TestMenuRecordingActionReturnsFailure(t *testing.T) {
	b := newBackend(fixedClock{}, nil, nil, false, false)
	err := b.runStatusRecordingAction("toggle_pause")
	var public *apperr.Error
	if !errors.As(err, &public) || public.Code != apperr.NotCaptureOwner {
		t.Fatalf("error=%v", err)
	}
}

func TestStatusPresentationReadsCurrentStateAfterStaleEvent(t *testing.T) {
	store := openTestStore(t, t.TempDir(), true)
	b := NewBackend(fake.NewSystem(), store)
	b.setCapture(fake.NewCapture())
	if _, err := b.ensureRecorder(); err != nil {
		t.Fatal(err)
	}
	if got := b.statusItemPresentation(recorder.StateCapturing); got.Title != defaultStatusItemLabels().TitleIdle {
		t.Fatalf("stale event changed surface=%+v", got)
	}
}

func TestRecordingDTOReportsTimedPauseAndClearsItOnStop(t *testing.T) {
	store := openTestStore(t, t.TempDir(), true)
	b := NewBackend(fake.NewSystem(), store)
	b.setCapture(fake.NewCapture())
	if err := b.SetRecording(true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.stopRecorder() })
	deadline := time.Now().Add(time.Second)
	for b.recorderState() != recorder.StateCapturing && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	before := time.Now().Add(15 * time.Minute).Unix()
	if err := b.PauseRecording(15); err != nil {
		t.Fatal(err)
	}
	dto, err := b.GetRecordingState()
	if err != nil {
		t.Fatal(err)
	}
	if !dto.UserPaused || dto.PauseEndsAtTs == nil || *dto.PauseEndsAtTs < before || *dto.PauseEndsAtTs > before+2 {
		t.Fatalf("pause DTO=%+v", dto)
	}
	if err := b.SetRecording(false); err != nil {
		t.Fatal(err)
	}
	dto, err = b.GetRecordingState()
	if err != nil || dto.UserPaused || dto.PauseEndsAtTs != nil {
		t.Fatalf("stopped DTO=%+v error=%v", dto, err)
	}
}
