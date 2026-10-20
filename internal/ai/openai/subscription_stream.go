package openai

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/Jwz-git/Daygo/internal/ai"
)

type subscriptionEvent struct {
	Type         string          `json:"type"`
	Response     json.RawMessage `json:"response"`
	Item         json.RawMessage `json:"item"`
	OutputIndex  *int            `json:"output_index"`
	ContentIndex *int            `json:"content_index"`
	Delta        string          `json:"delta"`
	Text         string          `json:"text"`
}

type subscriptionText struct {
	deltas strings.Builder
	text   string
	done   bool
}

// Output is indexed, not concatenated in event-arrival order. A done event
// replaces its deltas; a full completed output takes precedence over both.
// Nothing is returned to the caller until response.completed is validated.
type subscriptionStreamOutput struct {
	items   map[int]json.RawMessage
	texts   map[int]map[int]*subscriptionText
	refused bool
}

func invalidSubscriptionOutput() error {
	return ai.NewError(ai.ErrorInvalidOutput, "invalid subscription output event", 0, nil)
}

func (s *subscriptionStreamOutput) add(e subscriptionEvent) error {
	if e.Type == "response.refusal.delta" || e.Type == "response.refusal.done" {
		s.refused = true
		return nil
	}
	if e.OutputIndex == nil || *e.OutputIndex < 0 {
		return invalidSubscriptionOutput()
	}
	index := *e.OutputIndex
	if e.Type == "response.output_item.done" {
		var item struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(e.Item, &item) != nil || item.Type == "" {
			return invalidSubscriptionOutput()
		}
		if s.items == nil {
			s.items = make(map[int]json.RawMessage)
		}
		s.items[index] = e.Item
		return nil
	}
	if e.ContentIndex == nil || *e.ContentIndex < 0 {
		return invalidSubscriptionOutput()
	}
	if s.texts == nil {
		s.texts = make(map[int]map[int]*subscriptionText)
	}
	if s.texts[index] == nil {
		s.texts[index] = make(map[int]*subscriptionText)
	}
	part := s.texts[index][*e.ContentIndex]
	if part == nil {
		part = &subscriptionText{}
		s.texts[index][*e.ContentIndex] = part
	}
	if e.Type == "response.output_text.done" {
		part.text, part.done = e.Text, true
		part.deltas.Reset()
	} else if !part.done {
		part.deltas.WriteString(e.Delta)
	}
	return nil
}

func (s *subscriptionStreamOutput) complete(body json.RawMessage) (json.RawMessage, error) {
	if s.refused {
		return nil, ai.NewError(ai.ErrorInvalidOutput, "provider refused the request", 0, nil)
	}
	var response struct {
		Output []json.RawMessage `json:"output"`
	}
	if json.Unmarshal(body, &response) != nil {
		return nil, invalidSubscriptionOutput()
	}
	if len(response.Output) != 0 {
		return body, nil
	}
	indices := make(map[int]bool)
	for index := range s.items {
		indices[index] = true
	}
	for index := range s.texts {
		indices[index] = true
	}
	for _, index := range sortedStreamIndices(indices) {
		if item, ok := s.items[index]; ok {
			response.Output = append(response.Output, item)
			continue
		}
		parts := s.texts[index]
		content := make([]map[string]string, 0, len(parts))
		for _, contentIndex := range sortedStreamIndices(parts) {
			part := parts[contentIndex]
			text := part.text
			if !part.done {
				text = part.deltas.String()
			}
			content = append(content, map[string]string{"type": "output_text", "text": text})
		}
		item, _ := json.Marshal(map[string]any{"type": "message", "status": "completed", "content": content})
		response.Output = append(response.Output, item)
	}
	var payload map[string]json.RawMessage
	_ = json.Unmarshal(body, &payload)
	payload["output"], _ = json.Marshal(response.Output)
	return json.Marshal(payload)
}

func sortedStreamIndices[T any](m map[int]T) []int {
	indices := make([]int, 0, len(m))
	for index := range m {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	return indices
}
