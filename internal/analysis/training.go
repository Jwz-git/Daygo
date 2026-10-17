package analysis

import (
	"encoding/json"
	"errors"
	"time"
	_ "time/tzdata" // Saved sample zones must also resolve on hosts without a zone database.

	"github.com/Jwz-git/Daygo/internal/ai"
	"github.com/Jwz-git/Daygo/internal/domain"
	"github.com/Jwz-git/Daygo/internal/timeutil"
)

func trainingContext(from, to time.Time, loc *time.Location, language string, mode cardMode, categories []domain.Category) *domain.TrainingContext {
	names := []string{}
	for _, c := range categories {
		if !c.IsSystem {
			names = append(names, c.Name)
		}
	}
	name := "fresh"
	if mode == cardModeOngoing {
		name = "ongoing"
	}
	if mode == cardModeScoped {
		name = "scoped"
	}
	return &domain.TrainingContext{Start: from.Unix(), End: to.Unix(), Zone: timeutil.ZoneName(loc), Language: language, Mode: name, Categories: names}
}

// ValidateTrainingLabel shares production constraints without clamping frame
// indices or dropping invalid cards. Errors never include labels.
func ValidateTrainingLabel(sample domain.TrainingSample, label string) error {
	invalid := errors.New("training label violates analysis contract")
	if sample.Purpose == string(ai.PurposeTranscribe) {
		if ai.ValidateJSON(json.RawMessage(label), transcribeOutput) != nil {
			return invalid
		}
		var env transcribeEnvelope
		if json.Unmarshal([]byte(label), &env) != nil {
			return invalid
		}
		n := 0
		for _, p := range sample.Parts {
			if len(p.Image) > 0 {
				n++
			}
		}
		for _, o := range env.Observations {
			if o.FromFrame < 0 || o.ToFrame < o.FromFrame || o.ToFrame >= n {
				return invalid
			}
		}
		return nil
	}
	if sample.Purpose != string(ai.PurposeCards) || ai.ValidateJSON(json.RawMessage(label), cardsOutput) != nil {
		return invalid
	}
	loc, err := time.LoadLocation(sample.Context.Zone)
	if err != nil {
		return invalid
	}
	var env cardsEnvelope
	if json.Unmarshal([]byte(label), &env) != nil {
		return invalid
	}
	known := map[string]bool{}
	for _, name := range sample.Context.Categories {
		known[name] = true
	}
	for _, c := range env.Cards {
		if !known[c.Category] {
			return invalid
		}
	}
	from, to := time.Unix(sample.Context.Start, 0), time.Unix(sample.Context.End, 0)
	spans, issues := resolveCardSpans(shellsFromModel(env.Cards, known), from, to, loc)
	if len(issues) > 0 {
		return invalid
	}
	if sample.Context.Mode == "scoped" {
		issues = validateScopedCards(spans, from, to)
	} else {
		issues = validateCards(spans, from, to, sample.Context.Mode == "fresh")
	}
	if len(issues) > 0 {
		return invalid
	}
	return nil
}
