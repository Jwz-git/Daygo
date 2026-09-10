package fake_test

import (
	"testing"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
	"github.com/Jwz-git/Daygo/internal/platform/platformtest"
)

func TestCaptureContract(t *testing.T) {
	platformtest.Suite(t, func(t *testing.T) platform.Capture {
		t.Helper()
		return fake.NewCapture()
	})
}

func TestCapturePermissionContract(t *testing.T) {
	platformtest.SuitePermission(t, func(t *testing.T) platformtest.AuthorizedCapture {
		t.Helper()
		return fake.NewCapture()
	})
}

func TestCapturePrivacyContract(t *testing.T) {
	platformtest.SuitePrivacy(t, func(t *testing.T) platformtest.PrivacyCapture {
		t.Helper()
		return fake.NewCapture()
	})
}

func TestCaptureNoDisplayContract(t *testing.T) {
	platformtest.SuiteNoDisplay(t, func(t *testing.T) platformtest.DisplayCapture {
		t.Helper()
		return fake.NewCapture()
	})
}
