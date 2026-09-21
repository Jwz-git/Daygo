package app

import (
	"testing"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/fake"
)

// copyCapturingUpdater records what the app pushes to an adapter that renders
// the install refusal in its own dialog. It implements the sink but not the
// install coordinator, which is the Windows shape.
type copyCapturingUpdater struct {
	*fake.Updater
	messages []string
}

func (u *copyCapturingUpdater) SetInstallRefusedMessage(message string) {
	u.messages = append(u.messages, message)
}

// coordinatedCopyUpdater is the macOS shape: the adapter coordinates the
// install and renders the refusal copy.
type coordinatedCopyUpdater struct {
	*fake.Updater
	messages   []string
	callbacked bool
}

func (u *coordinatedCopyUpdater) SetInstallCallbacks(func() bool, func() error, func()) {
	u.callbacked = true
}

func (u *coordinatedCopyUpdater) SetInstallRefusedMessage(message string) {
	u.messages = append(u.messages, message)
}

var (
	_ platform.UpdateCopySink           = (*copyCapturingUpdater)(nil)
	_ platform.UpdateCopySink           = (*coordinatedCopyUpdater)(nil)
	_ platform.UpdateInstallCoordinator = (*coordinatedCopyUpdater)(nil)
)

func TestSetNativeUiLabelsStoresBundleAndForwardsUpdateCopy(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, true, true)
	updater := &copyCapturingUpdater{Updater: fake.NewUpdater()}
	backend.setUpdater(updater)

	labels := NativeUiLabelsDTO{
		ApplicationPickerTitle:  "选择应用",
		ApplicationPickerFilter: "Windows 应用 (*.exe)",
		UpdateOwnerRequired:     "只有持有捕获所有权的 Daygo 实例才能安装更新。",
	}
	if err := backend.SetNativeUiLabels(labels); err != nil {
		t.Fatalf("SetNativeUiLabels: %v", err)
	}

	if got := backend.nativeLabels.get(); got != labels {
		t.Fatalf("stored labels = %#v, want %#v", got, labels)
	}
	if len(updater.messages) != 1 || updater.messages[0] != labels.UpdateOwnerRequired {
		t.Fatalf("update copy pushes = %q, want exactly [%q]", updater.messages, labels.UpdateOwnerRequired)
	}
}

// An adapter that renders its own update UI holds no locale either: the push
// must be a no-op rather than a panic or an error.
func TestSetNativeUiLabelsWithoutUpdateCopySink(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, true, true)
	backend.setUpdater(fake.NewUpdater())

	labels := NativeUiLabelsDTO{UpdateOwnerRequired: "只有持有捕获所有权的 Daygo 实例才能安装更新。"}
	if err := backend.SetNativeUiLabels(labels); err != nil {
		t.Fatalf("SetNativeUiLabels: %v", err)
	}
	if got := backend.nativeLabels.get(); got != labels {
		t.Fatalf("stored labels = %#v, want %#v", got, labels)
	}
}

// The refusal copy must exist before an install can be attempted, so install
// configuration seeds the adapter instead of waiting for the frontend push.
func TestConfigureUpdateInstallSeedsUpdateCopy(t *testing.T) {
	backend := newBackend(fixedClock{}, nil, nil, true, true)
	updater := &coordinatedCopyUpdater{Updater: fake.NewUpdater()}
	backend.setUpdater(updater)

	backend.configureUpdateInstall(func() {})

	if !updater.callbacked {
		t.Fatal("install callbacks were not configured on the coordinated updater")
	}
	want := defaultNativeUiLabels().UpdateOwnerRequired
	if len(updater.messages) != 1 || updater.messages[0] != want {
		t.Fatalf("update copy pushes = %q, want exactly [%q]", updater.messages, want)
	}
}

func TestApplicationPickerOptionsCarryLocalizedCopy(t *testing.T) {
	labels := NativeUiLabelsDTO{
		ApplicationPickerTitle:  "选择应用",
		ApplicationPickerFilter: "Windows 应用 (*.exe)",
	}

	windows := applicationPickerOptions("windows", labels)
	if windows.Title != labels.ApplicationPickerTitle {
		t.Fatalf("windows picker title = %q, want %q", windows.Title, labels.ApplicationPickerTitle)
	}
	if len(windows.Filters) != 1 || windows.Filters[0].DisplayName != labels.ApplicationPickerFilter || windows.Filters[0].Pattern != "*.exe" {
		t.Fatalf("windows picker filters = %#v", windows.Filters)
	}
	if windows.DefaultDirectory != "" {
		t.Fatalf("windows picker default directory = %q, want the Explorer default", windows.DefaultDirectory)
	}

	// macOS gets neither: a "*.app" allowedFileTypes filter greys out every
	// application package, and the panel renders no app-supplied title.
	darwin := applicationPickerOptions("darwin", labels)
	if darwin.Title != "" {
		t.Fatalf("darwin picker title = %q, want none", darwin.Title)
	}
	if len(darwin.Filters) != 0 {
		t.Fatalf("darwin picker filters = %#v, want none", darwin.Filters)
	}
	if darwin.DefaultDirectory != "/Applications" {
		t.Fatalf("darwin picker default directory = %q, want /Applications", darwin.DefaultDirectory)
	}
}

// These surfaces have no copy source other than this seed, so an empty field
// renders an untitled or unexplained native dialog.
func TestDefaultNativeUiLabelsArePopulated(t *testing.T) {
	labels := defaultNativeUiLabels()
	if labels.ApplicationPickerTitle == "" || labels.ApplicationPickerFilter == "" || labels.UpdateOwnerRequired == "" {
		t.Fatalf("default native labels = %#v, want every field populated", labels)
	}
}
