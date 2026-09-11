//go:build windows

package storage

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const windowsLockHelperEnv = "DAYGO_WINDOWS_LOCK_HELPER"

// TestWindowsFileLockHelper is executed in a child copy of the test binary.
// It holds one lock until the parent closes stdin or terminates the process.
func TestWindowsFileLockHelper(t *testing.T) {
	path := os.Getenv(windowsLockHelperEnv)
	if path == "" {
		return
	}
	lock, err := tryLock(path)
	if err != nil {
		t.Fatalf("helper tryLock: %v", err)
	}
	defer func() { _ = lock.release() }()

	fmt.Println("locked")
	_, _ = os.Stdin.Read(make([]byte, 1))
}

func TestWindowsFileLockContendsAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cross-process.lock")
	cmd, stdin := startWindowsLockHelper(t, path)

	if _, err := tryLock(path); !errors.Is(err, ErrLockBusy) {
		t.Fatalf("tryLock while child holds lock = %v, want ErrLockBusy", err)
	}

	if err := stdin.Close(); err != nil {
		t.Fatalf("close helper stdin: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait for helper: %v", err)
	}

	lock, err := tryLock(path)
	if err != nil {
		t.Fatalf("tryLock after helper release: %v", err)
	}
	if err := lock.release(); err != nil {
		t.Fatalf("release reacquired lock: %v", err)
	}
}

func TestWindowsFileLockReleasedWhenProcessTerminates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "terminated-process.lock")
	cmd, stdin := startWindowsLockHelper(t, path)
	defer func() { _ = stdin.Close() }()

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill helper: %v", err)
	}
	_ = cmd.Wait()

	deadline := time.Now().Add(5 * time.Second)
	for {
		lock, err := tryLock(path)
		if err == nil {
			if releaseErr := lock.release(); releaseErr != nil {
				t.Fatalf("release reacquired lock: %v", releaseErr)
			}
			return
		}
		if !errors.Is(err, ErrLockBusy) {
			t.Fatalf("tryLock after helper termination: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("lock still busy five seconds after helper termination")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func startWindowsLockHelper(t *testing.T, path string) (*exec.Cmd, io.WriteCloser) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestWindowsFileLockHelper$")
	cmd.Env = append(os.Environ(), windowsLockHelperEnv+"="+path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("helper stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("helper stdout: %v", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "locked" {
		t.Fatalf("helper did not report lock acquisition: line=%q err=%v", scanner.Text(), scanner.Err())
	}
	return cmd, stdin
}
