package apperr

import (
	"errors"
	"strings"
	"testing"
)

func TestCodeContract(t *testing.T) {
	want := []Code{
		InvalidArgument, NotFound, NotCaptureOwner, PermissionDenied,
		ProviderNotConfigured, ProviderFailed, NativeUnavailable,
		MediaDecodeFailed, Conflict, DatabaseError, Canceled, Internal,
	}

	got := Codes()
	if len(got) != len(want) {
		t.Fatalf("Codes length = %d, want %d", len(got), len(want))
	}
	seen := make(map[Code]bool, len(got))
	for i, code := range got {
		if !code.Valid() {
			t.Errorf("Codes()[%d] = %q is invalid", i, code)
		}
		if seen[code] {
			t.Errorf("duplicate code %q", code)
		}
		seen[code] = true
		if code != want[i] {
			t.Errorf("Codes()[%d] = %q, want %q", i, code, want[i])
		}
	}
	if Code("").Valid() || Code("other").Valid() {
		t.Fatal("unknown code reported valid")
	}

	got[0] = "mutated"
	if Codes()[0] != InvalidArgument {
		t.Fatal("Codes exposed mutable package state")
	}
}

func TestErrorEncodingAndUnwrap(t *testing.T) {
	cause := errors.New("private diagnostic context")
	err := E(NotFound, "provider was not found", cause)

	if got, want := err.Error(), "daygo:not_found: provider was not found"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, cause) {
		t.Fatal("Error does not preserve its internal cause")
	}
	if strings.Contains(err.Error(), cause.Error()) {
		t.Fatal("public error leaked its internal cause")
	}
}

func TestUnknownCodeFallsBackToInternal(t *testing.T) {
	err := E("not_a_code", "operation failed", nil)
	if err.Code != Internal {
		t.Fatalf("E(unknown).Code = %q, want %q", err.Code, Internal)
	}
}

func TestStaticPolicies(t *testing.T) {
	retryable := map[Code]bool{
		ProviderFailed: true, NativeUnavailable: true, Conflict: true,
	}
	reportable := map[Code]bool{
		ProviderFailed: true, NativeUnavailable: true, MediaDecodeFailed: true,
		Conflict: true, DatabaseError: true, Internal: true,
	}
	for _, code := range Codes() {
		if got := code.Retryable(); got != retryable[code] {
			t.Errorf("%q Retryable() = %t, want %t", code, got, retryable[code])
		}
		if got := code.Reportable(); got != reportable[code] {
			t.Errorf("%q Reportable() = %t, want %t", code, got, reportable[code])
		}
	}
}
