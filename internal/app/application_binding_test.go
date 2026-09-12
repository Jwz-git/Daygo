package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform"
)

type privacyCaptureStub struct {
	compatibility platform.CapturePrivacyCompatibility
}

func (s privacyCaptureStub) Capture(context.Context, platform.CaptureRequest) (platform.CaptureResult, error) {
	return platform.CaptureResult{}, nil
}

func (s privacyCaptureStub) CapturePrivacyCompatibility(context.Context) (platform.CapturePrivacyCompatibility, error) {
	return s.compatibility, nil
}

func TestGetPrivacyCompatibility(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, true, true)
	backend.setCapture(privacyCaptureStub{compatibility: platform.CapturePrivacyCompatibility{
		Platform: "windows", Version: "Windows 11 10.0 (build 26100)",
		Build: 26100, MinimumBuild: 26100, Supported: true,
	}})
	got, err := backend.GetPrivacyCompatibility()
	if err != nil {
		t.Fatalf("GetPrivacyCompatibility: %v", err)
	}
	if got.Platform != "windows" || got.Build != 26100 || !got.Supported {
		t.Fatalf("compatibility = %#v", got)
	}
}

type fixedApplicationPicker struct {
	path string
	err  error
}

func (p fixedApplicationPicker) PickApplication() (string, error) {
	return p.path, p.err
}

type applicationInspectorStub struct {
	identity  platform.ApplicationIdentity
	err       error
	inspected string
	callCount int

	described     []platform.ApplicationIdentity
	describeErr   error
	describeIDs   []string
	describeCount int
}

func (s *applicationInspectorStub) InspectApplication(_ context.Context, path string) (platform.ApplicationIdentity, error) {
	s.inspected = path
	s.callCount++
	return s.identity, s.err
}

func (s *applicationInspectorStub) DescribeApplications(_ context.Context, ids []string) ([]platform.ApplicationIdentity, error) {
	s.describeIDs = append([]string(nil), ids...)
	s.describeCount++
	if s.describeErr != nil {
		return nil, s.describeErr
	}
	return s.described, nil
}

func TestPickApplicationReturnsPathFreeIdentityWithIcon(t *testing.T) {
	inspector := &applicationInspectorStub{identity: platform.ApplicationIdentity{
		ID:      "com.apple.calculator",
		Name:    "Calculator",
		IconPNG: []byte("\x89PNG fixture"),
	}}
	backend := newBackend(systemClock{}, nil, nil, false, false)
	backend.setApplicationPicker(fixedApplicationPicker{path: "/System/Applications/Calculator.app"})
	backend.setApplicationInspector(inspector)

	got, err := backend.PickApplication()
	if err != nil {
		t.Fatalf("PickApplication: %v", err)
	}
	if got == nil || got.ID != "com.apple.calculator" || got.Name != "Calculator" {
		t.Fatalf("application = %+v", got)
	}
	if !strings.HasPrefix(got.IconDataURL, "data:image/png;base64,") {
		t.Fatalf("iconDataUrl = %q, want a PNG data URL", got.IconDataURL)
	}
	if inspector.inspected != "/System/Applications/Calculator.app" {
		t.Fatalf("inspected path = %q", inspector.inspected)
	}
}

func TestPickApplicationCancelDoesNotInspect(t *testing.T) {
	inspector := &applicationInspectorStub{}
	backend := newBackend(systemClock{}, nil, nil, false, false)
	backend.setApplicationPicker(fixedApplicationPicker{})
	backend.setApplicationInspector(inspector)

	got, err := backend.PickApplication()
	if err != nil || got != nil {
		t.Fatalf("cancel result = %+v, error = %v", got, err)
	}
	if inspector.callCount != 0 {
		t.Fatalf("inspector calls = %d, want 0", inspector.callCount)
	}
}

