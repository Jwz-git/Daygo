//go:build darwin

package secrets

import (
	"context"
	"os"
	"strings"
	"testing"
)

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
