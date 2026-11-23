package app

import "time"

// Only numeric playback state and an optional ISO day can cross a window
// release. No editor text, media bytes, secrets or route URLs are retained.
type WindowPlayerDTO struct {
	Progress float64 `json:"progress"`
	Rate     int     `json:"rate"`
	Started  bool    `json:"started"`
	Playing  bool    `json:"playing"`
	Expanded bool    `json:"expanded"`
}
type WindowResumeDTO struct {
	Route  string           `json:"route"`
	Day    string           `json:"day"`
	CardID int64            `json:"cardId"`
	Player *WindowPlayerDTO `json:"player"`
}

func (s WindowResumeDTO) valid() bool {
	if s.Route != "timeline" && s.Route != "daily" && s.Route != "weekly" {
		return false
	}
	if s.Route != "timeline" && (s.CardID != 0 || s.Player != nil) {
		return false
	}
	if s.Day != "" {
		if _, err := time.Parse(time.DateOnly, s.Day); err != nil {
			return false
		}
	}
	if s.CardID < 0 || s.CardID > 1<<53-1 {
		return false
	}
	p := s.Player
	return p == nil || (s.CardID > 0 && p.Progress >= 0 && p.Progress <= 100 &&
		(p.Rate == 20 || p.Rate == 40 || p.Rate == 60 || p.Rate == 120) && (!p.Playing || p.Started))
}

// windowRelease is owned by the desktop main thread. A fresh token binds the
// frontend's positive acknowledgement to one red-button background interval.
type windowRelease struct {
	sequence  int
	active    int
	requested bool
}

func (r *windowRelease) begin() int {
	r.sequence++
	r.active = r.sequence
	r.requested = false
	return r.active
}
func (r *windowRelease) cancel() { r.active = 0; r.requested = false }
func (r *windowRelease) request(token int) bool {
	if token == 0 || token != r.active {
		return false
	}
	r.requested = true
	return true
}
func (r *windowRelease) accept(token int, s WindowResumeDTO) bool {
	if !r.requested || token != r.active || !s.valid() {
		return false
	}
	r.cancel()
	return true
}

// ConfirmWindowRelease is ignored by hosts without a pending release.
func (b *Backend) ConfirmWindowRelease(token int, state WindowResumeDTO) {
	if b.confirmWindowRelease != nil {
		b.confirmWindowRelease(token, state)
	}
}
func (b *Backend) TakeWindowResumeState() *WindowResumeDTO {
	return b.windowResume.Swap(nil)
}
