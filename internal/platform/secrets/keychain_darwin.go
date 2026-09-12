//go:build darwin

package secrets

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/Jwz-git/Daygo/internal/platform"
)

var _ platform.Secrets = (*Keychain)(nil)

// Keychain stores provider API keys as generic passwords in the user login
// keychain, reached through the security CLI. The decision record
// (decisions/providers-secrets-keychain) covers why a subprocess and not
// Security.framework: cgo is barred by the CGO_ENABLED=0 gate.
type Keychain struct{}

// New returns the macOS keychain implementation of platform.Secrets.
func New() *Keychain { return &Keychain{} }

func (k *Keychain) Set(ctx context.Context, providerID, secret string) error {
	if err := validateProvider(providerID); err != nil {
		return err
	}
	// -U updates an existing item instead of failing on the second write.
	return k.run(ctx, providerID,
		"add-generic-password", "-U",
		"-s", ServiceName(providerID),
		"-a", keychainAccount,
		"-w", secret)
}

func (k *Keychain) Get(ctx context.Context, providerID string) (string, error) {
	if err := validateProvider(providerID); err != nil {
		return "", err
	}
	out, err := k.output(ctx, providerID,
		"find-generic-password",
		"-s", ServiceName(providerID),
		"-a", keychainAccount,
		"-w")
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (k *Keychain) Delete(ctx context.Context, providerID string) error {
	if err := validateProvider(providerID); err != nil {
		return err
	}
	err := k.run(ctx, providerID,
		"delete-generic-password",
		"-s", ServiceName(providerID),
		"-a", keychainAccount)
	if err != nil && isSecurityNotFound(err) {
		// Deleting an absent entry is the caller's desired end state.
		return errNotFound(providerID)
	}
	return err
}

// run executes the security tool, discarding stdout.
func (k *Keychain) run(ctx context.Context, providerID string, args ...string) error {
	_, err := k.output(ctx, providerID, args...)
	return err
}

// output executes the security tool and returns stdout. The error carries no
// command output: stderr may quote the secret-bearing arguments.
func (k *Keychain) output(ctx context.Context, providerID string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/usr/bin/security", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if isSecurityNotFound(err) {
			return nil, errNotFound(providerID)
		}
		return nil, &SecretsError{Code: SecretNative, Provider: providerID}
	}
	return stdout.Bytes(), nil
}

// isSecurityNotFound recognizes the tool's "could not be found" exit; the
// security CLI exits 44 for a missing item, and the message also carries the
// phrase. Both are checked because the numeric code is not documented as
// stable API.
func isSecurityNotFound(err error) bool {
	if err == nil {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 44 {
		return true
	}
	return strings.Contains(err.Error(), "could not be found")
}
