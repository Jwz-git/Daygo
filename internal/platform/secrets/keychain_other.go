//go:build !darwin

package secrets

import (
	"context"

	"github.com/Jwz-git/Daygo/internal/platform"
)

var _ platform.Secrets = (*unavailableSecrets)(nil)

type unavailableSecrets struct{}

// New returns a Secrets implementation that refuses every operation: the
// keychain is a macOS capability, and a silent in-memory stand-in in a
// production binary would let a caller believe a key was stored when it was
// not. Tests use NewFake.
func New() platform.Secrets { return unavailableSecrets{} }

func (unavailableSecrets) Get(context.Context, string) (string, error) {
	return "", &SecretsError{Code: SecretUnsupported}
}

func (unavailableSecrets) Set(context.Context, string, string) error {
	return &SecretsError{Code: SecretUnsupported}
}

func (unavailableSecrets) Delete(context.Context, string) error {
	return &SecretsError{Code: SecretUnsupported}
}
