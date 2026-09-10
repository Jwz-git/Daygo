package app

import (
	"encoding/json"
	"testing"
)

func TestRecordingStateValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state RecordingState
		want  bool
	}{
		{name: "idle", state: RecordingStateIdle, want: true},
		{name: "starting", state: RecordingStateStarting, want: true},
		{name: "capturing", state: RecordingStateCapturing, want: true},
		{name: "paused", state: RecordingStatePaused, want: true},
		{name: "empty", state: "", want: false},
		{name: "unknown", state: "stopped", want: false},
		{name: "wrong case", state: "Idle", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.state.Valid(); got != tt.want {
				t.Fatalf("RecordingState(%q).Valid() = %t, want %t", tt.state, got, tt.want)
			}
		})
	}
}

func TestM1DTOJSONShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name: "day context",
			value: DayContextDTO{
				Day:             "2026-09-10",
				StandupDay:      "2026-09-11",
				DayStartTs:      1789038000,
				DayEndTs:        1789124400,
				NowTs:           1789052400,
				TimeZone:        "Asia/Kathmandu",
				DayBoundaryHour: 4,
			},
			want: `{"day":"2026-09-10","standupDay":"2026-09-11","dayStartTs":1789038000,"dayEndTs":1789124400,"nowTs":1789052400,"timeZone":"Asia/Kathmandu","dayBoundaryHour":4}`,
		},
		{
			name: "capabilities",
			value: CapabilitiesDTO{
				CanWrite:       true,
				IsCaptureOwner: false,
				Features:       []string{"timeline", "settings"},
				AppVersion:     "0.1.0",
				APIRevision:    1,
			},
			want: `{"canWrite":true,"isCaptureOwner":false,"features":["timeline","settings"],"appVersion":"0.1.0","apiRevision":1}`,
		},
		{
			name: "recording state with explicit nulls",
			value: RecordingStateDTO{
				State:          RecordingStatePaused,
				Reason:         nil,
				UserPaused:     true,
				PauseEndsAtTs:  nil,
				Permission:     "not_determined",
				IsCaptureOwner: false,
			},
			want: `{"state":"paused","reason":null,"userPaused":true,"pauseEndsAtTs":null,"permission":"not_determined","isCaptureOwner":false,"activeDisplayId":null,"lastFrameAtTs":null}`,
		},
		{
			name: "permission",
			value: PermissionDTO{
				ScreenRecording: "granted",
				Notifications:   "denied",
				CanRequest:      false,
			},
			want: `{"screenRecording":"granted","notifications":"denied","canRequest":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatalf("marshal DTO: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("JSON shape mismatch\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestRecordingStateJSONValues(t *testing.T) {
	t.Parallel()

	states := []RecordingState{
		RecordingStateIdle,
		RecordingStateStarting,
		RecordingStateCapturing,
		RecordingStatePaused,
	}
	const want = `["idle","starting","capturing","paused"]`

	got, err := json.Marshal(states)
	if err != nil {
		t.Fatalf("marshal recording states: %v", err)
	}
	if string(got) != want {
		t.Fatalf("recording state JSON mismatch\n got: %s\nwant: %s", got, want)
	}
}
