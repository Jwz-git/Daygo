//go:build windows

package windows

import "testing"

func TestUpdateInstallerArgumentsPreserveDirectory(t *testing.T) {
	directory := `D:\Apps with spaces\Daygo`
	if got, want := updateInstallerArguments(directory), `/DAYGO_UPDATE /D=`+directory; got != want {
		t.Fatalf("arguments = %q, want %q", got, want)
	}
}
