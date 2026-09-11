package app

import (
	"context"
	"errors"
	"time"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const applicationInspectionTimeout = 15 * time.Second

type applicationPicker interface {
	PickApplication() (string, error)
}

type wailsApplicationPicker struct {
	ctx context.Context
}

func (p wailsApplicationPicker) PickApplication() (string, error) {
	return wailsruntime.OpenFileDialog(p.ctx, wailsruntime.OpenDialogOptions{
		DefaultDirectory: "/Applications",
		// Wails v2 maps "*.app" to NSOpenPanel.allowedFileTypes, which leaves
		// application packages disabled on macOS. Leave the panel unfiltered;
		// ApplicationInspector remains the authoritative fail-closed .app check.
		ResolvesAliases:            true,
		TreatPackagesAsDirectories: false,
	})
}

// CaptureTestApplicationDTO is the safe, path-free identity returned by the
// temporary native picker. ID is the exact value consumed by Capture.
type CaptureTestApplicationDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PickCaptureTestApplication opens the native macOS application picker and
// resolves the selected bundle through the platform identity ABI. Cancellation
// returns nil without changing the current capture-test configuration.
func (b *Backend) PickCaptureTestApplication() (*CaptureTestApplicationDTO, error) {
	if b.applicationPicker == nil || b.applicationInspector == nil {
		return nil, apperr.E(apperr.NativeUnavailable, "application selection is unavailable", nil)
	}

	path, err := b.applicationPicker.PickApplication()
	if err != nil {
		return nil, apperr.E(apperr.NativeUnavailable, "could not open the application picker", err)
	}
	if path == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), applicationInspectionTimeout)
	defer cancel()
	info, err := b.applicationInspector.InspectApplication(ctx, path)
	if err != nil {
		return nil, applicationInspectionError(err)
	}
	if info.ID == "" || info.Name == "" {
		return nil, apperr.E(apperr.NativeUnavailable, "application identity is unavailable", nil)
	}
	return &CaptureTestApplicationDTO{ID: info.ID, Name: info.Name}, nil
}

func applicationInspectionError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return apperr.E(apperr.Canceled, "application inspection was canceled", err)
	}
	var inspectionError *platform.ApplicationError
	if errors.As(err, &inspectionError) {
		switch inspectionError.Code {
		case platform.ApplicationInvalidArgument,
			platform.ApplicationNotApplication:
			return apperr.E(apperr.InvalidArgument, "selected item is not an identifiable application", err)
		}
	}
	return apperr.E(apperr.NativeUnavailable, "application identity is unavailable", err)
}
