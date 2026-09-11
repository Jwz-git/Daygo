package app

import (
	"context"
	"errors"
	"testing"

	"github.com/Jwz-git/Daygo/internal/app/apperr"
	"github.com/Jwz-git/Daygo/internal/platform"
)

type fixedApplicationPicker struct {
	path string
	err  error
}

func (p fixedApplicationPicker) PickApplication() (string, error) {
	return p.path, p.err
}

type applicationInspectorStub struct {
	info      platform.AppInfo
	err       error
	inspected string
	callCount int
}

func (s *applicationInspectorStub) InspectApplication(_ context.Context, path string) (platform.AppInfo, error) {
	s.inspected = path
	s.callCount++
	return s.info, s.err
}

func TestPickCaptureTestApplicationReturnsPathFreeIdentity(t *testing.T) {
	inspector := &applicationInspectorStub{info: platform.AppInfo{
		ID:   "com.apple.calculator",
		Name: "Calculator",
	}}
	backend := newBackend(systemClock{}, nil, nil, false, false)
	backend.setApplicationPicker(fixedApplicationPicker{path: "/System/Applications/Calculator.app"})
	backend.setApplicationInspector(inspector)

	got, err := backend.PickCaptureTestApplication()
	if err != nil {
		t.Fatalf("PickCaptureTestApplication: %v", err)
	}
	if got == nil || got.ID != "com.apple.calculator" || got.Name != "Calculator" {
		t.Fatalf("application = %+v", got)
	}
	if inspector.inspected != "/System/Applications/Calculator.app" {
		t.Fatalf("inspected path = %q", inspector.inspected)
	}
}

func TestPickCaptureTestApplicationCancelDoesNotInspect(t *testing.T) {
	inspector := &applicationInspectorStub{}
	backend := newBackend(systemClock{}, nil, nil, false, false)
	backend.setApplicationPicker(fixedApplicationPicker{})
	backend.setApplicationInspector(inspector)

	got, err := backend.PickCaptureTestApplication()
	if err != nil || got != nil {
		t.Fatalf("cancel result = %+v, error = %v", got, err)
	}
	if inspector.callCount != 0 {
		t.Fatalf("inspector calls = %d, want 0", inspector.callCount)
	}
}

func TestPickCaptureTestApplicationRejectsNonApplication(t *testing.T) {
	inspector := &applicationInspectorStub{err: &platform.ApplicationError{
		Code: platform.ApplicationNotApplication,
	}}
	backend := newBackend(systemClock{}, nil, nil, false, false)
	backend.setApplicationPicker(fixedApplicationPicker{path: "/Applications/Example.app"})
	backend.setApplicationInspector(inspector)

	_, err := backend.PickCaptureTestApplication()
	var apiError *apperr.Error
	if !errors.As(err, &apiError) || apiError.Code != apperr.InvalidArgument {
		t.Fatalf("error = %v, want invalid_argument", err)
	}
}
