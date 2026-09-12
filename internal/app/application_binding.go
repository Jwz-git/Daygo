package app

import (
	"context"
	"encoding/base64"
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

// ApplicationDTO is the display identity of one application bundle.
//
// Name and IconDataURL are display data, not product state: the persisted
// privacy setting holds only ID (docs/03 §3.3.5). IconDataURL is a base64 PNG
// data URL, empty when the system has no icon for the bundle. The bundle path
// never crosses this boundary.
type ApplicationDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IconDataURL string `json:"iconDataUrl"`
}

// PickApplication opens the native application picker and resolves the chosen
// bundle through the platform identity ABI. Cancellation returns nil without
// changing anything.
func (b *Backend) PickApplication() (*ApplicationDTO, error) {
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
	identity, err := b.applicationInspector.InspectApplication(ctx, path)
	if err != nil {
		return nil, applicationInspectionError(err)
	}
	if identity.ID == "" || identity.Name == "" {
		return nil, apperr.E(apperr.NativeUnavailable, "application identity is unavailable", nil)
	}
	dto := applicationToDTO(identity)
	return &dto, nil
}

// GetBlockedApplications returns the configured privacy list in configuration
// order, with the display name and icon the platform can resolve for each
// identifier.
//
// An identifier the platform cannot resolve keeps its place with an empty Name
// instead of being dropped: the frontend then shows the raw identifier, which
// is the only honest label available. Identifiers are the same values the
// recorder filters on; this binding never writes settings.
func (b *Backend) GetBlockedApplications() ([]ApplicationDTO, error) {
	access, err := b.settingsAccess()
	if err != nil {
		return nil, err
	}

	loadCtx, cancelLoad := context.WithTimeout(context.Background(), settingsTimeout)
	defer cancelLoad()
	snapshot, err := access.Load(loadCtx)
	if err != nil {
		return nil, mapStorageError("read blocked applications", err)
	}

	ids := snapshot.BlockedApplicationIDs
	if len(ids) == 0 {
		return []ApplicationDTO{}, nil
	}
	if b.applicationInspector == nil {
		return idOnlyApplications(ids), nil
	}

	applications := make([]ApplicationDTO, 0, len(ids))
	ctx, cancel := context.WithTimeout(context.Background(), applicationInspectionTimeout)
	defer cancel()
	identities, err := b.applicationInspector.DescribeApplications(ctx, ids)
	if err != nil {
		return nil, applicationInspectionError(err)
	}
	if len(identities) != len(ids) {
		// The port promises one result per identifier. A different count would
		// silently hide configured identifiers from the list — and with them
		// the only way to remove them — so the identity data is dropped for the
		// whole call rather than partially applied.
		return idOnlyApplications(ids), nil
	}
	for index, identity := range identities {
		if identity.ID == "" {
			identity.ID = ids[index]
		}
		applications = append(applications, applicationToDTO(identity))
	}
	return applications, nil
}

// idOnlyApplications keeps every configured identifier visible when no display
// identity can be resolved for it. The frontend shows the identifier itself.
func idOnlyApplications(ids []string) []ApplicationDTO {
	applications := make([]ApplicationDTO, 0, len(ids))
	for _, id := range ids {
		applications = append(applications, ApplicationDTO{ID: id})
	}
	return applications
}

func applicationToDTO(identity platform.ApplicationIdentity) ApplicationDTO {
	dto := ApplicationDTO{ID: identity.ID, Name: identity.Name}
	if len(identity.IconPNG) > 0 {
		dto.IconDataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(identity.IconPNG)
	}
	return dto
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
		case platform.ApplicationNotFound:
			return apperr.E(apperr.NotFound, "application is not installed", err)
		}
	}
	return apperr.E(apperr.NativeUnavailable, "application identity is unavailable", err)
}
