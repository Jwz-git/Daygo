package app

import (
	"testing"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
)

func TestApplyCategoryPresetAndPaletteBindings(t *testing.T) {
	backend, _ := writerBackendWithStore(t, t.TempDir())

	if err := backend.ApplyCategoryPreset([]CategoryPresetDTO{
		{Name: "Coding", ColorHex: "#6366F1", Details: "code", Archetype: "forge"},
		{Name: "communication", ColorHex: "#06B6D4", Details: "talk", Archetype: "nexus"},
	}); err != nil {
		t.Fatalf("ApplyCategoryPreset: %v", err)
	}
	day, err := backend.GetTimelineDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetTimelineDay: %v", err)
	}
	byName := map[string]CategoryDTO{}
	for _, c := range day.Categories {
		byName[c.Name] = c
	}
	// Nothing referenced the starter set, so replace leaves the preset plus
	// the built-ins; the matched row keeps its stored name.
	if len(byName) != 4 || byName["Coding"].Archetype != "forge" ||
		byName["Communication"].Archetype != "nexus" || byName["Communication"].Details != "talk" ||
		byName["System"].Archetype != "order" || byName["Idle"].Archetype != "camp" {
		t.Fatalf("categories after preset = %+v", day.Categories)
	}

	if err := backend.ApplyCategoryPalette([]string{"#4A7C59"}); err != nil {
		t.Fatalf("ApplyCategoryPalette: %v", err)
	}
	day, err = backend.GetTimelineDay("2026-09-12")
	if err != nil {
		t.Fatalf("GetTimelineDay: %v", err)
	}
	for _, c := range day.Categories {
		if !c.IsSystem && c.ColorHex != "#4A7C59" {
			t.Fatalf("%q color = %q after palette", c.Name, c.ColorHex)
		}
	}

	// SaveCategories carries the archetype both ways and rejects unknown ones.
	next := []CategoryDTO{}
	for _, c := range day.Categories {
		if !c.IsSystem {
			c.Archetype = "sanctum"
			next = append(next, c)
		}
	}
	if err := backend.SaveCategories(next); err != nil {
		t.Fatalf("SaveCategories: %v", err)
	}
	day, _ = backend.GetTimelineDay("2026-09-12")
	for _, c := range day.Categories {
		if !c.IsSystem && c.Archetype != "sanctum" {
			t.Fatalf("%q archetype = %q after save", c.Name, c.Archetype)
		}
	}
	next[0].Archetype = "castle"
	assertAppCode(t, backend.SaveCategories(next), apperr.InvalidArgument)
	assertAppCode(t, backend.ApplyCategoryPreset(nil), apperr.InvalidArgument)
	assertAppCode(t, backend.ApplyCategoryPalette([]string{"blue"}), apperr.InvalidArgument)

	// Once the first-run guide is done, presets are refused and nothing changes.
	done := true
	if _, err := backend.UpdateSettings(SettingsPatchDTO{OnboardingCompleted: &done}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	assertAppCode(t, backend.ApplyCategoryPreset([]CategoryPresetDTO{
		{Name: "Late", ColorHex: "#111111", Archetype: "forge"},
	}), apperr.Conflict)
	day, _ = backend.GetTimelineDay("2026-09-12")
	for _, c := range day.Categories {
		if c.Name == "Late" {
			t.Fatal("a preset was applied after onboarding")
		}
	}
}

func TestCategoryPresetWritesRefuseReadOnlyInstance(t *testing.T) {
	dir := t.TempDir()
	writerBackendWithStore(t, dir) // holds the write lock

	backend := newBackend(fixedClock{}, nil, openTestStore(t, dir, false), false, false)
	backend.setEventEmitter(&recordingEmitter{})
	assertAppCode(t, backend.ApplyCategoryPreset([]CategoryPresetDTO{{Name: "X", ColorHex: "#111111"}}), apperr.NotCaptureOwner)
	assertAppCode(t, backend.ApplyCategoryPalette([]string{"#111111"}), apperr.NotCaptureOwner)
}
