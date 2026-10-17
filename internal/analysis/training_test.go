package analysis

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Jwz-git/Daygo/internal/domain"
)

func TestTrainingLabelRejectsSilentFrameClampAndUnknownCategory(t *testing.T) {
	sample := domain.TrainingSample{Purpose: "transcribe", Parts: []domain.TrainingPart{{Image: []byte("anonymous")}}}
	label := `{"observations":[{"from_frame":0,"to_frame":0,"observation":"anonymous","apps":[]}]}`
	if err := ValidateTrainingLabel(sample, label); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTrainingLabel(sample, `{"observations":[{"from_frame":0,"to_frame":4,"observation":"anonymous","apps":[]}]}`); err == nil {
		t.Fatal("clamping must not approve a label")
	}
	from := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	sample = domain.TrainingSample{Purpose: "cards", Context: *trainingContext(from, from.Add(15*time.Minute), time.UTC, "en", cardModeFresh, []domain.Category{{Name: "Work"}})}
	card := map[string]any{"start": "10:00 AM", "end": "10:15 AM", "category": "Work", "subcategory": "", "title": "Anonymous", "summary": "Anonymous", "detailed_summary": "Anonymous", "appSites": []string{}, "distractions": []any{}, "activityPoints": []any{}}
	raw, _ := json.Marshal(map[string]any{"cards": []any{card}})
	if err := ValidateTrainingLabel(sample, string(raw)); err != nil {
		t.Fatal(err)
	}
	card["category"] = "invented"
	raw, _ = json.Marshal(map[string]any{"cards": []any{card}})
	if ValidateTrainingLabel(sample, string(raw)) == nil {
		t.Fatal("unknown category")
	}
	card["category"] = "Work"
	card["end"] = "9:00 AM"
	raw, _ = json.Marshal(map[string]any{"cards": []any{card}})
	if ValidateTrainingLabel(sample, string(raw)) == nil {
		t.Fatal("invalid time")
	}
}
