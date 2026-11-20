package app

import "sync"

type wailsEmitter struct {
	mu   sync.RWMutex
	emit func(string, any)
}

func NewWailsEmitter() *wailsEmitter { return &wailsEmitter{} }

func (e *wailsEmitter) setPublisher(publish func(string, any)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.emit = publish
}

func (e *wailsEmitter) Emit(name EventName, payload any) {
	if e == nil {
		return
	}
	e.mu.RLock()
	publish := e.emit
	e.mu.RUnlock()
	if publish != nil {
		publish(string(name), payload)
	}
}
