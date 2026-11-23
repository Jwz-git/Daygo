package app

import (
	"context"
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/Jwz-git/Daygo/internal/platform/fake"
)

type loginSystem struct {
	residentSystemFixture
	confirmed bool
}

func (s *loginSystem) LoginItemLaunch() ([]string, bool) { return []string{"fi", "en-GB"}, s.confirmed }

func TestLoginWindowRequiresLaunchEventOnboardingMenuAndLabels(t *testing.T) {
	sys := &loginSystem{residentSystemFixture: residentSystemFixture{System: fake.NewSystem(), available: true}}
	b, _ := backendWithStore(t)
	b.system = sys
	labels := defaultStatusItemLabels()
	labels.Open = "Open fixture"
	data, err := json.Marshal(map[string]any{"status": labels, "native": defaultNativeUiLabels()})
	if err != nil {
		t.Fatal(err)
	}
	assets := fstest.MapFS{"native-labels/en.json": &fstest.MapFile{Data: data}}
	ctx := context.Background()
	if b.prepareLoginWindow(ctx, assets) {
		t.Fatal("manual launch went windowless")
	}
	sys.confirmed = true
	if b.prepareLoginWindow(ctx, assets) {
		t.Fatal("lost first-run guide")
	}
	done := true
	systemLanguage := ""
	if _, err := b.UpdateSettings(SettingsPatchDTO{OnboardingCompleted: &done, Language: &systemLanguage}); err != nil {
		t.Fatal(err)
	}
	if !b.prepareLoginWindow(ctx, assets) || b.statusLabels.get().Open != "Open fixture" {
		t.Fatal("login lost localized status menu")
	}
	sys.available = false
	if b.prepareLoginWindow(ctx, assets) {
		t.Fatal("no recovery entry")
	}
	sys.available = true
	if b.prepareLoginWindow(ctx, fstest.MapFS{}) {
		t.Fatal("started without language bundle")
	}
	sys.confirmed = false
	if b.prepareLoginWindow(ctx, assets) {
		t.Fatal("menu availability cannot identify a login launch")
	}
}
