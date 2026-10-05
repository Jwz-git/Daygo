//go:build windows

package windows

import (
	"syscall"
	"testing"
	"unsafe"

	xwindows "golang.org/x/sys/windows"
)

func TestUpdateInstallerArgumentsPreserveDirectory(t *testing.T) {
	directory := `D:\Apps with spaces\Daygo`
	if got, want := updateInstallerArguments(directory), `/DAYGO_UPDATE /D=`+directory; got != want {
		t.Fatalf("arguments = %q, want %q", got, want)
	}
}

func TestInstallerNativeCallbackDecodesPath(t *testing.T) {
	const payload = `C:\匿名 fixture\installer.exe`
	u := &Updater{updateSession: updateSession{
		canInstall: func() bool { return true },
		prepare:    func() error { return nil },
		shutdown:   func() {},
		launch: func(path string) error {
			if path != payload {
				t.Errorf("native callback path = %q, want %q", path, payload)
			}
			return nil
		},
	}}
	if !u.canShutdown() {
		t.Fatal("anonymous callback fixture must prepare successfully")
	}
	path, err := xwindows.UTF16PtrFromString(payload)
	if err != nil {
		t.Fatal(err)
	}
	callback := xwindows.NewCallbackCDecl(u.installerCallback)
	result, _, _ := syscall.SyscallN(callback, uintptr(unsafe.Pointer(path)))
	if result != 1 {
		t.Fatalf("native callback result = %d, want handled (1)", result)
	}
}

func TestInstallerNativeCallbackMissingPathRestoresRecording(t *testing.T) {
	resumed := 0
	u := &Updater{updateSession: updateSession{
		prepared: true,
		launch:   func(string) error { t.Fatal("missing native path must not launch"); return nil },
		cancel:   func() { resumed++ },
	}}
	callback := xwindows.NewCallbackCDecl(u.installerCallback)
	result, _, _ := syscall.SyscallN(callback, 0)
	if result != ^uintptr(0) || resumed != 1 {
		t.Fatalf("native callback result/resumed = %d/%d, want -1/1", result, resumed)
	}
}
