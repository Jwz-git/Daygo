// Package secrets implements the platform Secrets port.
//
// On macOS the keychain is reached through the /usr/bin/security command-line
// tool as a subprocess (decisions/providers-secrets-keychain): no cgo, so the
// CGO_ENABLED=0 gate and Linux testability hold. Other platforms return
// SecretUnsupported, and NewFake provides an in-memory implementation for
// tests.
//
// Secret values never appear in errors from this package: a leaked command or
// a failed lookup is reported by provider id and error code only.
package secrets

import (
	"errors"
	"fmt"
	"time"
)

// SecretsErrorCode is the stable classification for a keychain operation.
// Callers branch on Code; the underlying tool output stays out of the message.
type SecretsErrorCode string

const (
	SecretInvalidArgument SecretsErrorCode = "invalid_argument"
	SecretNotFound        SecretsErrorCode = "not_found"
	SecretUnsupported     SecretsErrorCode = "unsupported"
	SecretNative          SecretsErrorCode = "native"
)

func (c SecretsErrorCode) Valid() bool {
	switch c {
	case SecretInvalidArgument, SecretNotFound, SecretUnsupported, SecretNative:
		return true
	default:
		return false
	}
}

// SecretsError describes a failed keychain operation. Provider identifies the
// entry by provider id, never by secret content.
type SecretsError struct {
	Code     SecretsErrorCode
	Provider string
}

func (e *SecretsError) Error() string {
	if e == nil {
		return "secrets: operation failed"
	}
	return fmt.Sprintf("secrets: provider %q: %s", e.Provider, string(e.Code))
}

// keychainAccount is the fixed keychain account for every Daygo API key. The
// service name carries the provider identity (io.github.jwz-git.daygo.apikeys.<id>),
// so the account needs no variation.
const keychainAccount = "api-key"

// ServiceName builds the keychain service for a provider id. It is part of the
// published identity (docs/03 §3.1) and cannot change after the first public
// release.
func ServiceName(providerID string) string {
	return "io.github.jwz-git.daygo.apikeys." + providerID
}

// commandTimeout bounds each security subprocess. Keychain operations are
// interactive-rare and local; five seconds is generous for a non-interactive
// lookup and short enough that a stuck prompt does not hang a UI action.
const commandTimeout = 5 * time.Second

// validateProvider rejects an empty provider id before it reaches the keychain
// (an empty service name would silently address the wrong entry class).
func validateProvider(providerID string) error {
	if providerID == "" {
		return &SecretsError{Code: SecretInvalidArgument, Provider: ""}
	}
	return nil
}

// errNotFound reports a missing entry as a typed error rather than a failure.
func errNotFound(providerID string) error {
	return &SecretsError{Code: SecretNotFound, Provider: providerID}
}

// IsNotFound reports whether err is the typed not-found result, so callers can
// treat "no key stored" as a state rather than an error.
func IsNotFound(err error) bool {
	var se *SecretsError
	return errors.As(err, &se) && se.Code == SecretNotFound
}
