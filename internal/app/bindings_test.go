package app

import (
	"reflect"
	"sort"
	"testing"
)

// contractBindings is the complete set of methods the frontend may call,
// mirroring the catalog in docs/05 §5.2.1. Adding a name here is a wire change
// and belongs in the same commit as the documentation update (docs/05 §5.10.2).
var contractBindings = []string{
	"AddProvider",
	"CaptureTest",
	"DeleteProvider",
	"DeleteProviderSecret",
	"GetCapabilities",
	"GetDayContext",
	"GetDiagnostics",
	"GetPermissionState",
	"GetProviderRouting",
	"GetRecordingDirectory",
	"GetRecordingState",
	"GetSettings",
	"ListProviders",
	"OpenCaptureTestFolder",
	"PickCaptureTestApplication",
	"OpenSystemSettings",
	"PauseRecording",
	"RequestScreenRecordingPermission",
	"ResumeRecording",
	"SetProviderRouting",
	"SetProviderSecret",
	"SetRecording",
	"PollSystemEvents",
	"TestProvider",
	"TestProviderConnection",
	"UpdateProvider",
	"UpdateSettings",
}

// TestBackendExportsOnlyContractMethods locks the binding surface.
//
// Wails binds every exported method on the bound object, so an exported helper
// becomes part of the frontend API without anyone deciding that it should. That
// already happened once: SetEventEmitter and Store were exported for
// composition, and both ended up in the generated Backend.d.ts — Store even
// dragged storage.Store into the generated models, handing the frontend a type
// for a handle it must never hold.
//
// Comparing the reflected method set against the documented catalog turns that
// class of accident into a failing test instead of a review that has to notice
// a missing lowercase letter.
func TestBackendExportsOnlyContractMethods(t *testing.T) {
	backendType := reflect.TypeOf(&Backend{})

	exported := make([]string, 0, backendType.NumMethod())
	for index := 0; index < backendType.NumMethod(); index++ {
		exported = append(exported, backendType.Method(index).Name)
	}
	sort.Strings(exported)

	want := append([]string(nil), contractBindings...)
	sort.Strings(want)

	if !reflect.DeepEqual(exported, want) {
		t.Fatalf("exported methods on *Backend = %v, contract = %v\n"+
			"Wails binds every exported method: unexport the helper, or add the "+
			"method to docs/05 §5.5.1 and to contractBindings in the same commit.",
			exported, want)
	}
}
