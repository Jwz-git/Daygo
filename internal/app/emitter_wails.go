package app

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// wailsEmitter publishes events through the Wails runtime.
//
// It holds the context Wails hands to OnStartup, because runtime.EventsEmit
// requires one. Keeping the Wails type here means the binding methods stay free
// of Wails imports: only this file and app.go know Wails exists (docs/02 §2.1).
type wailsEmitter struct {
	mu  sync.RWMutex
	ctx context.Context
}

// NewWailsEmitter builds an emitter whose context is supplied later, at
// OnStartup. Until then Emit is a no-op: an event emitted before the window
// exists has no listener, so dropping it is correct rather than an error.
func NewWailsEmitter() *wailsEmitter {
	return &wailsEmitter{}
}

// SetContext installs the Wails context from OnStartup.
//
// The argument must be the context Wails passes to a lifecycle hook. Wails
// treats any other context as a programming error and terminates the process
// rather than returning an error, so this must never be called with a context
// this package created.
func (e *wailsEmitter) SetContext(ctx context.Context) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ctx = ctx
}

// Emit publishes an event, dropping it when the context is not ready.
func (e *wailsEmitter) Emit(name EventName, payload any) {
	if e == nil {
		return
	}
	e.mu.RLock()
	ctx := e.ctx
	e.mu.RUnlock()
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, string(name), payload)
}
