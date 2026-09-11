package app

import "github.com/Jwz-git/Daygo/internal/platform"

type SystemEventTestDTO struct {
	Kind string `json:"kind"`
	AtTs int64  `json:"atTs"`
}

// PollSystemEvents drains the app-owned broadcast buffer. The recorder and the
// test page must both observe every native event; consuming System.Events()
// directly would race and lose events for one consumer.
func (b *Backend) PollSystemEvents() []SystemEventTestDTO {
	b.systemEventMu.Lock()
	events := append([]platform.SystemEvent(nil), b.systemEventBuffer...)
	b.systemEventBuffer = nil
	b.systemEventMu.Unlock()
	result := make([]SystemEventTestDTO, 0, len(events))
	for _, event := range events {
		result = append(result, SystemEventTestDTO{Kind: string(event.Kind), AtTs: event.At.Unix()})
	}
	return result
}
