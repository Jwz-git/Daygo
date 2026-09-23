package app

import (
	"sync"

	"github.com/Jwz-git/Daygo/internal/platform"
)

// NativeUiLabelsDTO is the localized copy for native surfaces that render
// outside the webview and are not the status item: the platform application
// picker and the copy the platform updater shows when it refuses an install.
// vue-i18n cannot reach them, so the frontend pushes the translated bundle
// (docs/05 §5.5.1) and this layer routes each field to its surface — no
// adapter ever holds a locale.
type NativeUiLabelsDTO struct {
	// ApplicationPickerTitle titles the native application picker. Used where
	// the platform panel renders an app-supplied title.
	ApplicationPickerTitle string `json:"applicationPickerTitle"`
	// ApplicationPickerFilter names the executable filter in the application
	// picker. It is an affordance, not a security boundary.
	ApplicationPickerFilter string `json:"applicationPickerFilter"`
	// UpdateOwnerRequired is what the platform updater reports when an install
	// is refused because this instance is not the capture owner.
	UpdateOwnerRequired string `json:"updateOwnerRequired"`
}

// defaultNativeUiLabels seeds the native surfaces before the frontend pushes a
// bundle. zh-CN is the default interface language (settings.DefaultLanguage),
// so the seed matches what most users would otherwise wait for.
//
// Every field must be non-empty: these surfaces have no other copy source, so
// an empty seed renders an untitled or unexplained native dialog.
func defaultNativeUiLabels() NativeUiLabelsDTO {
	return NativeUiLabelsDTO{
		ApplicationPickerTitle:  "选择应用",
		ApplicationPickerFilter: "Windows 应用 (*.exe)",
		UpdateOwnerRequired:     "只有持有捕获所有权的 Daygo 实例才能安装更新。",
	}
}

type nativeUiLabelStore struct {
	mu     sync.RWMutex
	labels NativeUiLabelsDTO
}

func (s *nativeUiLabelStore) get() NativeUiLabelsDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.labels
}

func (s *nativeUiLabelStore) set(labels NativeUiLabelsDTO) {
	s.mu.Lock()
	s.labels = labels
	s.mu.Unlock()
}

// SetNativeUiLabels stores the localized copy for native surfaces outside the
// webview and forwards the update copy to the platform updater. It is safe to
// call on any instance: the picker reads the store at call time, and an adapter
// without a native update dialog simply never receives the copy.
func (b *Backend) SetNativeUiLabels(labels NativeUiLabelsDTO) error {
	b.nativeLabels.set(labels)
	b.pushUpdateCopy()
	return nil
}

// pushUpdateCopy hands the update refusal copy to an adapter that shows it in
// its own dialog. Adapters whose update UI is entirely rendered by the system
// do not implement the sink and are skipped.
func (b *Backend) pushUpdateCopy() {
	sink, ok := b.updater.(platform.UpdateCopySink)
	if !ok {
		return
	}
	sink.SetInstallRefusedMessage(b.nativeLabels.get().UpdateOwnerRequired)
}
