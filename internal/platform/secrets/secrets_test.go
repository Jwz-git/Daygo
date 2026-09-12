package secrets

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFakeRoundTrip(t *testing.T) {
	f := NewFake()
	ctx := context.Background()

	if err := f.Set(ctx, "p1", "s1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := f.Get(ctx, "p1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "s1" {
		t.Fatalf("Get = %q, want s1", got)
	}

	// Set overwrites.
	if err := f.Set(ctx, "p1", "s2"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	if got, _ = f.Get(ctx, "p1"); got != "s2" {
		t.Fatalf("Get after overwrite = %q, want s2", got)
	}

	if err := f.Delete(ctx, "p1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = f.Get(ctx, "p1")
	if !IsNotFound(err) {
		t.Fatalf("Get after delete = %v, want typed not_found", err)
	}
}

func TestFakeDeleteAbsentIsNotFound(t *testing.T) {
	f := NewFake()

	err := f.Delete(context.Background(), "absent")
	if !IsNotFound(err) {
		t.Fatalf("Delete absent = %v, want typed not_found", err)
	}
}

func TestEmptyProviderIDRejected(t *testing.T) {
	f := NewFake()
	ctx := context.Background()

	var err *SecretsError
	if e := f.Set(ctx, "", "s"); !errors.As(e, &err) || err.Code != SecretInvalidArgument {
		t.Fatalf("Set with empty id = %v, want invalid_argument", e)
	}
	if _, e := f.Get(ctx, ""); !errors.As(e, &err) || err.Code != SecretInvalidArgument {
		t.Fatalf("Get with empty id = %v, want invalid_argument", e)
	}
}

// The error string must identify the provider but never the secret. A canary
// value asserts the secret cannot leak through the typed error.
func TestSecretNeverAppearsInErrors(t *testing.T) {
	f := NewFake()
	canary := "sk-canary-DO-NOT-LEAK-0000"

	_ = f.Set(context.Background(), "p1", canary)
	// The only error path that touched the secret: delete then re-delete.
	_ = f.Delete(context.Background(), "p1")
	err := f.Delete(context.Background(), "p1")

	var secretErr *SecretsError
	if !errors.As(err, &secretErr) {
		t.Fatalf("Delete = %v, want a SecretsError", err)
	}
	if strings.Contains(secretErr.Error(), canary) {
		t.Fatalf("error message contains the secret: %q", secretErr.Error())
	}
	if strings.Contains(secretErr.Error(), "sk-") {
		t.Fatalf("error message may contain secret-shaped text: %q", secretErr.Error())
	}
}

func TestServiceName(t *testing.T) {
	if got, want := ServiceName("abc"), "io.github.jwz-git.daygo.apikeys.abc"; got != want {
		t.Fatalf("ServiceName = %q, want %q", got, want)
	}
}
