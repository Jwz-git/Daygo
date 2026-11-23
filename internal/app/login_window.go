package app

import (
	"context"
	"encoding/json"
	"io/fs"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/settings"
)

// Resolve all native menu/notification copy before allowing a windowless
// launch. A missing build resource or recovery entry point opens the UI.
func (b *Backend) prepareLoginWindow(ctx context.Context, assets fs.FS) bool {
	source, ok := b.system.(platform.LoginItemLaunch)
	if !ok {
		return false
	}
	languages, confirmed := source.LoginItemLaunch()
	if !confirmed {
		return false
	}
	snapshot, err := b.loadSettings(ctx)
	if err != nil || !snapshot.OnboardingCompleted {
		return false
	}
	locale := snapshot.Language
	if locale == "" {
		locale = settings.SystemLanguage(languages)
	}
	data, err := fs.ReadFile(assets, "native-labels/"+locale+".json")
	if err != nil {
		return false
	}
	var labels struct {
		Status StatusItemLabelsDTO
		Native NativeUiLabelsDTO
	}
	if json.Unmarshal(data, &labels) != nil || labels.Native.ApplicationMenu.Quit == "" ||
		labels.Native.UpdateOwnerRequired == "" || b.SetStatusItemLabels(labels.Status) != nil {
		return false
	}
	_ = b.SetNativeUiLabels(labels.Native)
	ready, ok := b.system.(platform.StatusItemAvailability)
	if !ok {
		return false
	}
	available, err := ready.StatusItemAvailable(ctx)
	return err == nil && available
}
