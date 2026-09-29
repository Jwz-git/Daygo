package app

import (
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/recorder"
)

// StatusItemLabelsDTO is the full set of localized strings the menu-bar item
// renders. The native status item is drawn outside the webview, so vue-i18n
// cannot reach it directly: the frontend pushes the translated bundle here
// (docs/05 §5.5.1) and the app layer forwards it to the platform adapter. The
// state → copy mapping stays in Go so the adapter never learns the recorder
// states.
type StatusItemLabelsDTO struct {
	Open              string `json:"open"`
	Recordings        string `json:"recordings"`
	Quit              string `json:"quit"`
	PauseMenu         string `json:"pauseMenu"`
	Pause15           string `json:"pause15"`
	Pause30           string `json:"pause30"`
	Pause60           string `json:"pause60"`
	PauseIndefinite   string `json:"pauseIndefinite"`
	Start             string `json:"start"`
	Resume            string `json:"resume"`
	Tooltip           string `json:"tooltip"`
	TitleRecording    string `json:"titleRecording"`
	TitlePaused       string `json:"titlePaused"`
	TitleIdle         string `json:"titleIdle"`
	TitleStarting     string `json:"titleStarting"`
	TitleReadOnly     string `json:"titleReadOnly"`
	TitleUnavailable  string `json:"titleUnavailable"`
	TitleError        string `json:"titleError"`
	TitleSystemPaused string `json:"titleSystemPaused"`
	PausedUntil       string `json:"pausedUntil"`
	ActionFailedTitle string `json:"actionFailedTitle"`
	ErrorOwner        string `json:"errorOwner"`
	ErrorPermission   string `json:"errorPermission"`
	ErrorUnavailable  string `json:"errorUnavailable"`
	ErrorFailed       string `json:"errorFailed"`
	ErrorDock         string `json:"errorDock"`
	QuitFailed        string `json:"quitFailed"`
	KeepOpen          string `json:"keepOpen"`
	QuitAnyway        string `json:"quitAnyway"`
	OK                string `json:"ok"`
}

// defaultStatusItemLabels seeds the menu bar before the frontend pushes a
// bundle — the item is created during OnStartup, ahead of the first webview
// paint. zh-CN is the default interface language (settings.DefaultLanguage), so
// the seed matches what most users would otherwise wait for.
func defaultStatusItemLabels() StatusItemLabelsDTO {
	return StatusItemLabelsDTO{
		Open:              "打开 Daygo",
		Recordings:        "打开录制文件夹",
		Quit:              "停止录制并退出 Daygo",
		PauseMenu:         "暂停录制",
		Pause15:           "15 分钟",
		Pause30:           "30 分钟",
		Pause60:           "1 小时",
		PauseIndefinite:   "一直暂停",
		Start:             "开始录制",
		Resume:            "恢复录制",
		Tooltip:           "Daygo",
		TitleRecording:    "正在录制",
		TitlePaused:       "已暂停",
		TitleIdle:         "未在录制",
		TitleStarting:     "正在启动录制",
		TitleReadOnly:     "只读实例 · 录制由其他实例控制",
		TitleUnavailable:  "录制服务不可用",
		TitleError:        "截图失败 · 正在重试",
		TitleSystemPaused: "系统暂停 · 等待唤醒或解锁",
		PausedUntil:       "已暂停 · {time} 恢复",
		ActionFailedTitle: "Daygo 操作未完成",
		ErrorOwner:        "请在负责录制的 Daygo 实例中操作。",
		ErrorPermission:   "请先授予屏幕录制权限，再重启 Daygo。",
		ErrorUnavailable:  "录制服务暂不可用，请检查应用中的设置与诊断。",
		ErrorFailed:       "操作未能完成，请检查当前录制状态后重试。",
		ErrorDock:         "Dock 显示设置已保存，但暂时无法应用。请从菜单栏重新打开 Daygo 后重试。",
		QuitFailed:        "当前录制分段未能安全收尾。仍然退出可能丢失当前分段；保留应用可以检查磁盘空间与诊断。",
		KeepOpen:          "保留应用",
		QuitAnyway:        "仍然退出",
		OK:                "知道了",
	}
}

type statusItemLabelStore struct {
	mu     sync.RWMutex
	labels StatusItemLabelsDTO
}

func (s *statusItemLabelStore) get() StatusItemLabelsDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.labels
}

func (s *statusItemLabelStore) set(labels StatusItemLabelsDTO) {
	s.mu.Lock()
	s.labels = labels
	s.mu.Unlock()
}

