//go:build windows

package secrets

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func realCredentialManager(t *testing.T) *CredentialManager {
	t.Helper()
	if testing.Short() {
		t.Skip("real Credential Manager test skipped in short mode")
	}
	if os.Getenv("DAYGO_CREDENTIAL_MANAGER_TEST") != "1" {
		t.Skip("set DAYGO_CREDENTIAL_MANAGER_TEST=1 to run real Credential Manager tests")
	}
	return New()
}

const windowsTestProvider = "daygo-test-credential-manager-smoke"

func TestCredentialManagerRoundTrip(t *testing.T) {
	m := realCredentialManager(t)
	ctx := context.Background()

	_ = m.Delete(ctx, windowsTestProvider)
	if err := m.Set(ctx, windowsTestProvider, "test-value-1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	t.Cleanup(func() { _ = m.Delete(context.Background(), windowsTestProvider) })

	got, err := m.Get(ctx, windowsTestProvider)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "test-value-1" {
		t.Fatalf("Get = %q, want test-value-1", got)
	}

	if err := m.Set(ctx, windowsTestProvider, "test-value-2"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	got, err = m.Get(ctx, windowsTestProvider)
	if err != nil {
		t.Fatalf("Get after overwrite: %v", err)
	}
	if got != "test-value-2" {
		t.Fatalf("Get after overwrite = %q, want test-value-2", got)
	}

	if err := m.Delete(ctx, windowsTestProvider); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := m.Get(ctx, windowsTestProvider); !IsNotFound(err) {
		t.Fatalf("Get after delete = %v, want typed not_found", err)
	}
}

func TestCredentialManagerRejectsOversizedSecretWithoutLeakingIt(t *testing.T) {
	m := New()
	secret := strings.Repeat("x", maxCredentialBlobSize+1)
	err := m.Set(context.Background(), windowsTestProvider, secret)

	var secretErr *SecretsError
	if !errors.As(err, &secretErr) || secretErr.Code != SecretInvalidArgument {
		t.Fatalf("Set oversized secret = %v, want invalid_argument", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("error contains the rejected secret")
	}
}

func TestCredentialManagerRejectsProviderWithNUL(t *testing.T) {
	m := New()
	err := m.Set(context.Background(), "provider\x00suffix", "test-value")

	var secretErr *SecretsError
	if !errors.As(err, &secretErr) || secretErr.Code != SecretInvalidArgument {
		t.Fatalf("Set provider with NUL = %v, want invalid_argument", err)
	}
}
