package app

import (
	"sync"

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
	Open            string `json:"open"`
	Recordings      string `json:"recordings"`
	Quit            string `json:"quit"`
	PauseMenu       string `json:"pauseMenu"`
	Pause15         string `json:"pause15"`
	Pause30         string `json:"pause30"`
	Pause60         string `json:"pause60"`
	PauseIndefinite string `json:"pauseIndefinite"`
	Start           string `json:"start"`
	Resume          string `json:"resume"`
	Tooltip         string `json:"tooltip"`
	TitleRecording  string `json:"titleRecording"`
	TitlePaused     string `json:"titlePaused"`
	TitleIdle       string `json:"titleIdle"`
}

// defaultStatusItemLabels seeds the menu bar before the frontend pushes a
// bundle — the item is created during OnStartup, ahead of the first webview
// paint. zh-CN is the default interface language (settings.DefaultLanguage), so
// the seed matches what most users would otherwise wait for.
func defaultStatusItemLabels() StatusItemLabelsDTO {
	return StatusItemLabelsDTO{
		Open:            "打开 Daygo",
		Recordings:      "打开录制文件夹",
		Quit:            "退出 Daygo",
		PauseMenu:       "暂停录制",
		Pause15:         "15 分钟",
		Pause30:         "30 分钟",
		Pause60:         "1 小时",
		PauseIndefinite: "一直暂停",
		Start:           "开始录制",
		Resume:          "恢复录制",
		Tooltip:         "Daygo",
		TitleRecording:  "正在录制",
		TitlePaused:     "已暂停",
		TitleIdle:       "未在录制",
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
// no-op, so a read-only second instance simply keeps the bundle for later.
func (b *Backend) SetStatusItemLabels(labels StatusItemLabelsDTO) error {
	b.statusLabels.set(labels)
	b.updateStatus(b.recorderState())
	return nil
}

// statusItemState maps a recorder state onto the menu-bar surface using the
// current label bundle. Capturing shows the pause-duration submenu; every other
// state shows a single primary action (start when idle, resume when paused,
// disabled while starting).
func statusItemState(state recorder.State, labels StatusItemLabelsDTO) platform.StatusItemState {
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
	switch state {
	case recorder.StateCapturing:
		item.Title = labels.TitleRecording
		item.PauseDurationsEnabled = true
	case recorder.StateStarting:
		item.Title = labels.TitleRecording
		item.PrimaryActionLabel = labels.Start
		item.PrimaryActionEnabled = false
	case recorder.StatePaused:
		item.Title = labels.TitlePaused
		item.PrimaryActionLabel = labels.Resume
		item.PrimaryActionEnabled = true
	default:
		item.Title = labels.TitleIdle
		item.PrimaryActionLabel = labels.Start
		item.PrimaryActionEnabled = true
	}
	return item
}