// SetStatusItemLabels stores the localized menu-bar strings and repaints the
// status item so a language change takes effect immediately. It is safe to call
// on any instance: when the platform adapter is unavailable the repaint is a
// no-op. A read-only instance with an adapter renders disabled controls.
func (b *Backend) SetStatusItemLabels(labels StatusItemLabelsDTO) error {
	values := []string{labels.Open, labels.Recordings, labels.Quit, labels.PauseMenu, labels.Pause15, labels.Pause30, labels.Pause60, labels.PauseIndefinite, labels.Start, labels.Resume, labels.Tooltip, labels.TitleRecording, labels.TitlePaused, labels.TitleIdle, labels.TitleStarting, labels.TitleReadOnly, labels.TitleUnavailable, labels.TitleError, labels.TitleSystemPaused, labels.PausedUntil, labels.ActionFailedTitle, labels.ErrorOwner, labels.ErrorPermission, labels.ErrorUnavailable, labels.ErrorFailed, labels.ErrorDock, labels.QuitFailed, labels.KeepOpen, labels.QuitAnyway, labels.OK}
	for _, value := range values {
		if strings.TrimSpace(value) == "" || len(value) > 4096 || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
			return apperr.E(apperr.InvalidArgument, "status item labels must be complete and bounded", nil)
		}
	}
	if labels.KeepOpen == labels.QuitAnyway {
		return apperr.E(apperr.InvalidArgument, "quit choices must be distinct", nil)
	}
	b.statusLabels.set(labels)
	b.updateStatus(b.recorderState())
	return nil
}

// statusItemView combines recorder state, ownership and pause metadata. The
// app maps it to copy and controls without exposing business state to native UI.
type statusItemView struct {
	state                                               recorder.State
	owner, available, failed, userPaused, systemBlocked bool
	until                                               *time.Time
}

func (b *Backend) statusItemPresentation(state recorder.State) platform.StatusItemState {
	_, owner := b.instanceOwnership()
	view := statusItemView{state: state, owner: owner, available: b.capture != nil && b.storage != nil}
	b.recorderMu.Lock()
	r := b.recorder
	b.recorderMu.Unlock()
	if r != nil {
		view.state = r.State()
		view.failed = r.LastError() != nil
		pause := r.PauseInfo()
		view.userPaused, view.systemBlocked, view.until = pause.UserPaused, pause.SystemBlocked, pause.Until
	}
	return presentStatusItem(view, b.statusLabels.get())
}

func presentStatusItem(view statusItemView, labels StatusItemLabelsDTO) platform.StatusItemState {
	item := platform.StatusItemState{
		Visible:              true,
		Tooltip:              labels.Tooltip,
		OpenLabel:            labels.Open,
		RecordingsLabel:      labels.Recordings,
		QuitLabel:            labels.Quit,
		PauseMenuLabel:       labels.PauseMenu,
		Pause15Label:         labels.Pause15,
		Pause30Label:         labels.Pause30,
		Pause60Label:         labels.Pause60,
		PauseIndefiniteLabel: labels.PauseIndefinite,
	}
	switch view.state {
	case recorder.StateCapturing:
		item.Title = labels.TitleRecording
		item.Icon = platform.StatusIconActive
		item.PauseDurationsEnabled = true
	case recorder.StateStarting:
		item.Title = labels.TitleStarting
		item.Icon = platform.StatusIconBusy
		item.PrimaryActionLabel = labels.Start
		item.PrimaryActionEnabled = false
	case recorder.StatePaused:
		item.Title = labels.TitlePaused
		item.Icon = platform.StatusIconPaused
		item.PrimaryActionLabel = labels.Resume
		item.PrimaryActionEnabled = !view.systemBlocked
		if view.systemBlocked {
			item.Title = labels.TitleSystemPaused
		}
		if view.userPaused && view.until != nil {
			item.Title = strings.ReplaceAll(labels.PausedUntil, "{time}", view.until.Format("15:04"))
		}
	default:
		item.Title = labels.TitleIdle
		item.PrimaryActionLabel = labels.Start
		item.PrimaryActionEnabled = true
	}
	if view.failed && view.state == recorder.StateCapturing {
		item.Title = labels.TitleError
		item.Icon = platform.StatusIconWarning
	}
	if !view.available {
		item.Title = labels.TitleUnavailable
		item.Icon = platform.StatusIconWarning
		item.PrimaryActionEnabled = false
		item.PauseDurationsEnabled = false
	}
	if !view.owner && view.available {
		item.Title = labels.TitleReadOnly
		item.Icon = platform.StatusIconInactive
		item.PrimaryActionEnabled = false
		item.PauseDurationsEnabled = false
	}
	return item
}

func nativeActionErrorMessage(err error, labels StatusItemLabelsDTO) string {
	var public *apperr.Error
	if errors.As(err, &public) {
		switch public.Code {
		case apperr.NotCaptureOwner:
			return labels.ErrorOwner
		case apperr.PermissionDenied:
			return labels.ErrorPermission
		case apperr.NativeUnavailable, apperr.DatabaseError:
			return labels.ErrorUnavailable
		}
	}
	return labels.ErrorFailed
}

func (b *Backend) runStatusRecordingAction(action string) error {
	switch action {
	case "toggle_pause":
		switch b.recorderState() {
		case recorder.StateIdle:
			return b.SetRecording(true)
		case recorder.StatePaused:
			return b.ResumeRecording()
		case recorder.StateCapturing:
			return b.PauseRecording(0)
		default:
			return apperr.E(apperr.Conflict, "recording transition is in progress", nil)
		}
	case "pause_15":
		return b.PauseRecording(15)
	case "pause_30":
		return b.PauseRecording(30)
	case "pause_60":
		return b.PauseRecording(60)
	case "pause_indefinite":
		return b.PauseRecording(0)
	default:
		return apperr.E(apperr.InvalidArgument, "unknown recording action", nil)
	}
}