func TestPickApplicationRejectsNonApplication(t *testing.T) {
	inspector := &applicationInspectorStub{err: &platform.ApplicationError{
		Code: platform.ApplicationNotApplication,
	}}
	backend := newBackend(systemClock{}, nil, nil, false, false)
	backend.setApplicationPicker(fixedApplicationPicker{path: "/Applications/Example.app"})
	backend.setApplicationInspector(inspector)

	_, err := backend.PickApplication()
	var apiError *apperr.Error
	if !errors.As(err, &apiError) || apiError.Code != apperr.InvalidArgument {
		t.Fatalf("error = %v, want invalid_argument", err)
	}
}

// The privacy screen renders whatever the platform resolved, in configuration
// order, without inventing a label for an identifier it cannot resolve.
func TestGetBlockedApplicationsResolvesConfiguredIdentifiers(t *testing.T) {
	backend, _ := backendWithStore(t)
	blocked := []string{"com.apple.Safari", "com.example.gone"}
	inspector := &applicationInspectorStub{described: []platform.ApplicationIdentity{
		{ID: "com.apple.Safari", Name: "Safari", IconPNG: []byte("\x89PNG fixture")},
		{ID: "com.example.gone"},
	}}
	backend.setApplicationInspector(inspector)
	if _, err := backend.UpdateSettings(SettingsPatchDTO{BlockedApplicationIDs: &blocked}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	got, err := backend.GetBlockedApplications()
	if err != nil {
		t.Fatalf("GetBlockedApplications: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("applications = %+v", got)
	}
	if got[0].ID != "com.apple.Safari" || got[0].Name != "Safari" || got[0].IconDataURL == "" {
		t.Fatalf("resolved application = %+v", got[0])
	}
	if got[1].ID != "com.example.gone" || got[1].Name != "" || got[1].IconDataURL != "" {
		t.Fatalf("unresolved application = %+v", got[1])
	}
	if len(inspector.describeIDs) != 2 || inspector.describeIDs[0] != "com.apple.Safari" {
		t.Fatalf("describe ids = %v", inspector.describeIDs)
	}
}

// Without a platform resolver the list still returns one entry per configured
// identifier so the user can see and remove them.
func TestGetBlockedApplicationsWithoutResolverKeepsIdentifiers(t *testing.T) {
	backend, _ := backendWithStore(t)
	blocked := []string{"com.apple.Safari"}
	if _, err := backend.UpdateSettings(SettingsPatchDTO{BlockedApplicationIDs: &blocked}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	got, err := backend.GetBlockedApplications()
	if err != nil {
		t.Fatalf("GetBlockedApplications: %v", err)
	}
	if len(got) != 1 || got[0].ID != "com.apple.Safari" || got[0].Name != "" {
		t.Fatalf("applications = %+v", got)
	}
}

// An adapter that answers with the wrong count must not hide configured
// identifiers: without a row the user cannot remove the app from the list.
func TestGetBlockedApplicationsFallsBackWhenAdapterCountDiffers(t *testing.T) {
	backend, _ := backendWithStore(t)
	blocked := []string{"com.apple.Safari", "com.example.other"}
	inspector := &applicationInspectorStub{described: []platform.ApplicationIdentity{
		{ID: "com.apple.Safari", Name: "Safari"},
	}}
	backend.setApplicationInspector(inspector)
	if _, err := backend.UpdateSettings(SettingsPatchDTO{BlockedApplicationIDs: &blocked}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	got, err := backend.GetBlockedApplications()
	if err != nil {
		t.Fatalf("GetBlockedApplications: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("applications = %+v", got)
	}
	for index, application := range got {
		if application.ID != blocked[index] || application.Name != "" {
			t.Fatalf("application %d = %+v, want an ID-only entry", index, application)
		}
	}
}

func TestGetBlockedApplicationsWithoutStoreFails(t *testing.T) {
	backend := newBackend(systemClock{}, nil, nil, false, false)

	_, err := backend.GetBlockedApplications()
	var apiError *apperr.Error
	if !errors.As(err, &apiError) || apiError.Code != apperr.DatabaseError {
		t.Fatalf("error = %v, want database_error", err)
	}
}
