package secrets

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type secretToolCall struct {
	path  string
	args  []string
	stdin string
}

type fakeSecretTool struct {
	calls  []secretToolCall
	stdout []byte
	err    error
}

func (f *fakeSecretTool) output(_ context.Context, path string, args []string, stdin string) ([]byte, error) {
	f.calls = append(f.calls, secretToolCall{path: path, args: append([]string(nil), args...), stdin: stdin})
	return append([]byte(nil), f.stdout...), f.err
}

func TestSecretServiceRoundTripCommandsKeepSecretOutOfArguments(t *testing.T) {
	runner := &fakeSecretTool{}
	service := newSecretService("/usr/bin/secret-tool", runner)
	const canary = "sk-linux-canary"

	if err := service.Set(context.Background(), "p1", canary); err != nil {
		t.Fatalf("Set: %v", err)
	}
	wantSet := []string{"store", "--label=Daygo API key", "service", ServiceName("p1"), "account", keychainAccount}
	if got := runner.calls[0]; !reflect.DeepEqual(got.args, wantSet) || got.stdin != canary {
		t.Fatalf("Set call = %#v, want args %#v and secret on stdin", got, wantSet)
	}
	if strings.Contains(strings.Join(runner.calls[0].args, " "), canary) {
		t.Fatal("secret appears in secret-tool arguments")
	}

	runner.stdout = []byte("stored-value\n")
	got, err := service.Get(context.Background(), "p1")
	if err != nil || got != "stored-value" {
		t.Fatalf("Get = %q, %v", got, err)
	}

	if err := service.Delete(context.Background(), "p1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if runner.calls[2].args[0] != "clear" {
		t.Fatalf("Delete command = %q, want clear", runner.calls[2].args[0])
	}
}

func TestSecretServiceUnavailableAndFailuresAreTypedAndSanitized(t *testing.T) {
	service := newSecretService("", &fakeSecretTool{})
	err := service.Set(context.Background(), "p1", "sk-do-not-leak")
	var secretErr *SecretsError
	if !errors.As(err, &secretErr) || secretErr.Code != SecretUnsupported {
		t.Fatalf("missing tool error = %v, want unsupported", err)
	}

	runner := &fakeSecretTool{err: errors.New("sk-do-not-leak")}
	service = newSecretService("secret-tool", runner)
	err = service.Set(context.Background(), "p1", "sk-do-not-leak")
	if !errors.As(err, &secretErr) || secretErr.Code != SecretNative {
		t.Fatalf("runner error = %v, want native", err)
	}
	if strings.Contains(err.Error(), "sk-do-not-leak") {
		t.Fatalf("error leaked secret: %v", err)
	}
}

func TestSecretServiceExitOneLookupAndClearAreNotFound(t *testing.T) {
	for _, operation := range []string{"Get", "Delete"} {
		runner := &fakeSecretTool{err: fakeExitError(1)}
		service := newSecretService("secret-tool", runner)
		var got error
		if operation == "Get" {
			_, got = service.Get(context.Background(), "missing")
		} else {
			got = service.Delete(context.Background(), "missing")
		}
		if !IsNotFound(got) {
			t.Errorf("%s missing = %v, want not_found", operation, got)
		}
	}
}

type fakeExitError int

func (e fakeExitError) Error() string { return "command failed" }
func (e fakeExitError) ExitCode() int { return int(e) }
