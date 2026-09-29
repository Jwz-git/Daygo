package app

import "sync"

type UIVisibilityDTO struct {
	Visible bool `json:"visible"`
}

// Native app hiding and Wails window order-out are independent. An app-unhide
// must not accidentally resume an ordered-out soft-quit window.
type uiVisibilityState struct {
	mu           sync.Mutex
	appHidden    bool
	windowHidden bool
}

func (b *Backend) GetUIVisibility() UIVisibilityDTO {
	b.uiVisibility.mu.Lock()
	defer b.uiVisibility.mu.Unlock()
	return UIVisibilityDTO{Visible: !b.uiVisibility.appHidden && !b.uiVisibility.windowHidden}
}
func (b *Backend) setApplicationHidden(hidden bool) { b.setUIHidden(true, hidden) }
func (b *Backend) setWindowHidden(hidden bool)      { b.setUIHidden(false, hidden) }
func (b *Backend) setUIHidden(application, hidden bool) {
	state := &b.uiVisibility
	state.mu.Lock()
	defer state.mu.Unlock()
	before := !state.appHidden && !state.windowHidden
	if application {
		state.appHidden = hidden
	} else {
		state.windowHidden = hidden
	}
	visible := !state.appHidden && !state.windowHidden
	if before != visible {
		// Serialize publication as well as state; opposite transitions cannot be
		// emitted out of order by the platform pump and the Wails lifecycle hook.
		b.emitter.Emit(EventUIVisibilityChanged, UIVisibilityDTO{Visible: visible})
	}
}
