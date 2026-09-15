//go:build linux

package secrets

import (
	"context"
	"os"
	"testing"
)

// This test touches the current desktop session's keyring and therefore runs
// only with an explicit opt-in on a real Linux desktop.
func TestSecretServiceRealRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("real Secret Service test skipped in short mode")
	}
	if os.Getenv("DAYGO_SECRET_SERVICE_TEST") != "1" {
		t.Skip("set DAYGO_SECRET_SERVICE_TEST=1 to run the real keyring test")
	}

	service := New()
	ctx := context.Background()
	const providerID = "daygo-test-secret-service-smoke"
	_ = service.Delete(ctx, providerID)

	if err := service.Set(ctx, providerID, "test-value-1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	t.Cleanup(func() { _ = service.Delete(context.Background(), providerID) })

	got, err := service.Get(ctx, providerID)
	if err != nil || got != "test-value-1" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if err := service.Set(ctx, providerID, "test-value-2"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	got, err = service.Get(ctx, providerID)
	if err != nil || got != "test-value-2" {
		t.Fatalf("Get after overwrite = %q, %v", got, err)
	}
	if err := service.Delete(ctx, providerID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := service.Get(ctx, providerID); !IsNotFound(err) {
		t.Fatalf("Get after delete = %v, want not_found", err)
	}
}
