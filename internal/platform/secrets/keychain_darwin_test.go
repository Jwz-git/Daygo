//go:build darwin

package secrets

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestKeychainPresenceCachesOnlyStatusAndInvalidatesAfterWrites(t *testing.T) {
	k := New()
	present := false
	finds := 0
	k.command = func(_ context.Context, id string, args ...string) ([]byte, error) {
		switch args[0] {
		case "find-generic-password":
			finds++
			for _, arg := range args {
				if arg == "-w" || arg == "-g" {
					t.Fatal("presence query asked for a password")
				}
			}
			if !present {
				return nil, errNotFound(id)
			}
		case "add-generic-password":
			present = true
		case "delete-generic-password":
			present = false
		}
		return nil, nil
	}
	ctx := t.Context()
	check := func(want bool, reads int) {
		t.Helper()
		got, err := k.Has(ctx, testProvider)
		if err != nil || got != want || finds != reads {
			t.Fatalf("present=%v reads=%d err=%v, want %v and %d", got, finds, err, want, reads)
		}
	}
	check(false, 1)
	check(false, 1)
	if err := k.Set(ctx, testProvider, "anonymous-fixture"); err != nil {
		t.Fatal(err)
	}
	check(true, 2)
	check(true, 2)
	if err := k.Delete(ctx, testProvider); err != nil {
		t.Fatal(err)
	}
	check(false, 3)
}

// The real-keychain tests run only when explicitly opted in: they touch the
// user's login keychain (a personal, interactive environment), which CI and
// `go test ./...` must never do.
func realKeychain(t *testing.T) *Keychain {
	t.Helper()
	if testing.Short() {
		t.Skip("real keychain test skipped in short mode")
	}
	if os.Getenv("DAYGO_KEYCHAIN_TEST") != "1" {
		t.Skip("set DAYGO_KEYCHAIN_TEST=1 to run real keychain tests")
	}
	return New()
}

// The provider id is namespaced so a failed cleanup cannot touch anything but
// this test's own entries.
const testProvider = "daygo-test-keychain-smoke"

func TestKeychainRoundTrip(t *testing.T) {
	k := realKeychain(t)
	ctx := context.Background()

	_ = k.Delete(ctx, testProvider) // cleanup any earlier run
	if err := k.Set(ctx, testProvider, "test-value-1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	t.Cleanup(func() { _ = k.Delete(context.Background(), testProvider) })

	got, err := k.Get(ctx, testProvider)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "test-value-1" {
		t.Fatalf("Get = %q, want test-value-1", got)
	}

	if err := k.Set(ctx, testProvider, "test-value-2"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	got, _ = k.Get(ctx, testProvider)
	if got != "test-value-2" {
		t.Fatalf("Get after overwrite = %q, want test-value-2", got)
	}

	if err := k.Delete(ctx, testProvider); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := k.Get(ctx, testProvider); !IsNotFound(err) {
		t.Fatalf("Get after delete = %v, want typed not_found", err)
	}
}

// The Get error for a missing entry must not leak the service name or any
// secret-shaped text.
func TestKeychainNotFoundIsTypedAndSanitized(t *testing.T) {
	k := realKeychain(t)

	_, err := k.Get(context.Background(), testProvider+"-absent")
	if !IsNotFound(err) {
		t.Fatalf("Get absent = %v, want typed not_found", err)
	}
	if strings.Contains(err.Error(), "apikeys") {
		t.Fatalf("error leaks the service name: %q", err.Error())
	}
}

// Exercise the same bounded bundle used by OAuth with anonymous values only.
func TestBundleKeychainRoundTrip(t *testing.T) {
	k := realKeychain(t)
	v := NewBundleStore(k)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	id := "daygo-test-chatgpt-bundle-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = v.Delete(context.Background(), id) })
	first := strings.Repeat("anonymous-oauth-fixture-", 600)
	if err := v.Set(ctx, id, first); err != nil {
		t.Fatal(err)
	}
	if got, err := v.Get(ctx, id); err != nil || got != first {
		t.Fatal("anonymous bundle readback failed")
	}
	second := strings.Repeat("anonymous-replacement-", 400)
	if err := v.Set(ctx, id, second); err != nil {
		t.Fatal(err)
	}
	if got, err := v.Get(ctx, id); err != nil || got != second {
		t.Fatal("anonymous bundle rotation failed")
	}
	if err := v.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Get(ctx, id); !IsNotFound(err) {
		t.Fatal("deleted bundle remained readable")
	}
}
